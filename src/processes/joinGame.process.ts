import { initializeMockMatch, isMockApiEnabled } from "@/mocks/mockApi";
import { httpRequest } from "@/shared/api/http";
import { onWsEvent } from "@/shared/api/ws/events";
import { setConnectionHandler, wsClient } from "@/shared/api/ws/socket";
import { trRuntime } from "@/shared/i18n/runtime";
import {
  addActivity,
  addActivityItem,
  clearGameError,
  getGameState,
  setGameConnecting,
  setGameError,
  setGameReady,
  setInteractionLocked,
  setMatchState,
  setMatchStateIfNewer,
  setMatchFinishedAbandoned,
  setMatchResult,
  setIsFinishedHandled,
  setReconnectingPlayer,
} from "@/store/game.store";
import { setSocketReconnecting } from "@/store/socket.store";

export async function joinGameRoom(roomId: string) {
  setGameConnecting(roomId);

  try {
    await httpRequest<{ ok: boolean }>(`/api/rooms/${roomId}/join`, {
      method: "POST",
    });
  } catch {
    setGameError(trRuntime("Не удалось войти в комнату", "Не вдалося увійти в кімнату"));
    return () => undefined;
  }

  if (isMockApiEnabled()) {
    setGameReady();
    addActivity(trRuntime(`Mock: вход в комнату ${roomId}`, `Mock: вхід у кімнату ${roomId}`));
    setMatchState(initializeMockMatch(roomId));
    return () => undefined;
  }

  wsClient.connect(roomId);
  setConnectionHandler((event) => {
    setSocketReconnecting(event === "disconnect");
  }); // connect -> false, disconnect -> true

  const offRoom = onWsEvent("room_update", () => {
    setGameReady();
    addActivity(trRuntime(`Подключение к комнате ${roomId}`, `Підключення до кімнати ${roomId}`));
  });

  let lastGameStateAt = Date.now();
  const offState = onWsEvent("game_state", ({ payload }) => {
    if (payload.roomId !== roomId) {
      return;
    }
    clearGameError();
    lastGameStateAt = Date.now();
    setMatchStateIfNewer(payload);
  });

  const SYNC_FALLBACK_MS = 5000;
  const syncIntervalId = window.setInterval(() => {
    const { matchState } = getGameState();
    if (
      matchState?.status === "playing" &&
      Date.now() - lastGameStateAt > SYNC_FALLBACK_MS
    ) {
      wsClient.send({ type: "sync_request", payload: { roomId } });
    }
  }, 2000);

  const offMove = onWsEvent("move_applied", ({ payload }) => {
    if (payload?.roomId !== roomId || !payload?.playerId) return;
    const eventId = payload.eventId ?? `${payload.matchId}-${Date.now()}`;
    const { activity } = getGameState();
    const alreadyExists = activity.some(
      (a) => a.type === "move" && a.eventId === eventId,
    );
    if (alreadyExists) return;
    addActivityItem({
      eventId,
      playerId: payload.playerId,
      action: payload.action,
      cardId: payload.cardId,
      timestamp: Date.now(),
    });
  });

  const offTimer = onWsEvent("timer_update", ({ payload }) => {
    addActivity(trRuntime(`Таймер обновлен: ${payload.turnPlayerId}`, `Таймер оновлено: ${payload.turnPlayerId}`));
  });

  const offFinished = onWsEvent("match_finished", ({ payload }) => {
    const { isFinishedHandled } = getGameState();
    if (isFinishedHandled) return;
    setIsFinishedHandled(true);

    setMatchFinishedAbandoned(Boolean(payload.abandoned));
    addActivity(
      payload.abandoned
        ? trRuntime("Соперник не вернулся. Победа.", "Суперник не повернувся. Перемога.")
        : trRuntime(
            `Матч завершен. Победитель: ${payload.winnerPlayerId}`,
            `Матч завершено. Переможець: ${payload.winnerPlayerId}`,
          ),
    );

    if (payload.payouts && payload.payouts.length > 0) {
      setMatchResult({
        settlementId: payload.settlementId,
        payouts: payload.payouts,
        commission: payload.commission,
        pot: payload.pot,
        newBalances: payload.newBalances,
        abandoned: payload.abandoned,
      });
    } else {
      setMatchResult({ payouts: [], abandoned: payload.abandoned });
    }
  });

  const offError = onWsEvent("error", ({ payload }) => {
    if (payload?.errorCode === "VERSION_MISMATCH") {
      return; // Handled by version_mismatch, no error UI
    }
    setGameError(payload?.message ?? trRuntime("Ошибка", "Помилка"));
  });

  const offVersionMismatch = onWsEvent("version_mismatch", ({ payload }) => {
    if (payload?.roomId !== roomId) return;
    setInteractionLocked(true);
    clearGameError();
    const { matchState } = getGameState();
    const action = payload.action;
    const cardId = payload.cardId;
    const actionId = payload.actionId ?? crypto.randomUUID();
    const expectedVersion =
      matchState?.version != null ? { expectedVersion: matchState.version } : {};
    wsClient.send({
      type: "make_move",
      payload: {
        roomId,
        action: (action ?? "pass_turn") as
          | "attack_card"
          | "defend_card"
          | "translate"
          | "take_cards"
          | "pass_turn"
          | "end_round"
          | "shuler_report",
        ...(cardId && { cardId }),
        ...expectedVersion,
        actionId,
      },
    });
  });

  const offDisconnected = onWsEvent("player_disconnected", ({ payload }) => {
    if (payload?.roomId === roomId && payload?.playerId) {
      setReconnectingPlayer(payload.playerId);
      addActivity(trRuntime("Соперник отключился. Ожидание 60 сек...", "Суперник відключився. Очікування 60 сек..."));
    }
  });

  const offReconnected = onWsEvent("player_reconnected", ({ payload }) => {
    if (payload?.roomId === roomId && payload?.playerId) {
      setReconnectingPlayer(null);
      addActivity(trRuntime("Соперник переподключился.", "Суперник перепідключився."));
    }
  });

  return () => {
    window.clearInterval(syncIntervalId);
    setConnectionHandler(null);
    setSocketReconnecting(false);
    offRoom();
    offState();
    offMove();
    offTimer();
    offFinished();
    offError();
    offVersionMismatch();
    offDisconnected();
    offReconnected();
    setReconnectingPlayer(null);
    setMatchFinishedAbandoned(false);
    wsClient.disconnect();
  };
}
