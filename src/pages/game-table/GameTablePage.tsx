import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { motion } from "framer-motion";
import { applyMockMatchAction, isMockApiEnabled } from "@/mocks/mockApi";
import type { MatchActionType, Room } from "@/entities/match/types";
import type { IntentType } from "@/shared/api/ws/types";
import type { Card } from "@/entities/card/types";
import { joinGameRoom } from "@/processes/joinGame.process";
import { getRoom, leaveRoom } from "@/shared/api/rooms";
import { getProfile } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
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
import { onWsEvent } from "@/shared/api/ws/events";
import {
  addActivity,
  clearGameError,
  getGameState,
  setGameConnecting,
  setMatchState,
  subscribeGameStore,
} from "@/store/game.store";

type ChatReaction = {
  id: string;
  userId: string;
  message: string;
  createdAt: number;
};

const QUICK_CHAT_REACTIONS: Record<"ru" | "uk", string[]> = {
  ru: ["Беру!", "Бито!", "Пас!", "Ход!"],
  uk: ["Беру!", "Бито!", "Пас!", "Хід!"],
};

export function GameTablePage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);
  const [room, setRoom] = useState<Room | null>(null);
  const [gameState, setGameState] = useState(getGameState());
  const [isRoomLoading, setIsRoomLoading] = useState(true);
  const [isExitModalOpen, setIsExitModalOpen] = useState(false);
  const [selectedCardId, setSelectedCardId] = useState<string | null>(null);
  const [secondsLeft, setSecondsLeft] = useState<number | null>(null);
  const [currentUserId, setCurrentUserId] = useState<string | null>(null);
  const [profileBalance, setProfileBalance] = useState<number | null>(null);
  const [intentError, setIntentError] = useState<{ text: string; code?: string } | null>(null);
  const [chatInput, setChatInput] = useState("");
  const [chatReactions, setChatReactions] = useState<ChatReaction[]>([]);
  const currency = "USD";
  const tableZoneRef = useRef<HTMLDivElement>(null);
  const quickChatReactions = QUICK_CHAT_REACTIONS[language];

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
      const text = detail.text || tr("Ошибка выполнения действия.", "Помилка виконання дії.");
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
  }, [language, navigate]);

  useEffect(() => {
    const offChat = onWsEvent("chat_message", ({ payload }) => {
      const message = payload?.message?.trim();
      const userId = payload?.userId?.trim();
      if (!message || !userId) {
        return;
      }
      const reaction: ChatReaction = {
        id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
        userId,
        message,
        createdAt: Date.now(),
      };
      setChatReactions((prev) => [...prev, reaction].slice(-16));
    });

    return () => {
      offChat();
    };
  }, []);

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      const now = Date.now();
      setChatReactions((prev) => prev.filter((item) => now - item.createdAt < 12000));
    }, 1000);
    return () => {
      window.clearInterval(intervalId);
    };
  }, []);

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
  const playerNameById = useMemo(() => {
    const map: Record<string, string> = {};
    for (const player of players) {
      if (!player?.id) {
        continue;
      }
      map[player.id] = player.displayName ?? player.username ?? player.id;
    }
    return map;
  }, [players]);
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
  const shulerWindowOpen = Boolean(matchState?.shuler?.isWindowOpen);
  const shulerPlayers = useMemo(
    () => new Set(matchState?.shuler?.activePlayers ?? []),
    [matchState?.shuler?.activePlayers],
  );
  const canReportShuler = Boolean(currentUserId) && shulerWindowOpen && !shulerPlayers.has(currentUserId ?? "");
  const isTranslateMode = useMemo(() => {
    const raw = (matchState?.mode ?? room?.mode ?? "").toLowerCase();
    return raw.includes("perevod") || raw.includes("перевод");
  }, [matchState?.mode, room?.mode]);
  const canTranslate = canDefend && isTranslateMode;
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
      const mockAction =
        action === "translate"
          ? "defend"
          : action === "shuler_report"
            ? "pass"
            : action;
      const next = applyMockMatchAction({ roomId: id, action: mockAction, cardId });
      setMatchState(next);
      addActivity(`Mock action: ${action}`);
      if (action === "attack" || action === "defend" || action === "translate") {
        setSelectedCardId(null);
      }
      return;
    }

    const intentByAction: Record<MatchActionType, IntentType> = {
      attack: "playAttack",
      defend: "playDefend",
      take: "take",
      pass: "pass",
      translate: "translate",
      shuler_report: "shulerReport",
    };
    const intent = intentByAction[action];
    if (!intent) {
      return;
    }

    const intentPayload: Record<string, unknown> = { roomId: id };
    if (cardId && (action === "attack" || action === "defend" || action === "translate")) {
      intentPayload.cardId = cardId;
    }

    hapticImpact("medium");
    wsClient.sendIntent(intent, intentPayload);
    if (action === "attack" || action === "defend" || action === "translate") {
      setSelectedCardId(null);
    }
  }

  function sendChatMessage(rawMessage: string) {
    if (!id) {
      return;
    }
    const message = rawMessage.trim();
    if (!message) {
      return;
    }
    wsClient.send({
      type: "send_message",
      payload: {
        roomId: id,
        message,
      },
    });
    setChatInput("");
    hapticSelection();
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
            <p className="reconnect-overlay__text">{tr("Синхронизация…", "Синхронізація…")}</p>
          </div>
        </div>
      )}
      <div className="page-header">
        <button
          type="button"
          className="icon-button"
          aria-label={tr("Назад", "Назад")}
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
        <h1 className="page-header__title">{tr("Игровой стол", "Ігровий стіл")}</h1>
        <div className="page-header__spacer" />
      </div>

      {isRoomLoading && <CardSkeleton rows={4} />}

      {gameState.status === "connecting" && room && (
        <AppCard>
          <div className="card__hint">{tr("Подключение к игре...", "Підключення до гри...")}</div>
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
            {tr("Повторить подключение", "Повторити підключення")}
          </AppButton>
        </AppCard>
      )}

      {!isRoomLoading && !room && (
        <EmptyStateBlock
          title={tr("Комната не найдена", "Кімнату не знайдено")}
          message={tr(
            "Стол недоступен. Вернитесь в список игр и выберите другую комнату.",
            "Стіл недоступний. Поверніться до списку ігор та виберіть іншу кімнату.",
          )}
          actionLabel={tr("К списку игр", "До списку ігор")}
          onAction={() => navigate("/play")}
        />
      )}

      {room && (
        <>
          <AppCard className="game-board">
            <div className="game-board__top">
              <div className="game-board__room">
                <strong>{room.title}</strong>
                <span>
                  ${room.stakeUsd} · {room.mode} · {currency}
                </span>
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
                        name={seat.name ?? tr(`Игрок ${index + 1}`, `Гравець ${index + 1}`)}
                        photoUrl={seat.avatarUrl}
                        className="game-opponent__avatar"
                      />
                      <div className="game-opponent__name">
                        {seat.name ?? tr(`Игрок ${index + 1}`, `Гравець ${index + 1}`)}
                      </div>
                      {shulerPlayers.has(seat.id) && (
                        <div className="game-opponent__badge game-opponent__badge--shuler">
                          {tr("Шулер", "Шулер")}
                        </div>
                      )}
                      <div className="game-opponent__cards" title={tr("Карт на руке", "Карт на руці")}>
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
                        name={player.displayName ?? player.username ?? tr(`Игрок ${index + 1}`, `Гравець ${index + 1}`)}
                        photoUrl={player.photoUrl}
                        className="game-opponent__avatar"
                      />
                      <div className="game-opponent__name">
                        {player.displayName ?? player.username ?? tr(`Игрок ${index + 1}`, `Гравець ${index + 1}`)}
                      </div>
                      {shulerPlayers.has(player.id) && (
                        <div className="game-opponent__badge game-opponent__badge--shuler">
                          {tr("Шулер", "Шулер")}
                        </div>
                      )}
                      <div className="game-opponent__cards" title={tr("Карт на руке", "Карт на руці")}>
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
                <div className="game-board__table-empty">{tr("Стол пока пуст", "Стіл поки порожній")}</div>
              )}
            </div>

            <div className="game-board__turn">
              {isMyTurn
                ? tr("Ваш ход", "Ваш хід")
                : turnPlayerName
                  ? tr(`Ход: ${turnPlayerName}`, `Хід: ${turnPlayerName}`)
                  : tr("Ход соперника", "Хід суперника")}
              {gameState.error && <span className="game-board__error"> · {gameState.error}</span>}
            </div>
            {gameState.reconnectingPlayerId && gameState.reconnectingPlayerId !== currentUserId && (
              <div className="game-board__reconnect-hint card__hint card__hint--center">
                {tr("Соперник отключился. Ожидание 60 сек...", "Суперник відключився. Очікування 60 сек...")}
              </div>
            )}
          </AppCard>

          <AppCard className="hand-zone">
            <div className="hand-zone__header">
              <div
                className={`game-opponent game-opponent--me ${isMyTurn ? "game-opponent--turn" : ""} ${isMyTurn && secondsLeft != null && secondsLeft <= 5 ? "game-opponent--urgent" : ""}`}
              >
                <AppAvatar
                  name={meSeat?.name ?? currentPlayer?.displayName ?? currentPlayer?.username ?? tr("Вы", "Ви")}
                  photoUrl={meSeat?.avatarUrl ?? currentPlayer?.photoUrl}
                  className="game-opponent__avatar"
                />
                <div className="game-opponent__info">
                  <span className="game-opponent__name">{tr("Вы", "Ви")}</span>
                  {isAttackerRole && (
                    <span className="game-opponent__role">{tr("Атакуете", "Атакуєте")}</span>
                  )}
                  {isDefenderRole && !isAttackerRole && (
                    <span className="game-opponent__role">{tr("Защищаетесь", "Захищаєтесь")}</span>
                  )}
                  {currentUserId && shulerPlayers.has(currentUserId) && (
                    <span className="game-opponent__badge game-opponent__badge--shuler">{tr("Шулер", "Шулер")}</span>
                  )}
                </div>
                <div className="game-opponent__cards">
                  {meSeat?.cardCount ?? currentPlayer?.handCount ?? 0}
                </div>
              </div>
            </div>
            <div className="card__label">{tr("Ваши карты", "Ваші карти")}</div>
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
              <div className="card__hint">{tr("Карт на руке нет", "Карт на руці немає")}</div>
            )}
          </AppCard>

          <AppCard className="game-actions">
            <div className="action-list action-list--inline">
              <AppButton
                type="button"
                onClick={() => sendAction("attack")}
                disabled={!canAttack || interactionLocked || (hasDetailedHand && !selectedCardId)}
              >
                {tr("Ход", "Хід")}
              </AppButton>
              <AppButton
                type="button"
                onClick={() => sendAction("defend")}
                disabled={!canDefend || interactionLocked || (hasDetailedHand && !selectedCardId)}
              >
                {tr("Защита", "Захист")}
              </AppButton>
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
                  {tr("Беру", "Беру")}
                </AppButton>
              </div>
            </div>
            <div className="game-actions__secondary">
              <AppButton
                type="button"
                onClick={() => sendAction("pass")}
                disabled={!canPass || interactionLocked}
              >
                {tr("Бито", "Бито")}
              </AppButton>
              {isTranslateMode && (
                <AppButton
                  type="button"
                  onClick={() => sendAction("translate")}
                  disabled={!canTranslate || interactionLocked || (hasDetailedHand && !selectedCardId)}
                >
                  {tr("Перевести", "Перевести")}
                </AppButton>
              )}
              {canReportShuler && (
                <AppButton
                  type="button"
                  onClick={() => sendAction("shuler_report")}
                  disabled={interactionLocked}
                >
                  {tr("Сообщить о шулере", "Повідомити про шулера")}
                </AppButton>
              )}
              <Link className="button" to={`/game/${id}/friends`}>
                {tr("Друзья", "Друзі")}
              </Link>
              <AppButton type="button" onClick={() => setIsExitModalOpen(true)}>
                {tr("Выйти", "Вийти")}
              </AppButton>
            </div>
            <div className="game-actions__balance">
              {tr("Ваш баланс", "Ваш баланс")}:{" "}
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

          <AppCard className="game-chat">
            <div className="card__label">{tr("Чат и реакции", "Чат і реакції")}</div>
            <div className="game-chat__quick">
              {quickChatReactions.map((item) => (
                <button
                  key={item}
                  type="button"
                  className="pill"
                  onClick={() => sendChatMessage(item)}
                >
                  {item}
                </button>
              ))}
            </div>
            <div className="game-chat__feed">
              {chatReactions.length > 0 ? (
                chatReactions.slice(-4).map((item) => (
                  <div key={item.id} className="game-chat__item">
                    <strong>{playerNameById[item.userId] ?? tr("Игрок", "Гравець")}:</strong> {item.message}
                  </div>
                ))
              ) : (
                <div className="card__hint">{tr("Сообщений пока нет", "Повідомлень поки немає")}</div>
              )}
            </div>
            <div className="game-chat__composer">
              <input
                value={chatInput}
                onChange={(event) => setChatInput(event.target.value)}
                maxLength={120}
                placeholder={tr("Введите сообщение...", "Введіть повідомлення...")}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    sendChatMessage(chatInput);
                  }
                }}
              />
              <AppButton type="button" onClick={() => sendChatMessage(chatInput)}>
                {tr("Отправить", "Надіслати")}
              </AppButton>
            </div>
          </AppCard>

          <AppCard>
            <div className="card__label">{tr("Лог событий", "Лог подій")}</div>
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
              <div className="card__hint">{tr("Событий пока нет", "Подій поки немає")}</div>
            )}
          </AppCard>

          {finish && (
            <div className="result-card">
              <div className="result-card__title">
                {isWinner ? tr("Победа!", "Перемога!") : tr("Игра завершена", "Гру завершено")}
              </div>
              <div className="result-card__message">
                {isWinner && gameState.matchFinishedAbandoned
                  ? tr("Соперник не вернулся. Победа.", "Суперник не повернувся. Перемога.")
                  : winnerName
                    ? tr(`Победитель: ${winnerName}`, `Переможець: ${winnerName}`)
                    : tr("Ожидание итогового расчета.", "Очікування підсумкового розрахунку.")}
              </div>
              <div className="result-card__summary">
                <div>{tr("Банк", "Банк")}: {finish.bank.toFixed(2)} USD</div>
                <div>{tr("Комиссия", "Комісія")}: {finish.commission.toFixed(2)} USD</div>
                <div>{tr("Ваш выигрыш", "Ваш виграш")}: {myPayout.toFixed(2)} USD</div>
              </div>
              <div className="result-card__places">
                <div className="result-card__places-title">{tr("Результаты игроков", "Результати гравців")}</div>
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
                            {isMeRow ? tr(" (Вы)", " (Ви)") : ""}
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
                {tr("Продолжить", "Продовжити")}
              </button>
            </div>
          )}
        </>
      )}

      <ConfirmModal
        isOpen={isExitModalOpen}
        title={tr("Выйти из игры?", "Вийти з гри?")}
        message={tr("Вы покинули активную игру. Вернуться?", "Ви покидаєте активну гру. Повернутися?")}
        cancelLabel={tr("Вернуться", "Повернутися")}
        confirmLabel={tr("Покинуть", "Покинути")}
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
