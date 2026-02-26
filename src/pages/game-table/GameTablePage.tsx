import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { motion } from "framer-motion";
import { applyMockMatchAction, isMockApiEnabled } from "@/mocks/mockApi";
import type { MatchActionType, Room } from "@/entities/match/types";
import type { Card } from "@/entities/card/types";
import { joinGameRoom } from "@/processes/joinGame.process";
import { getRoom, leaveRoom } from "@/shared/api/rooms";
import { getProfile } from "@/shared/api/user";
import {
  hapticImpact,
  hapticNotification,
  hapticSelection,
} from "@/shared/lib/telegram";
import { wsClient } from "@/shared/api/ws/socket";
import { AppAvatar } from "@/shared/ui/Avatar";
import { BackIcon } from "@/shared/ui/Icons";
import { AppCard } from "@/shared/ui/Card";
import { PlayingCard } from "@/shared/ui/PlayingCard";
import { AppButton } from "@/shared/ui/Button";
import { ReconnectOverlay } from "@/shared/ui/ReconnectOverlay";
import { CardSkeleton, ConfirmModal, EmptyStateBlock } from "@/shared/ui/StateBlocks";
import { formatActivityItem } from "@/entities/game/lib/formatActivity";
import { PlayerHandFan } from "@/features/game/PlayerHandFan";
import {
  selectCanAct,
  selectCanAttack,
  selectCanDefend,
  selectCanPass,
  selectCanTake,
  selectIsMyTurn,
  selectCurrentPhase,
  selectOrderedSeats,
  selectIsAttacker,
  selectIsDefender,
} from "@/entities/game/model/selectors";
import { useSwipeDown } from "@/shared/hooks/useSwipeDown";
import {
  addActivity,
  clearGameError,
  getGameState,
  setGameConnecting,
  setMatchState,
  subscribeGameStore,
} from "@/store/game.store";

export function GameTablePage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [room, setRoom] = useState<Room | null>(null);
  const [gameState, setGameState] = useState(getGameState());
  const [isRoomLoading, setIsRoomLoading] = useState(true);
  const [isExitModalOpen, setIsExitModalOpen] = useState(false);
  const [selectedCardId, setSelectedCardId] = useState<string | null>(null);
  const [secondsLeft, setSecondsLeft] = useState<number | null>(null);
  const [currentUserId, setCurrentUserId] = useState<string | null>(null);
  const [profileBalance, setProfileBalance] = useState<number | null>(null);
  const [intentError, setIntentError] = useState<{ text: string; code?: string } | null>(null);
  const currency = "USD";
  const tableZoneRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    void getProfile()
      .then((r) => {
        setCurrentUserId(r.user.id);
        setProfileBalance(r.balance);
      })
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    function onIntentError(ev: Event) {
      const ce = ev as CustomEvent;
      const detail = (ce.detail ?? {}) as { text?: string; code?: string; raw?: unknown };
      const text = detail.text || "Ошибка выполнения действия.";
      const code = detail.code;

      // eslint-disable-next-line no-console
      console.warn("[tma:intentError] received", detail);

      setIntentError({ text, code });

      if (code === "kicked") {
        window.setTimeout(() => {
          navigate("/play");
        }, 800);
      }
    }

    window.addEventListener("tma:intentError", onIntentError as EventListener);
    return () => {
      window.removeEventListener("tma:intentError", onIntentError as EventListener);
    };
  }, [navigate]);

  const matchState = gameState.matchState;
  useEffect(() => {
    if (gameState.matchResult) {
      void getProfile()
        .then((r) => setProfileBalance(r.balance))
        .catch(() => undefined);
    }
  }, [gameState.matchResult]);

  const currentPhase = useMemo(
    () => selectCurrentPhase(gameState),
    [gameState],
  );

  const orderedSeats = useMemo(
    () => selectOrderedSeats(gameState, currentUserId),
    [gameState, currentUserId],
  );
  const meSeat = orderedSeats[0];
  const seatOpponents = orderedSeats.slice(1);

  const players = (matchState as any)?.players ?? [];
  const currentPlayer = useMemo(
    () => players.find((player: any) => player.id === currentUserId) ?? players[0],
    [currentUserId, players],
  );
  const opponents = useMemo(
    () => players.filter((player: any) => player.id !== currentPlayer?.id),
    [currentPlayer?.id, players],
  );
  const hasDetailedHand = Boolean(currentPlayer?.hand?.length);
  const isMyTurn = useMemo(
    () => selectIsMyTurn(gameState, currentUserId),
    [gameState, currentUserId],
  );
  const canAct = useMemo(
    () => selectCanAct(gameState, currentUserId),
    [gameState, currentUserId],
  );
  const canTake = useMemo(
    () => selectCanTake(gameState, currentUserId),
    [gameState, currentUserId],
  );
  const canDefend = useMemo(
    () => selectCanDefend(gameState, currentUserId),
    [gameState, currentUserId],
  );
  const canAttack = useMemo(
    () => selectCanAttack(gameState, currentUserId),
    [gameState, currentUserId],
  );
  const canPass = useMemo(
    () => selectCanPass(gameState, currentUserId),
    [gameState, currentUserId],
  );
  const tablePairs = useMemo(() => {
    if (!matchState) return [];
    if (matchState.tablePiles && matchState.tablePiles.length > 0) {
      return matchState.tablePiles;
    }
    const legacyCards = (matchState as any).tableCards as Card[] | undefined;
    if (legacyCards && legacyCards.length > 0) {
      return buildTablePairs(legacyCards);
    }
    return [];
  }, [matchState]);

  const finish = useMemo(() => {
    if (matchState?.finish) {
      return matchState.finish;
    }
    if (!gameState.matchResult) {
      return null;
    }
    const payouts: Record<string, number> = {};
    for (const item of gameState.matchResult.payouts) {
      payouts[item.userId] = item.amount;
    }
    const places = [...gameState.matchResult.payouts]
      .sort((a, b) => b.amount - a.amount)
      .map((item) => item.userId);
    return {
      bank: gameState.matchResult.pot ?? 0,
      commission: gameState.matchResult.commission ?? 0,
      payouts,
      places,
    };
  }, [gameState.matchResult, matchState?.finish]);
  const mainWinnerId = finish?.places?.[0];
  const winnerName: string | null = useMemo(() => {
    if (!mainWinnerId) return null;
    const fromSeats = matchState?.seats?.find((seat) => seat.id === mainWinnerId)?.name;
    if (fromSeats) return fromSeats;
    const fromPlayers = players.find((player: any) => player.id === mainWinnerId);
    return fromPlayers?.displayName ?? fromPlayers?.username ?? null;
  }, [mainWinnerId, matchState?.seats, players]);

  const myPayout =
    finish && currentUserId ? finish.payouts[currentUserId] ?? 0 : 0;
  const isWinner = !!finish && myPayout > 0;

  const turnPlayerName: string | null = useMemo(() => {
    if (!matchState) return null;
    if (typeof matchState.turnPlayerId === "string" && matchState.turnPlayerId) {
      const turnPlayer = players.find((player: any) => player.id === matchState.turnPlayerId);
      if (turnPlayer) {
        return turnPlayer.displayName ?? turnPlayer.username ?? null;
      }
    }
    if (typeof matchState.turnSeatIndex === "number" && matchState.seats?.[matchState.turnSeatIndex]) {
      return matchState.seats[matchState.turnSeatIndex].name ?? null;
    }
    return null;
  }, [matchState, players]);

  const isAttackerRole = useMemo(
    () => selectIsAttacker(gameState, currentUserId),
    [gameState, currentUserId],
  );
  const isDefenderRole = useMemo(
    () => selectIsDefender(gameState, currentUserId),
    [gameState, currentUserId],
  );

  useEffect(() => {
    if (!id) {
      setIsRoomLoading(false);
      return;
    }
    void getRoom(id)
      .then((data) => setRoom(data))
      .catch(() => setRoom(null))
      .finally(() => setIsRoomLoading(false));
  }, [id]);

  useEffect(() => {
    return subscribeGameStore(setGameState);
  }, []);

  useEffect(() => {
    if (!id) {
      return;
    }

    let cleanup: (() => void) | undefined;
    void joinGameRoom(id).then((dispose) => {
      cleanup = dispose;
    });

    return () => cleanup?.();
  }, [id]);

  useEffect(() => {
    if (!matchState?.turnEndsAt || currentPhase !== "playing") {
      setSecondsLeft(null);
      return;
    }

    const tick = () => {
      const diffMs = matchState.turnEndsAt! - Date.now();
      setSecondsLeft(Math.max(0, Math.ceil(diffMs / 1000)));
    };

    tick();
    const interval = window.setInterval(tick, 1000);
    return () => window.clearInterval(interval);
  }, [currentPhase, matchState?.turnEndsAt]);

  const interactionLocked = gameState.interactionLocked ?? false;
  const swipeTake = useSwipeDown(() => {
    if (canTake && !interactionLocked) {
      hapticImpact("medium");
      sendAction("take");
    }
  });

  function sendAction(action: MatchActionType, cardIdOverride?: string) {
    if (!id || interactionLocked) {
      return;
    }
    const cardId = cardIdOverride ?? selectedCardId ?? undefined;
    if (isMockApiEnabled()) {
      const next = applyMockMatchAction({ roomId: id, action, cardId });
      setMatchState(next);
      addActivity(`Mock action: ${action}`);
      if (action === "attack" || action === "defend") {
        setSelectedCardId(null);
      }
      return;
    }
    const intentPayload: Record<string, unknown> = { roomId: id };
    if (cardId && (action === "attack" || action === "defend")) {
      intentPayload.cardId = cardId;
    }

    hapticImpact("medium");
    wsClient.sendIntent(
      action === "attack"
        ? "playAttack"
        : action === "defend"
          ? "playDefend"
          : action === "take"
            ? "take"
            : action === "pass"
              ? "pass"
              : "pass",
      intentPayload,
    );
    if (action === "attack" || action === "defend") {
      setSelectedCardId(null);
    }
  }

  return (
    <section className="screen game-table-screen">
      <ReconnectOverlay />
      {intentError && (
        <div className="game-intent-error-banner" role="alert" aria-live="polite">
          <div className="game-intent-error-banner__content">
            <span>{intentError.text}</span>
            <button
              type="button"
              className="game-intent-error-banner__close"
              onClick={() => setIntentError(null)}
            >
              ×
            </button>
          </div>
        </div>
      )}
      {interactionLocked && (
        <div className="reconnect-overlay" role="status" aria-live="polite">
          <div className="reconnect-overlay__content">
            <p className="reconnect-overlay__text">Синхронизация…</p>
          </div>
        </div>
      )}
      <div className="page-header">
        <button
          type="button"
          className="icon-button"
          aria-label="Назад"
          onClick={() => {
            if (currentPhase === "playing") {
              setIsExitModalOpen(true);
            } else {
              navigate("/play");
            }
          }}
        >
          <BackIcon size={17} />
        </button>
        <h1 className="page-header__title">Игровой стол</h1>
        <div className="page-header__spacer" />
      </div>

      {isRoomLoading && <CardSkeleton rows={4} />}

      {gameState.status === "connecting" && room && (
        <AppCard>
          <div className="card__hint">Подключение к игре...</div>
        </AppCard>
      )}

      {gameState.status === "error" && room && (
        <AppCard>
          <div className="card__hint card__hint--error">{gameState.error}</div>
          <AppButton
            variant="primary"
            type="button"
            onClick={() => {
              if (!id) return;
              clearGameError();
              setGameConnecting(id);
              wsClient.disconnect();
              wsClient.connect(id);
            }}
          >
            Повторить подключение
          </AppButton>
        </AppCard>
      )}

      {!isRoomLoading && !room && (
        <EmptyStateBlock
          title="Комната не найдена"
          message="Стол недоступен. Вернитесь в список игр и выберите другую комнату."
          actionLabel="К списку игр"
          onAction={() => navigate("/play")}
        />
      )}

      {room && (
        <>
          <AppCard className="game-board">
            <div className="game-board__top">
              <div className="game-board__room">
                <strong>{room.title}</strong>
                <span>${room.stakeUsd}</span>
              </div>
              <div className="game-board__meta">
                <div className="game-board__trump">
                  {formatSuit(matchState?.trumpSuit)}
                </div>
                <span>{secondsLeft !== null ? `${secondsLeft}с` : "..."}</span>
              </div>
            </div>

            <div className="game-board__opponents">
              {seatOpponents.length > 0
                ? seatOpponents.slice(0, 4).map((seat, index) => (
                    <div
                      className="game-opponent"
                      key={seat.id}
                    >
                      <AppAvatar
                        name={seat.name ?? `Игрок ${index + 1}`}
                        photoUrl={seat.avatarUrl}
                        className="game-opponent__avatar"
                      />
                      <div className="game-opponent__name">
                        {seat.name ?? `Игрок ${index + 1}`}
                      </div>
                      <div className="game-opponent__cards" title="Карт на руке">
                        {seat.cardCount}
                      </div>
                    </div>
                  ))
                : opponents.slice(0, 4).map((player: any, index: number) => (
                    <div
                      className="game-opponent"
                      key={player.id}
                    >
                      <AppAvatar
                        name={player.displayName ?? player.username ?? `Игрок ${index + 1}`}
                        photoUrl={player.photoUrl}
                        className="game-opponent__avatar"
                      />
                      <div className="game-opponent__name">
                        {player.displayName ?? player.username ?? `Игрок ${index + 1}`}
                      </div>
                      <div className="game-opponent__cards" title="Карт на руке">
                        {player.handCount}
                      </div>
                    </div>
                  ))}
            </div>

            <div
              ref={tableZoneRef}
              className="game-board__table"
              {...(canTake && !interactionLocked ? swipeTake : {})}
            >
              {tablePairs.length ? (
                <motion.div layout className="table-pairs">
                  {tablePairs.map((pair, index) => {
                    const defenseCard = (pair as any).defense ?? (pair as any).defend;
                    return (
                      <div className="table-pairs__item" key={`pair-${index}`}>
                        <PlayingCard
                          suit={pair.attack.suit}
                          rank={pair.attack.rank}
                          variant="table"
                        />
                        {defenseCard ? (
                          <PlayingCard
                            suit={defenseCard.suit}
                            rank={defenseCard.rank}
                            variant="table"
                          />
                        ) : (
                          <PlayingCard placeholder variant="table" />
                        )}
                      </div>
                    );
                  })}
                </motion.div>
              ) : (
                <div className="game-board__table-empty">Стол пока пуст</div>
              )}
            </div>

            <div className="game-board__turn">
              {isMyTurn
                ? "Ваш ход"
                : turnPlayerName
                  ? `Ход: ${turnPlayerName}`
                  : "Ход соперника"}
              {gameState.error && <span className="game-board__error"> · {gameState.error}</span>}
            </div>
            {gameState.reconnectingPlayerId && gameState.reconnectingPlayerId !== currentUserId && (
              <div className="game-board__reconnect-hint card__hint card__hint--center">
                Соперник отключился. Ожидание 60 сек...
              </div>
            )}
          </AppCard>

          <AppCard className="hand-zone">
            <div className="hand-zone__header">
              <div
                className={`game-opponent game-opponent--me ${isMyTurn ? "game-opponent--turn" : ""} ${isMyTurn && secondsLeft != null && secondsLeft <= 5 ? "game-opponent--urgent" : ""}`}
              >
                <AppAvatar
                  name={meSeat?.name ?? currentPlayer?.displayName ?? currentPlayer?.username ?? "Вы"}
                  photoUrl={meSeat?.avatarUrl ?? currentPlayer?.photoUrl}
                  className="game-opponent__avatar"
                />
                <div className="game-opponent__info">
                  <span className="game-opponent__name">Вы</span>
                  {isAttackerRole && (
                    <span className="game-opponent__role">Атакуете</span>
                  )}
                  {isDefenderRole && !isAttackerRole && (
                    <span className="game-opponent__role">Защищаетесь</span>
                  )}
                </div>
                <div className="game-opponent__cards">
                  {meSeat?.cardCount ?? currentPlayer?.handCount ?? 0}
                </div>
              </div>
            </div>
            <div className="card__label">Ваши карты</div>
            {currentPlayer?.handCount ? (
              hasDetailedHand && !isMockApiEnabled() ? (
                <PlayerHandFan
                  cards={currentPlayer.hand ?? []}
                  matchState={matchState}
                  currentUserId={currentUserId}
                  canAct={canAct && !interactionLocked}
                  interactionLocked={interactionLocked}
                  tableRectRef={tableZoneRef}
                  onPlayCard={(cardId, action) => {
                    sendAction(action === "attack" ? "attack" : "defend", cardId);
                  }}
                />
              ) : (
                <div className="cards-grid cards-grid--hand">
                  {hasDetailedHand
                    ? currentPlayer.hand?.map((card: Card) => (
                        <PlayingCard
                          key={card.id}
                          rank={card.rank}
                          suit={card.suit}
                          variant="hand"
                          selected={selectedCardId === card.id}
                          interactive
                          onClick={() => {
                            hapticSelection();
                            setSelectedCardId(card.id);
                          }}
                        />
                      ))
                    : Array.from({ length: currentPlayer.handCount }).map((_, index) => (
                        <PlayingCard key={`back-${index}`} faceUp={false} variant="hand" />
                      ))}
                </div>
              )
            ) : (
              <div className="card__hint">Карт на руке нет</div>
            )}
          </AppCard>

          <AppCard className="game-actions">
            <div className="action-list action-list--inline">
              <div
                className="action-take-wrap"
                {...(canTake && !interactionLocked ? swipeTake : {})}
              >
                <AppButton
                  variant="primary"
                  type="button"
                  onClick={() => sendAction("take")}
                  disabled={!canTake || interactionLocked}
                >
                  Беру
                </AppButton>
              </div>
              <AppButton
                type="button"
                onClick={() => sendAction("defend")}
                disabled={!canDefend || interactionLocked || (hasDetailedHand && !selectedCardId)}
              >
                Бью
              </AppButton>
              <AppButton
                type="button"
                onClick={() => sendAction("attack")}
                disabled={!canAttack || interactionLocked || (hasDetailedHand && !selectedCardId)}
              >
                Подкинуть
              </AppButton>
            </div>
            <div className="game-actions__secondary">
              <AppButton type="button" onClick={() => sendAction("pass")} disabled={!canPass || interactionLocked}>
                Пас
              </AppButton>
              <Link className="button" to={`/game/${id}/friends`}>
                Друзья
              </Link>
              <AppButton type="button" onClick={() => setIsExitModalOpen(true)}>
                Выйти
              </AppButton>
            </div>
            <div className="game-actions__balance">
              Ваш баланс:{" "}
              {(() => {
                const fromNewBalances =
                  currentUserId && gameState.matchResult?.newBalances?.[currentUserId];
                const bal =
                  fromNewBalances != null ? fromNewBalances : profileBalance;
                return typeof bal === "number"
                  ? `${bal.toFixed(3)} ${currency}`
                  : `— ${currency}`;
              })()}
            </div>
          </AppCard>

          <AppCard>
            <div className="card__label">Лог событий</div>
            {gameState.activity.length ? (
              <div className="list">
                {gameState.activity.slice(-10).map((item, index) => (
                  <div
                    className="card__hint"
                    key={
                      item.type === "move"
                        ? item.eventId ?? `m-${index}`
                        : `s-${item.timestamp}-${index}`
                    }
                  >
                    {formatActivityItem(item, players)}
                  </div>
                ))}
              </div>
            ) : (
              <div className="card__hint">Событий пока нет</div>
            )}
          </AppCard>

          {finish && (
            <div className="result-card">
              <div className="result-card__title">
                {isWinner ? "Победа!" : "Игра завершена"}
              </div>
              <div className="result-card__message">
                {isWinner && gameState.matchFinishedAbandoned
                  ? "Соперник не вернулся. Победа."
                  : winnerName
                    ? `Победитель: ${winnerName}`
                    : "Ожидание итогового расчета."}
              </div>
              <div className="result-card__summary">
                <div>Банк: {finish.bank.toFixed(2)} USD</div>
                <div>Комиссия: {finish.commission.toFixed(2)} USD</div>
                <div>Ваш выигрыш: {myPayout.toFixed(2)} USD</div>
              </div>
              <div className="result-card__places">
                <div className="result-card__places-title">Результаты игроков</div>
                <div className="result-card__places-list">
                  {finish.places.map((playerId, index) => {
                    const seat = matchState?.seats?.find((s) => s.id === playerId);
                    const name = seat?.name ?? playerId;
                    const payout = finish.payouts[playerId] ?? 0;
                    const placeNumber = index + 1;
                    const isMeRow = currentUserId === playerId;
                    return (
                      <div
                        key={playerId}
                        className={`result-card__place-row ${isMeRow ? "result-card__place-row--me" : ""}`}
                      >
                        <div className="result-card__place-left">
                          <span className="result-card__place-number">{placeNumber}</span>
                          <span className="result-card__place-name">
                            {name}
                            {isMeRow ? " (Вы)" : ""}
                          </span>
                        </div>
                        <div className="result-card__place-right">
                          {payout !== 0 ? `${payout.toFixed(2)} USD` : "—"}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
              <button
                className="button button--primary"
                type="button"
                onClick={() => {
                  hapticNotification(isWinner ? "success" : "warning");
                  navigate(isWinner ? "/finish/win" : "/finish/lose");
                }}
              >
                Продолжить
              </button>
            </div>
          )}
        </>
      )}

      <ConfirmModal
        isOpen={isExitModalOpen}
        title="Выйти из игры?"
        message="Вы покинули активную игру. Вернуться?"
        cancelLabel="Вернуться"
        confirmLabel="Покинуть"
        onCancel={() => setIsExitModalOpen(false)}
        onConfirm={() => {
          if (id) {
            void leaveRoom(id).then(() => navigate("/play")).catch(() => navigate("/play"));
          } else {
            navigate("/play");
          }
        }}
      />
    </section>
  );
}

function getSuitSymbol(suit: string) {
  const symbols: Record<string, string> = {
    hearts: "♥",
    diamonds: "♦",
    clubs: "♣",
    spades: "♠",
  };
  return symbols[suit] ?? "?";
}

function formatSuit(suit?: string) {
  if (!suit) {
    return "-";
  }
  return `${getSuitSymbol(suit)} ${suit}`;
}

function buildTablePairs(cards: Card[]) {
  const pairs: Array<{
    attack: { id: string; suit: string; rank: string };
    defense?: { id: string; suit: string; rank: string };
  }> = [];

  for (let index = 0; index < cards.length; index += 2) {
    pairs.push({
      attack: cards[index],
      defense: cards[index + 1],
    });
  }

  return pairs;
}

function toWireAction(action: MatchActionType) {
  switch (action) {
    case "attack":
      return "attack_card";
    case "defend":
      return "defend_card";
    case "take":
      return "take_cards";
    case "pass":
      return "pass_turn";
    default:
      return action;
  }
}
