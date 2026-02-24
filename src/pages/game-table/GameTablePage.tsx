import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { motion } from "framer-motion";
import { applyMockMatchAction, isMockApiEnabled } from "@/mocks/mockApi";
import type { MatchActionType, Room } from "@/entities/match/types";
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

  const matchState = gameState.matchState;
  useEffect(() => {
    if (matchState?.status === "finished") {
      void getProfile()
        .then((r) => setProfileBalance(r.balance))
        .catch(() => undefined);
    }
  }, [matchState?.status]);
  const players = matchState?.players ?? [];
  const currentPlayer = useMemo(
    () => players.find((player) => player.id === currentUserId) ?? players[0],
    [currentUserId, players],
  );
  const opponents = useMemo(
    () => players.filter((player) => player.id !== currentPlayer?.id),
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
  const tablePairs = useMemo(() => buildTablePairs(matchState?.tableCards ?? []), [matchState?.tableCards]);
  const winnerName = useMemo(() => {
    if (!matchState?.winnerPlayerId) {
      return null;
    }
    const winner = players.find((player) => player.id === matchState.winnerPlayerId);
    return winner?.displayName ?? winner?.username ?? "Игрок";
  }, [matchState?.winnerPlayerId, players]);

  const turnPlayerName = useMemo(() => {
    if (!matchState?.turnPlayerId) return null;
    const p = players.find((x) => x.id === matchState.turnPlayerId);
    return p?.displayName ?? p?.username ?? "Игрок";
  }, [matchState?.turnPlayerId, players]);
  const isWinner = matchState?.winnerPlayerId && currentPlayer
    ? matchState.winnerPlayerId === currentPlayer.id
    : false;

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
    if (!matchState?.turnEndsAt || matchState.status !== "playing") {
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
  }, [matchState?.status, matchState?.turnEndsAt, matchState?.turnPlayerId]);

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
    const selectedCard =
      action === "attack" || action === "defend" ? { cardId } : {};
    const expectedVersion =
      matchState?.version != null ? { expectedVersion: matchState.version } : {};
    const actionId = crypto.randomUUID();

    hapticImpact("medium");
    wsClient.send({
      type: "make_move",
      payload: {
        roomId: id,
        action: toWireAction(action),
        ...selectedCard,
        ...expectedVersion,
        actionId,
      },
    });
    if (action === "attack" || action === "defend") {
      setSelectedCardId(null);
    }
  }

  return (
    <section className="screen game-table-screen">
      <ReconnectOverlay />
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
            if (matchState?.status === "playing") {
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
                  {matchState?.trumpCard ? (
                    <PlayingCard
                      rank={matchState.trumpCard.rank}
                      suit={matchState.trumpCard.suit}
                      variant="mini"
                    />
                  ) : (
                    formatSuit(matchState?.trumpSuit)
                  )}
                </div>
                <span>{secondsLeft !== null ? `${secondsLeft}с` : "..."}</span>
              </div>
            </div>

            <div className="game-board__opponents">
              {opponents.slice(0, 4).map((player, index) => (
                <div
                  className={`game-opponent ${matchState?.turnPlayerId === player.id ? "game-opponent--turn" : ""} ${matchState?.turnPlayerId === player.id && secondsLeft != null && secondsLeft <= 5 ? "game-opponent--urgent" : ""}`}
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
                  {tablePairs.map((pair, index) => (
                    <div className="table-pairs__item" key={`pair-${index}`}>
                      <PlayingCard
                        suit={pair.attack.suit}
                        rank={pair.attack.rank}
                        variant="table"
                      />
                      {pair.defense ? (
                        <PlayingCard
                          suit={pair.defense.suit}
                          rank={pair.defense.rank}
                          variant="table"
                        />
                      ) : (
                        <PlayingCard placeholder variant="table" />
                      )}
                    </div>
                  ))}
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
                  name={currentPlayer?.displayName ?? currentPlayer?.username ?? "Вы"}
                  photoUrl={currentPlayer?.photoUrl}
                  className="game-opponent__avatar"
                />
                <span className="game-opponent__name">Вы</span>
                {currentPlayer && (
                  <div className="game-opponent__cards">{currentPlayer.handCount}</div>
                )}
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
                    ? currentPlayer.hand?.map((card) => (
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

          {matchState?.status === "finished" && (
            <div className="result-card">
              <div className="result-card__title">{isWinner ? "Победа!" : "Игра завершена"}</div>
              <div className="result-card__message">
                {isWinner && gameState.matchFinishedAbandoned
                  ? "Соперник не вернулся. Победа."
                  : winnerName
                    ? `Победитель: ${winnerName}`
                    : "Ожидание итогового расчета."}
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

function buildTablePairs(cards: Array<{ id: string; suit: string; rank: string }>) {
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
