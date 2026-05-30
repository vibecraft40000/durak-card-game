import { initializeMockMatch, isMockApiEnabled } from "@/mocks/mockApi";
import { createJoinGameMoveReplayBuffer } from "@/processes/joinGameMoveReplayBuffer";
import { ReplayDebugTracker } from "@/processes/replayDebug";
import { httpRequest } from "@/shared/api/http";
import { onWsEvent } from "@/shared/api/ws/events";
import { setConnectionHandler, wsClient } from "@/shared/api/ws/socket";
import { trRuntime } from "@/shared/i18n/runtime";
import { mapServerErrorToMessage } from "@/shared/utils/mapServerErrorToMessage";
import {
  addActivity,
  addActivityItem,
  clearGameError,
  getGameState,
  pruneStaleMoveActivity,
  resetActivityForMatchFinished,
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
const RESET_ACTIVITY_ON_MATCH_FINISH =
  String(import.meta.env.VITE_MATCH_ACTIVITY_RESET_ON_FINISH ?? "1") !== "0";

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

  const replayDebug = new ReplayDebugTracker(roomId);

  const hasMoveEventIdInActivity = (eventId: string): boolean =>
    getGameState().activity.some(
      (item) => item.type === "move" && item.eventId === eventId,
    );

  const moveReplayBuffer = createJoinGameMoveReplayBuffer({
    getCurrentVersion: () => getGameState().matchState?.version,
    hasMoveEventIdInActivity,
    addBufferedMove: (move) => {
      addActivityItem({
        eventId: move.eventId,
        playerId: move.playerId,
        action: move.action,
        cardId: move.cardId,
        timestamp: move.receivedAt,
      });
    },
    replayDebug,
  });

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
    pruneStaleMoveActivity(payload.version);
    replayDebug.setStateVersion(payload.version);
    moveReplayBuffer.flush();
  });

  const offStateSync = onWsEvent("state_sync", ({ payload }) => {
    if (payload?.roomId !== roomId) {
      return;
    }
    replayDebug.markSync(payload.mode);
  });

  const offStateDiff = onWsEvent("state_diff", ({ payload }) => {
    if (payload?.roomId !== roomId) {
      return;
    }
    const current = getGameState().matchState;
    if (!current) {
      return;
    }
    const nextVersion =
      typeof payload.toVersion === "number"
        ? payload.toVersion
        : typeof payload.patch?.version === "number"
          ? payload.patch.version
          : current.version;
    if (
      typeof current.version === "number" &&
      typeof nextVersion === "number" &&
      nextVersion <= current.version
    ) {
      return;
    }
    const merged = {
      ...current,
      ...payload.patch,
      version: nextVersion,
      roomId: payload.roomId,
    };
    setMatchStateIfNewer(merged);
    if (typeof nextVersion === "number") {
      pruneStaleMoveActivity(nextVersion);
      replayDebug.setStateVersion(nextVersion);
    }
    moveReplayBuffer.flush();
  });

  const SYNC_FALLBACK_MS = 5000;
  const syncIntervalId = window.setInterval(() => {
    const { matchState } = getGameState();
    if (
      matchState?.status === "playing" &&
      Date.now() - lastGameStateAt > SYNC_FALLBACK_MS
    ) {
      replayDebug.incSyncRequest();
      wsClient.send({
        type: "sync_request",
        payload: {
          roomId,
          ...(typeof matchState.version === "number"
            ? { lastKnownVersion: matchState.version }
            : {}),
          ...(typeof matchState.matchId === "string" && matchState.matchId
            ? { lastKnownMatchId: matchState.matchId }
            : {}),
          supportsStateDiff: true,
        },
      });
    }
  }, 2000);

  const offMove = onWsEvent("move_applied", ({ payload }) => {
    if (payload?.roomId !== roomId || !payload?.playerId) return;
    moveReplayBuffer.handleMoveApplied(payload);
  });

  const offTimer = onWsEvent("timer_update", ({ payload }) => {
    addActivity(trRuntime(`Таймер обновлен: ${payload.turnPlayerId}`, `Таймер оновлено: ${payload.turnPlayerId}`));
  });

  const offFinished = onWsEvent("match_finished", ({ payload }) => {
    if (payload?.roomId !== roomId) return;
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
        netResults: payload.payouts,
        commission: payload.commission,
        pot: payload.pot,
        newBalances: payload.newBalances,
        abandoned: payload.abandoned,
      });
    } else {
      setMatchResult({ netResults: [], abandoned: payload.abandoned });
    }
    if (RESET_ACTIVITY_ON_MATCH_FINISH) {
      resetActivityForMatchFinished();
    }
  });

  const offError = onWsEvent("error", ({ payload }) => {
    if (payload?.errorCode === "VERSION_MISMATCH") {
      return; // Handled by version_mismatch, no error UI
    }
    const mapped = mapServerErrorToMessage({
      code: payload?.errorCode,
      message: payload?.message,
    });
    setGameError(mapped.text);

    if (
      payload?.errorCode === "INVALID_ACTION" ||
      payload?.errorCode === "INVALID_CARD" ||
      payload?.errorCode === "INVALID_TURN"
    ) {
      window.dispatchEvent(
        new CustomEvent("tma:intentError", {
          detail: {
            raw: payload,
            text: mapped.text,
            code: mapped.code ?? payload.errorCode,
          },
        }),
      );
    }
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
          | "throw_card"
          | "translate"
          | "take_cards"
          | "pass_turn"
          | "end_round"
          | "shuler_play"
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

  const offBotTakeover = onWsEvent("player_afk_bot_takeover", ({ payload }) => {
    if (payload?.roomId !== roomId || !payload?.playerId) return;
    setReconnectingPlayer(null);
    addActivity(
      trRuntime(
        "Игрок долго офлайн. Для него включены автоходы.",
        "Гравець довго офлайн. Для нього ввімкнено автоходи.",
      ),
    );
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
    offStateSync();
    offStateDiff();
    offMove();
    offTimer();
    offFinished();
    offError();
    offVersionMismatch();
    offDisconnected();
    offBotTakeover();
    offReconnected();
    setReconnectingPlayer(null);
    setMatchFinishedAbandoned(false);
    replayDebug.dispose();
    wsClient.disconnect();
  };
}
