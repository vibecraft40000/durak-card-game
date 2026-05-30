import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { applyMockMatchAction, isMockApiEnabled } from "@/mocks/mockApi";
import type { MatchActionType } from "@/entities/match/types";
import type { IntentType, MatchAffordances } from "@/shared/api/ws/types";
import type { Card } from "@/entities/card/types";
import { leaveRoom } from "@/shared/api/rooms";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { hapticImpact, hapticNotification, hapticSelection } from "@/shared/lib/telegram";
import { wsClient } from "@/shared/api/ws/socket";
import { AppAvatar } from "@/shared/ui/Avatar";
import { BackIcon, CloseIcon } from "@/shared/ui/Icons";
import { AppCard } from "@/shared/ui/Card";
import { PlayingCard } from "@/shared/ui/PlayingCard";
import { AppButton } from "@/shared/ui/Button";
import { ReconnectOverlay } from "@/shared/ui/ReconnectOverlay";
import { CardSkeleton, ConfirmModal, EmptyStateBlock } from "@/shared/ui/StateBlocks";
import { PlayerHandFan } from "@/features/game/PlayerHandFan";
import {
  selectCanAct,
  selectCanAttack,
  selectCanDefend,
  selectCanPass,
  selectCanShulerPlay,
  selectCanTake,
  selectCanThrow,
  selectCurrentPhase,
  selectIsAttacker,
  selectIsDefender,
  selectIsMyTurn,
  selectOrderedSeats,
} from "@/entities/game/model/selectors";
import { useSwipeDown } from "@/shared/hooks/useSwipeDown";
import { addActivity, clearGameError, setGameConnecting, setMatchState } from "@/store/game.store";
import { useGameTableSession } from "./hooks/useGameTableSession";

type SeatViewModel = {
  id: string;
  name: string;
  avatarUrl?: string;
  cardCount: number;
  roleLabel: string | null;
  isShuler: boolean;
};

type TablePair = {
  attack: { id: string; suit: string; rank: string };
  defense?: { id: string; suit: string; rank: string };
};

type ActionDescriptor = {
  key: string;
  label: string;
  variant?: "primary" | "default" | "secondary" | "ghost";
  disabled?: boolean;
  onClick: () => void;
};

export function GameTablePage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);
  const { room, isRoomLoading, gameState, currentUserId, profileBalance } = useGameTableSession(id);
  const [isExitModalOpen, setIsExitModalOpen] = useState(false);
  const [selectedCardId, setSelectedCardId] = useState<string | null>(null);
  const [secondsLeft, setSecondsLeft] = useState<number | null>(null);
  const [shulerWindowMsLeft, setShulerWindowMsLeft] = useState(0);
  const [intentError, setIntentError] = useState<{ text: string; code?: string } | null>(null);
  const currency = "USD";
  const tableZoneRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onIntentError(ev: Event) {
      const ce = ev as CustomEvent;
      const detail = (ce.detail ?? {}) as { text?: string; code?: string };
      setIntentError({
        text:
          detail.text ||
          tr("Не удалось выполнить действие. Попробуйте еще раз.", "Не вдалося виконати дію. Спробуйте ще раз."),
        code: detail.code,
      });

      if (detail.code === "kicked") {
        window.setTimeout(() => navigate("/play"), 800);
      }
    }

    window.addEventListener("tma:intentError", onIntentError as EventListener);
    return () => window.removeEventListener("tma:intentError", onIntentError as EventListener);
  }, [language, navigate]);

  const matchState = gameState.matchState;
  const affordances = matchState?.affordances;
  const currentPhase = useMemo(() => selectCurrentPhase(gameState), [gameState]);
  const orderedSeats = useMemo(() => selectOrderedSeats(gameState, currentUserId), [gameState, currentUserId]);
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
  const isMyTurn = useMemo(() => selectIsMyTurn(gameState, currentUserId), [gameState, currentUserId]);
  const canAct = useMemo(() => selectCanAct(gameState, currentUserId), [gameState, currentUserId]);
  const canTake = useMemo(() => selectCanTake(gameState, currentUserId), [gameState, currentUserId]);
  const canDefend = useMemo(() => selectCanDefend(gameState, currentUserId), [gameState, currentUserId]);
  const canAttack = useMemo(() => selectCanAttack(gameState, currentUserId), [gameState, currentUserId]);
  const canPass = useMemo(() => selectCanPass(gameState, currentUserId), [gameState, currentUserId]);
  const canThrow = useMemo(() => selectCanThrow(gameState, currentUserId), [gameState, currentUserId]);
  const canShulerPlay = useMemo(() => selectCanShulerPlay(gameState, currentUserId), [gameState, currentUserId]);
  const isAttackerRole = useMemo(() => selectIsAttacker(gameState, currentUserId), [gameState, currentUserId]);
  const isDefenderRole = useMemo(() => selectIsDefender(gameState, currentUserId), [gameState, currentUserId]);

  const shulerWindowOpen = Boolean(matchState?.shuler?.isWindowOpen);
  const shulerWindowEndsAt = useMemo(() => {
    const raw = matchState?.shuler?.windowEndsAt;
    return typeof raw === "number" && raw > 0 ? raw : null;
  }, [matchState?.shuler?.windowEndsAt]);
  const shulerWindowIsLive = shulerWindowOpen && (shulerWindowEndsAt == null ? true : shulerWindowMsLeft > 0);
  const shulerWindowSecondsLeft = shulerWindowEndsAt == null ? null : Math.max(0, Math.ceil(shulerWindowMsLeft / 1000));
  const shulerPlayers = useMemo(() => new Set(matchState?.shuler?.activePlayers ?? []), [matchState?.shuler?.activePlayers]);
  const canReportShuler = useMemo(() => {
    if (typeof affordances?.canShulerReport === "boolean") return affordances.canShulerReport;
    return Boolean(currentUserId) && shulerWindowIsLive && !shulerPlayers.has(currentUserId ?? "");
  }, [affordances?.canShulerReport, currentUserId, shulerPlayers, shulerWindowIsLive]);
  const isTranslateMode = useMemo(() => {
    const raw = `${matchState?.mode ?? room?.mode ?? ""}`.toLowerCase();
    return raw.includes("perevod") || raw.includes("перевод") || raw.includes("перекид");
  }, [matchState?.mode, room?.mode]);
  const canTranslate = useMemo(() => {
    if (typeof affordances?.canTranslate === "boolean") return affordances.canTranslate;
    return canDefend && isTranslateMode;
  }, [affordances?.canTranslate, canDefend, isTranslateMode]);
  const tablePairs = useMemo<TablePair[]>(() => {
    if (!matchState) return [];
    if (matchState.tablePiles?.length) return matchState.tablePiles;
    const legacyCards = (matchState as any).tableCards as Card[] | undefined;
    return legacyCards?.length ? buildTablePairs(legacyCards) : [];
  }, [matchState]);
  const finish = useMemo(() => {
    if (matchState?.finish) return matchState.finish;
    if (!gameState.matchResult) return null;
    const payouts: Record<string, number> = {};
    for (const item of gameState.matchResult.netResults) payouts[item.userId] = item.amount;
    const places = [...gameState.matchResult.netResults].sort((a, b) => b.amount - a.amount).map((item) => item.userId);
    return { bank: gameState.matchResult.pot ?? 0, commission: gameState.matchResult.commission ?? 0, payouts, places };
  }, [gameState.matchResult, matchState?.finish]);

  const mainWinnerId = finish?.places?.[0];
  const winnerName = useMemo(() => {
    if (!mainWinnerId) return null;
    const fromSeats = matchState?.seats?.find((seat) => seat.id === mainWinnerId)?.name;
    if (fromSeats) return fromSeats;
    const fromPlayers = players.find((player: any) => player.id === mainWinnerId);
    return fromPlayers?.displayName ?? fromPlayers?.username ?? null;
  }, [mainWinnerId, matchState?.seats, players]);

  const myPayout = finish && currentUserId ? finish.payouts[currentUserId] ?? 0 : 0;
  const isWinner = Boolean(finish) && myPayout > 0;
  const turnPlayerName = useMemo(() => {
    if (!matchState) return null;
    if (typeof matchState.turnPlayerId === "string" && matchState.turnPlayerId) {
      const turnPlayer = players.find((player: any) => player.id === matchState.turnPlayerId);
      if (turnPlayer) return turnPlayer.displayName ?? turnPlayer.username ?? null;
    }
    if (typeof matchState.turnSeatIndex === "number" && matchState.seats?.[matchState.turnSeatIndex]) {
      return matchState.seats[matchState.turnSeatIndex].name ?? null;
    }
    return null;
  }, [matchState, players]);

  const attackerId =
    matchState?.attackerPlayerId ??
    (typeof matchState?.attackerSeat === "number" ? matchState?.seats?.[matchState.attackerSeat]?.id : null) ??
    null;
  const defenderId =
    matchState?.defenderPlayerId ??
    (typeof matchState?.defenderSeat === "number" ? matchState?.seats?.[matchState.defenderSeat]?.id : null) ??
    null;

  useEffect(() => {
    if (!matchState?.turnEndsAt || currentPhase !== "playing") {
      setSecondsLeft(null);
      return;
    }
    const tick = () => setSecondsLeft(Math.max(0, Math.ceil((matchState.turnEndsAt ?? 0) - Date.now()) / 1000));
    tick();
    const interval = window.setInterval(tick, 1000);
    return () => window.clearInterval(interval);
  }, [currentPhase, matchState?.turnEndsAt]);

  useEffect(() => {
    if (!shulerWindowEndsAt) {
      setShulerWindowMsLeft(0);
      return;
    }
    const tick = () => setShulerWindowMsLeft(Math.max(0, shulerWindowEndsAt - Date.now()));
    tick();
    const interval = window.setInterval(tick, 200);
    return () => window.clearInterval(interval);
  }, [shulerWindowEndsAt]);

  const interactionLocked = gameState.interactionLocked ?? false;
  const swipeTake = useSwipeDown(() => {
    if (canTake && !interactionLocked) {
      hapticImpact("medium");
      sendAction("take");
    }
  });

  const topBalance =
    currentUserId && gameState.matchResult?.newBalances?.[currentUserId] != null
      ? gameState.matchResult.newBalances[currentUserId]
      : profileBalance;
  const balanceLabel = topBalance == null ? `— ${currency}` : `${topBalance.toFixed(2)} ${currency}`;
  const selectedCard = currentPlayer?.hand?.find((card: Card) => card.id === selectedCardId) ?? null;
  const roomModeLabel = formatModeLabel(matchState?.mode ?? room?.mode, tr);
  const fairnessLabel = formatFairnessLabel(matchState?.mode ?? room?.mode, tr);
  const deckLabel = formatDeckLabel(matchState?.deckType ?? room?.deck, tr);
  const playersLabel = formatPlayersLabel((room?.players || matchState?.seats?.length || room?.maxPlayers || 0), room?.maxPlayers ?? matchState?.seats?.length ?? 0, tr);
  const trumpLabel = formatSuitLabel(matchState?.trumpSuit, tr);
  const stockLabel = typeof matchState?.stockCount === "number" ? tr(`Колода ${matchState.stockCount}`, `Колода ${matchState.stockCount}`) : tr("Колода скрыта", "Колода прихована");
  const turnLabel = isMyTurn ? tr("Ваш ход", "Ваш хід") : turnPlayerName ? tr(`Ходит ${turnPlayerName}`, `Ходить ${turnPlayerName}`) : tr("Ожидайте ход", "Очікуйте хід");
  const tableEmptyLabel = isMyTurn ? tr("Перетащите карту на стол", "Перетягніть карту на стіл") : tr("Стол пока пуст", "Стіл поки порожній");
  const handHint = hasDetailedHand && canAct
    ? tr("Перетащите карту на стол или выберите ее ниже.", "Перетягніть карту на стіл або виберіть її нижче.")
    : selectedCard
      ? tr(`Выбрана карта ${selectedCard.rank}${getSuitSymbol(selectedCard.suit)}`, `Вибрано карту ${selectedCard.rank}${getSuitSymbol(selectedCard.suit)}`)
      : tr("Выберите карту для действия.", "Виберіть карту для дії.");

  const opponentSeats = useMemo<SeatViewModel[]>(() => {
    if (seatOpponents.length > 0) {
      return seatOpponents.slice(0, 4).map((seat, index) => ({
        id: seat.id,
        name: seat.name ?? fallbackPlayerName(index + 2, tr),
        avatarUrl: seat.avatarUrl,
        cardCount: seat.cardCount,
        roleLabel: getRoleLabel(seat.id, attackerId, defenderId, tr),
        isShuler: shulerPlayers.has(seat.id),
      }));
    }
    return opponents.slice(0, 4).map((player: any, index: number) => ({
      id: player.id,
      name: player.displayName ?? player.username ?? fallbackPlayerName(index + 2, tr),
      avatarUrl: player.photoUrl,
      cardCount: player.handCount ?? player.hand?.length ?? 0,
      roleLabel: getRoleLabel(player.id, attackerId, defenderId, tr),
      isShuler: shulerPlayers.has(player.id),
    }));
  }, [attackerId, defenderId, opponents, seatOpponents, shulerPlayers, tr]);

  const primaryActions = useMemo<ActionDescriptor[]>(() => {
    const actions: ActionDescriptor[] = [];
    if (canAttack || isAttackerRole || Boolean(affordances?.attackCardIds?.length)) {
      actions.push({
        key: "attack",
        label: tr("Ход", "Хід"),
        variant: "primary",
        disabled:
          !canAttack ||
          interactionLocked ||
          !isCardAllowedForAction("attack", selectedCardId ?? undefined, hasDetailedHand, affordances),
        onClick: () => sendAction("attack"),
      });
    }
    if (canDefend || isDefenderRole || Boolean(affordances?.defendCardIds?.length)) {
      actions.push({
        key: "defend",
        label: tr("Защита", "Захист"),
        variant: canDefend ? "primary" : "default",
        disabled:
          !canDefend ||
          interactionLocked ||
          !isCardAllowedForAction("defend", selectedCardId ?? undefined, hasDetailedHand, affordances),
        onClick: () => sendAction("defend"),
      });
    }
    if (canTake || isDefenderRole) {
      actions.push({
        key: "take",
        label: tr("Беру", "Беру"),
        variant: "primary",
        disabled: !canTake || interactionLocked,
        onClick: () => sendAction("take"),
      });
    }
    return actions;
  }, [
    affordances,
    canAttack,
    canDefend,
    canTake,
    hasDetailedHand,
    interactionLocked,
    isAttackerRole,
    isDefenderRole,
    selectedCardId,
    tr,
  ]);

  const secondaryActions = useMemo<ActionDescriptor[]>(() => {
    const actions: ActionDescriptor[] = [];
    if (canPass) {
      actions.push({
        key: "pass",
        label: tr("Бито", "Бито"),
        disabled: interactionLocked,
        onClick: () => sendAction("pass"),
      });
    }
    if (isTranslateMode) {
      actions.push({
        key: "translate",
        label: tr("Перевести", "Перевести"),
        disabled:
          !canTranslate ||
          interactionLocked ||
          !isCardAllowedForAction("translate", selectedCardId ?? undefined, hasDetailedHand, affordances),
        onClick: () => sendAction("translate"),
      });
    }
    if (canThrow || Boolean(affordances?.throwInCardIds?.length)) {
      actions.push({
        key: "throw",
        label: tr("Подкинуть", "Підкинути"),
        disabled:
          !canThrow ||
          interactionLocked ||
          !isCardAllowedForAction("throw", selectedCardId ?? undefined, hasDetailedHand, affordances),
        onClick: () => sendAction("throw"),
      });
    }
    if (canShulerPlay || Boolean(affordances?.shulerPlayCardIds?.length)) {
      actions.push({
        key: "shuler_play",
        label: tr("Шулер-ход", "Шулер-хід"),
        disabled:
          !canShulerPlay ||
          interactionLocked ||
          !isCardAllowedForAction("shuler_play", selectedCardId ?? undefined, hasDetailedHand, affordances),
        onClick: () => sendAction("shuler_play"),
      });
    }
    if (canReportShuler) {
      actions.push({
        key: "shuler_report",
        label:
          shulerWindowSecondsLeft != null
            ? tr(`Жалоба на шулера (${shulerWindowSecondsLeft}с)`, `Скарга на шулера (${shulerWindowSecondsLeft}с)`)
            : tr("Жалоба на шулера", "Скарга на шулера"),
        disabled: interactionLocked,
        onClick: () => sendAction("shuler_report"),
      });
    }
    return actions;
  }, [
    affordances,
    canPass,
    canReportShuler,
    canShulerPlay,
    canThrow,
    canTranslate,
    hasDetailedHand,
    interactionLocked,
    isTranslateMode,
    selectedCardId,
    shulerWindowSecondsLeft,
    tr,
  ]);

  function sendAction(action: MatchActionType, cardIdOverride?: string) {
    if (!id || interactionLocked) return;
    if (action === "shuler_report" && !canReportShuler) {
      setIntentError({
        text: tr("Окно жалобы на шулера уже закрыто.", "Вікно скарги на шулера вже закрите."),
        code: "shulerReportWindowClosed",
      });
      hapticNotification("warning");
      return;
    }
    const cardId = cardIdOverride ?? selectedCardId ?? undefined;
    if (!isCardAllowedForAction(action, cardId, hasDetailedHand, affordances)) {
      hapticNotification("warning");
      return;
    }
    if (isMockApiEnabled()) {
      const mockAction =
        action === "translate"
          ? "defend"
          : action === "shuler_play"
            ? "defend"
            : action === "shuler_report"
              ? "pass"
              : action === "throw"
                ? "attack"
                : action;
      const next = applyMockMatchAction({ roomId: id, action: mockAction, cardId });
      setMatchState(next);
      addActivity(`Mock action: ${action}`);
      if (["attack", "defend", "throw", "translate", "shuler_play"].includes(action)) {
        setSelectedCardId(null);
      }
      return;
    }
    const intentByAction: Record<MatchActionType, IntentType> = {
      attack: "playAttack",
      defend: "playDefend",
      throw: "throwIn",
      shuler_play: "shulerPlay",
      take: "take",
      pass: "pass",
      translate: "translate",
      shuler_report: "shulerReport",
    };
    const intent = intentByAction[action];
    const intentPayload: Record<string, unknown> = { roomId: id };
    if (cardId && ["attack", "defend", "throw", "translate", "shuler_play"].includes(action)) {
      intentPayload.cardId = cardId;
    }
    hapticImpact("medium");
    wsClient.sendIntent(intent, intentPayload);
    if (["attack", "defend", "throw", "translate", "shuler_play"].includes(action)) {
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
              aria-label={tr("Закрыть", "Закрити")}
            >
              <CloseIcon size={16} />
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

      <header className="game-table-screen__header">
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
        <div className="game-table-screen__heading">
          <span className="game-table-screen__eyebrow">{roomModeLabel}</span>
          <h1 className="game-table-screen__title">{tr("Игровой стол", "Ігровий стіл")}</h1>
        </div>
        <div className="game-table-screen__timer-pill">{secondsLeft !== null ? `${secondsLeft}с` : "—"}</div>
      </header>

      {isRoomLoading && <CardSkeleton rows={4} />}
      {gameState.status === "connecting" && room && (
        <AppCard>
          <div className="card__hint">{tr("Подключение к матчу…", "Підключення до матчу…")}</div>
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
            "Стол недоступен. Вернитесь к списку игр и выберите другую комнату.",
            "Стіл недоступний. Поверніться до списку ігор та виберіть іншу кімнату.",
          )}
          actionLabel={tr("К списку игр", "До списку ігор")}
          onAction={() => navigate("/play")}
        />
      )}
      {room && (
        <>
          <AppCard className="game-surface">
            <div className="game-surface__hero">
              <div className="game-surface__status-copy">
                <span className="game-surface__status-eyebrow">{tr("Партия", "Партія")}</span>
                <strong>{turnLabel}</strong>
              </div>
              <div className="game-surface__balance-pill">
                <span>{tr("Баланс", "Баланс")}</span>
                <strong>{balanceLabel}</strong>
              </div>
            </div>

            <div className="game-surface__chip-row">
              <span className="game-surface__chip game-surface__chip--accent">${room.stakeUsd.toFixed(2)}</span>
              <span className="game-surface__chip">{roomModeLabel}</span>
              <span className="game-surface__chip">{fairnessLabel}</span>
              <span className="game-surface__chip">{deckLabel}</span>
              <span className="game-surface__chip">{playersLabel}</span>
              <span className="game-surface__chip">{trumpLabel}</span>
              <span className="game-surface__chip">{stockLabel}</span>
            </div>

            <div className={`game-seats game-seats--count-${Math.min(Math.max(opponentSeats.length, 1), 4)}`}>
              {opponentSeats.length > 0 ? (
                opponentSeats.map((seat) => (
                  <div className="game-seat" key={seat.id}>
                    <div className="game-seat__header">
                      <AppAvatar name={seat.name} photoUrl={seat.avatarUrl} className="game-seat__avatar" />
                      <div className="game-seat__identity">
                        <span className="game-seat__name">{seat.name}</span>
                        <div className="game-seat__badges">
                          {seat.roleLabel && <span className="game-seat__badge">{seat.roleLabel}</span>}
                          {seat.isShuler && <span className="game-seat__badge game-seat__badge--danger">{tr("Шулер", "Шулер")}</span>}
                        </div>
                      </div>
                    </div>
                    <div className="game-seat__hand">
                      <div className="game-seat__hand-stack" aria-label={tr("Карты соперника", "Карти суперника")}>
                        {Array.from({ length: Math.max(1, Math.min(3, seat.cardCount || 1)) }).map((_, index) => (
                          <div className={`game-seat__hand-card game-seat__hand-card--${index}`} key={`${seat.id}-${index}`}>
                            <PlayingCard faceUp={false} variant="mini" />
                          </div>
                        ))}
                      </div>
                      <span className="game-seat__count">{seat.cardCount}</span>
                    </div>
                  </div>
                ))
              ) : (
                <div className="game-seat game-seat--empty">{tr("Ожидаем соперников", "Очікуємо суперників")}</div>
              )}
            </div>

            <div ref={tableZoneRef} className="game-surface__felt" {...(canTake && !interactionLocked ? swipeTake : {})}>
              <div className="game-surface__felt-glow" />
              {tablePairs.length > 0 ? (
                <div className="table-pairs">
                  {tablePairs.map((pair, index) => {
                    const defenseCard = pair.defense ?? (pair as any).defend;
                    return (
                      <div className="table-pairs__item" key={`pair-${index}`}>
                        <div className="table-pairs__card table-pairs__card--attack">
                          <PlayingCard suit={pair.attack.suit} rank={pair.attack.rank} variant="table" />
                        </div>
                        <div className={`table-pairs__card ${defenseCard ? "table-pairs__card--defense" : "table-pairs__card--pending"}`}>
                          {defenseCard ? (
                            <PlayingCard suit={defenseCard.suit} rank={defenseCard.rank} variant="table" />
                          ) : (
                            <PlayingCard placeholder variant="table" />
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="game-surface__empty">{tableEmptyLabel}</div>
              )}
              <div className="game-surface__center-pill">{turnLabel}</div>
            </div>

            <div className="game-surface__footer">
              {gameState.reconnectingPlayerId && gameState.reconnectingPlayerId !== currentUserId ? (
                <span className="game-surface__footer-note game-surface__footer-note--warning">
                  {tr("Соперник отключился. Ждем возвращения 60 секунд.", "Суперник відключився. Очікуємо повернення 60 секунд.")}
                </span>
              ) : (
                <span className="game-surface__footer-note">{handHint}</span>
              )}
              {gameState.error && <span className="game-surface__footer-note game-surface__footer-note--error">{gameState.error}</span>}
            </div>
          </AppCard>

          <AppCard className="game-hand-panel">
            <div className="game-hand-panel__hero">
              <div className="game-hand-panel__identity">
                <AppAvatar
                  name={meSeat?.name ?? currentPlayer?.displayName ?? currentPlayer?.username ?? tr("Вы", "Ви")}
                  photoUrl={meSeat?.avatarUrl ?? currentPlayer?.photoUrl}
                  className="game-hand-panel__avatar"
                />
                <div className="game-hand-panel__copy">
                  <span className="game-hand-panel__name">{tr("Вы", "Ви")}</span>
                  <div className="game-hand-panel__badges">
                    {isAttackerRole && <span className="game-seat__badge">{tr("Атакуете", "Атакуєте")}</span>}
                    {isDefenderRole && !isAttackerRole && <span className="game-seat__badge">{tr("Защищаетесь", "Захищаєтесь")}</span>}
                    {currentUserId && shulerPlayers.has(currentUserId) && <span className="game-seat__badge game-seat__badge--danger">{tr("Шулер", "Шулер")}</span>}
                  </div>
                </div>
              </div>
              <div className="game-hand-panel__meta">
                <div className="game-hand-panel__balance-chip">
                  <span>{tr("Баланс", "Баланс")}</span>
                  <strong>{balanceLabel}</strong>
                </div>
                <div className="game-hand-panel__count">{meSeat?.cardCount ?? currentPlayer?.handCount ?? 0}</div>
              </div>
            </div>

            <div className="game-hand-panel__label-row">
              <span className="card__label">{tr("Ваши карты", "Ваші карти")}</span>
              {selectedCard && <span className="game-hand-panel__selection">{selectedCard.rank}{getSuitSymbol(selectedCard.suit)}</span>}
            </div>

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
                    sendAction(action === "attack" ? "attack" : action === "defend" ? "defend" : action === "throw" ? "throw" : "shuler_play", cardId);
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
                    : Array.from({ length: currentPlayer.handCount }).map((_, index) => <PlayingCard key={`back-${index}`} faceUp={false} variant="hand" />)}
                </div>
              )
            ) : (
              <div className="card__hint">{tr("Карт на руке нет", "Карт на руці немає")}</div>
            )}
          </AppCard>

          <AppCard className="game-command-bar">
            {primaryActions.length > 0 && (
              <div className={`game-command-bar__primary game-command-bar__primary--${Math.min(primaryActions.length, 3)}`}>
                {primaryActions.map((action) => (
                  <AppButton
                    key={action.key}
                    type="button"
                    variant={action.variant ?? "default"}
                    className="game-command-bar__button"
                    onClick={action.onClick}
                    disabled={action.disabled}
                  >
                    {action.label}
                  </AppButton>
                ))}
              </div>
            )}
            {secondaryActions.length > 0 && (
              <div className="game-command-bar__secondary">
                {secondaryActions.map((action) => (
                  <AppButton
                    key={action.key}
                    type="button"
                    variant={action.variant ?? "secondary"}
                    className="game-command-bar__secondary-button"
                    onClick={action.onClick}
                    disabled={action.disabled}
                  >
                    {action.label}
                  </AppButton>
                ))}
              </div>
            )}
            <div className="game-command-bar__footer">
              <Link className="button button--secondary game-command-bar__invite" to={`/game/${id}/friends`}>
                {tr("Пригласить друга", "Запросити друга")}
              </Link>
              <AppButton type="button" variant="ghost" className="game-command-bar__leave" onClick={() => setIsExitModalOpen(true)}>
                {tr("Выйти", "Вийти")}
              </AppButton>
            </div>
          </AppCard>
          {finish && (
            <div className={`result-card ${isWinner ? "result-card--win" : "result-card--lose"}`}>
              <div className="result-card__title">{isWinner ? tr("Победа!", "Перемога!") : tr("Матч завершен", "Матч завершено")}</div>
              <div className="result-card__message">
                {isWinner && gameState.matchFinishedAbandoned
                  ? tr("Соперник не вернулся. Победа засчитана.", "Суперник не повернувся. Перемогу зараховано.")
                  : winnerName
                    ? tr(`Победитель: ${winnerName}`, `Переможець: ${winnerName}`)
                    : tr("Ожидаем итоговый расчет.", "Очікуємо фінальний розрахунок.")}
              </div>
              <div className={`result-card__amount ${isWinner ? "result-card__amount--plus" : "result-card__amount--minus"}`}>
                {myPayout >= 0 ? "+" : ""}
                {myPayout.toFixed(2)} USD
              </div>
              <div className="result-card__summary">
                <div>{tr("Банк", "Банк")}: {finish.bank.toFixed(2)} USD</div>
                <div>{tr("Комиссия", "Комісія")}: {finish.commission.toFixed(2)} USD</div>
                <div>{tr("Чистый результат", "Чистий результат")}: {myPayout.toFixed(2)} USD</div>
              </div>
              <div className="result-card__places">
                <div className="result-card__places-title">{tr("Результаты игроков", "Результати гравців")}</div>
                <div className="result-card__places-list">
                  {finish.places.map((playerId, index) => {
                    const seat = matchState?.seats?.find((s) => s.id === playerId);
                    const fromPlayers = players.find((player: any) => player.id === playerId);
                    const name = seat?.name ?? fromPlayers?.displayName ?? fromPlayers?.username ?? playerId;
                    const payout = finish.payouts[playerId] ?? 0;
                    const isMeRow = currentUserId === playerId;
                    return (
                      <div key={playerId} className={`result-card__place-row ${isMeRow ? "result-card__place-row--me" : ""}`}>
                        <div className="result-card__place-left">
                          <span className="result-card__place-number">{index + 1}</span>
                          <span className="result-card__place-name">
                            {name}
                            {isMeRow ? tr(" (Вы)", " (Ви)") : ""}
                          </span>
                        </div>
                        <div className="result-card__place-right">{payout === 0 ? "—" : `${payout.toFixed(2)} USD`}</div>
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
        message={tr(
          "Вы покинете активный матч и вернетесь к списку игр.",
          "Ви залишите активний матч і повернетеся до списку ігор.",
        )}
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

function isCardAllowedForAction(
  action: MatchActionType,
  cardId: string | undefined,
  hasDetailedHand: boolean,
  affordances?: MatchAffordances,
) {
  if (!hasDetailedHand) return true;
  if (!cardId) return false;
  if (!affordances) return true;
  const allowedCardIdsByAction: Partial<Record<MatchActionType, string[] | undefined>> = {
    attack: affordances.attackCardIds,
    defend: affordances.defendCardIds,
    throw: affordances.throwInCardIds,
    translate: affordances.translateCardIds,
    shuler_play: affordances.shulerPlayCardIds,
  };
  const allowedCardIds = allowedCardIdsByAction[action];
  return !Array.isArray(allowedCardIds) ? true : allowedCardIds.includes(cardId);
}

function getSuitSymbol(suit?: string) {
  const symbols: Record<string, string> = { hearts: "♥", diamonds: "♦", clubs: "♣", spades: "♠" };
  return suit ? symbols[suit] ?? "?" : "?";
}

function formatSuitLabel(suit: string | undefined, tr: (ru: string, uk: string) => string) {
  if (!suit) return tr("Козырь —", "Козир —");
  const names: Record<string, string> = {
    hearts: tr("Черви", "Чирва"),
    diamonds: tr("Бубны", "Бубна"),
    clubs: tr("Трефы", "Трефа"),
    spades: tr("Пики", "Піка"),
  };
  return `${getSuitSymbol(suit)} ${names[suit] ?? suit}`;
}

function formatModeLabel(rawMode: string | undefined, tr: (ru: string, uk: string) => string) {
  const normalized = `${rawMode ?? ""}`.toLowerCase();
  if (normalized.includes("podkid") || normalized.includes("подкид")) return tr("Подкидной", "Підкидний");
  if (normalized.includes("perevod") || normalized.includes("перевод")) return tr("Переводной", "Перевідний");
  if (normalized.includes("fair") || normalized.includes("чест")) return tr("Честная игра", "Чесна гра");
  if (normalized.includes("shuler") || normalized.includes("шулер")) return tr("Шулер", "Шулер");
  if (normalized.includes("classic") || normalized.includes("класс")) return tr("Классика", "Класика");
  return rawMode || tr("Матч", "Матч");
}

function formatFairnessLabel(rawMode: string | undefined, tr: (ru: string, uk: string) => string) {
  const normalized = `${rawMode ?? ""}`.toLowerCase();
  if (normalized.includes("shuler") || normalized.includes("шулер")) return tr("Шулер", "Шулер");
  return tr("Честная игра", "Чесна гра");
}

function formatDeckLabel(deck: number | undefined, tr: (ru: string, uk: string) => string) {
  if (!deck) return tr("Колода —", "Колода —");
  return tr(`${deck} карт`, `${deck} карт`);
}

function formatPlayersLabel(players: number, maxPlayers: number, tr: (ru: string, uk: string) => string) {
  const total = maxPlayers || players;
  return tr(`${players}/${total} игрока`, `${players}/${total} гравця`);
}

function getRoleLabel(
  playerId: string,
  attackerId: string | null,
  defenderId: string | null,
  tr: (ru: string, uk: string) => string,
) {
  if (playerId === attackerId) return tr("Атакует", "Атакує");
  if (playerId === defenderId) return tr("Защита", "Захист");
  return null;
}

function fallbackPlayerName(index: number, tr: (ru: string, uk: string) => string) {
  return tr(`Игрок ${index}`, `Гравець ${index}`);
}

function buildTablePairs(cards: Card[]) {
  const pairs: TablePair[] = [];
  for (let index = 0; index < cards.length; index += 2) {
    pairs.push({ attack: cards[index], defense: cards[index + 1] });
  }
  return pairs;
}
