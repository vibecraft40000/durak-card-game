import type { MatchStatePayload } from "@/shared/api/ws/types";

export type GameState = {
  roomId: string | null;
  status: "idle" | "connecting" | "ready" | "error";
  matchState: MatchStatePayload | null;
  error: string | null;
  activity: string[];
  /** Player ID currently in reconnect grace period (shows "Соперник отключился. Ожидание 60 сек...") */
  reconnectingPlayerId: string | null;
  /** True when match finished due to opponent abandon (disconnect timeout) */
  matchFinishedAbandoned: boolean;
};

type Listener = (state: GameState) => void;

const listeners = new Set<Listener>();

let state: GameState = {
  roomId: null,
  status: "idle",
  matchState: null,
  error: null,
  activity: [],
  reconnectingPlayerId: null,
  matchFinishedAbandoned: false,
};

function update(partial: Partial<GameState>) {
  state = { ...state, ...partial };
  listeners.forEach((listener) => listener(state));
}

export function setGameConnecting(roomId: string) {
  update({ roomId, status: "connecting", error: null });
}

export function setGameReady() {
  update({ status: "ready", error: null });
}

export function setGameError(message: string) {
  update({ status: "error", error: message, activity: [...state.activity, `Ошибка: ${message}`] });
}

export function clearGameError() {
  if (state.error) update({ error: null });
}

export function setMatchState(matchState: MatchStatePayload) {
  update({ matchState });
}

export function setReconnectingPlayer(playerId: string | null) {
  update({ reconnectingPlayerId: playerId });
}

export function setMatchFinishedAbandoned(abandoned: boolean) {
  update({ matchFinishedAbandoned: abandoned });
}

export function addActivity(message: string) {
  update({ activity: [...state.activity, message] });
}

export function getGameState() {
  return state;
}

export function subscribeGameStore(listener: Listener) {
  listeners.add(listener);
  listener(state);
  return () => {
    listeners.delete(listener);
  };
}
