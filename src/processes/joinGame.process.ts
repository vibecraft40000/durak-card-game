import { initializeMockMatch, isMockApiEnabled } from "@/mocks/mockApi";
import { httpRequest } from "@/shared/api/http";
import { onWsEvent } from "@/shared/api/ws/events";
import { wsClient } from "@/shared/api/ws/socket";
import {
  addActivity,
  clearGameError,
  setGameConnecting,
  setGameError,
  setGameReady,
  setMatchState,
  setMatchFinishedAbandoned,
  setReconnectingPlayer,
} from "@/store/game.store";

export async function joinGameRoom(roomId: string) {
  setGameConnecting(roomId);

  try {
    await httpRequest<{ ok: boolean }>(`/api/rooms/${roomId}/join`, {
      method: "POST",
    });
  } catch {
    setGameError("Не удалось войти в комнату");
    return () => undefined;
  }

  if (isMockApiEnabled()) {
    setGameReady();
    addActivity(`Mock: вход в комнату ${roomId}`);
    setMatchState(initializeMockMatch(roomId));
    return () => undefined;
  }

  wsClient.connect(roomId);

  const offRoom = onWsEvent("room_update", () => {
    setGameReady();
    addActivity(`Подключение к комнате ${roomId}`);
  });

  const offState = onWsEvent("game_state", ({ payload }) => {
    if (payload.roomId !== roomId) {
      return;
    }
    clearGameError();
    setMatchState(payload);
  });

  const offMove = onWsEvent("move_applied", ({ payload }) => {
    if (payload?.roomId === roomId && payload?.playerId) {
      addActivity(`Ход: ${payload.action}${payload.cardId ? ` (карта)` : ""}`);
    }
  });

  const offTimer = onWsEvent("timer_update", ({ payload }) => {
    addActivity(`Таймер обновлен: ${payload.turnPlayerId}`);
  });

  const offFinished = onWsEvent("match_finished", ({ payload }) => {
    setMatchFinishedAbandoned(Boolean(payload.abandoned));
    addActivity(
      payload.abandoned
        ? "Соперник не вернулся. Победа."
        : `Матч завершен. Победитель: ${payload.winnerPlayerId}`,
    );
  });

  const offError = onWsEvent("error", ({ payload }) => {
    setGameError(payload.message);
  });

  const offDisconnected = onWsEvent("player_disconnected", ({ payload }) => {
    if (payload?.roomId === roomId && payload?.playerId) {
      setReconnectingPlayer(payload.playerId);
      addActivity("Соперник отключился. Ожидание 60 сек...");
    }
  });

  const offReconnected = onWsEvent("player_reconnected", ({ payload }) => {
    if (payload?.roomId === roomId && payload?.playerId) {
      setReconnectingPlayer(null);
      addActivity("Соперник переподключился.");
    }
  });

  return () => {
    offRoom();
    offState();
    offMove();
    offTimer();
    offFinished();
    offError();
    offDisconnected();
    offReconnected();
    setReconnectingPlayer(null);
    setMatchFinishedAbandoned(false);
    wsClient.disconnect();
  };
}
