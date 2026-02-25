import type { MatchStatePayload } from "@/shared/api/ws/types";

export type ActivityMoveItem = {
  type: "move";
  eventId: string;
  playerId: string;
  action: string;
  cardId?: string;
  timestamp: number;
};

export type ActivitySystemItem = {
  type: "system";
  message: string;
  timestamp: number;
};

export type ActivityItem = ActivityMoveItem | ActivitySystemItem;

export type MatchResult = {
  settlementId?: string;
  payouts: { userId: string; amount: number }[];
  commission?: number;
  pot?: number;
  newBalances?: Record<string, number>;
  abandoned?: boolean;
};

export type GameState = {
  roomId: string | null;
  status: "idle" | "connecting" | "ready" | "error";
  matchState: MatchStatePayload | null;
  error: string | null;
  activity: ActivityItem[];
  /** Player ID currently in reconnect grace period */
  reconnectingPlayerId: string | null;
  /** True when match finished due to opponent abandon */
  matchFinishedAbandoned: boolean;
  /** Payout info from match_finished (server is source of truth) */
  matchResult: MatchResult | null;
  /** Guard: avoid processing match_finished twice */
  isFinishedHandled: boolean;
  /** Display balance (from profile or newBalances; safeguard: do not null if newBalance missing) */
  displayBalance: number | null;
  /** Block drag/buttons during version_mismatch reconcile */
  interactionLocked: boolean;
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
  matchResult: null,
  isFinishedHandled: false,
  displayBalance: null,
  interactionLocked: false,
};

function update(partial: Partial<GameState>) {
  state = { ...state, ...partial };
  listeners.forEach((listener) => listener(state));
}

export function setGameConnecting(roomId: string) {
  update({
    roomId,
    status: "connecting",
    error: null,
    matchResult: null,
    isFinishedHandled: false,
    activity: [],
    interactionLocked: false,
  });
}

export function setInteractionLocked(locked: boolean) {
  update({ interactionLocked: locked });
}

export function setGameReady() {
  update({ status: "ready", error: null });
}

export function setGameError(message: string) {
  const item: ActivitySystemItem = {
    type: "system",
    message: `Ошибка: ${message}`,
    timestamp: Date.now(),
  };
  update({
    status: "error",
    error: message,
    activity: [...state.activity, item].slice(-ACTIVITY_LIMIT),
  });
}

export function clearGameError() {
  if (state.error) update({ error: null });
}

export function setMatchState(matchState: MatchStatePayload) {
  update({ matchState });
}

/** Only update if payload has same or newer version (discard stale out-of-order delivery) */
export function setMatchStateIfNewer(payload: MatchStatePayload) {
  const current = state.matchState;
  const incomingVersion = payload.version;
  const currentVersion = current?.version;
  if (typeof currentVersion === "number" && incomingVersion <= currentVersion) {
    return; // ignore stale or duplicate state
  }
  update({ matchState: payload, interactionLocked: false });
}

export function setReconnectingPlayer(playerId: string | null) {
  update({ reconnectingPlayerId: playerId });
}

export function setMatchFinishedAbandoned(abandoned: boolean) {
  update({ matchFinishedAbandoned: abandoned });
}

export function setMatchResult(result: MatchResult | null) {
  update({ matchResult: result });
}

export function setIsFinishedHandled(handled: boolean) {
  update({ isFinishedHandled: handled });
}

export function setDisplayBalance(balance: number | null) {
  update({ displayBalance: balance });
}

const ACTIVITY_LIMIT = 50;

export function addActivity(message: string) {
  const item: ActivitySystemItem = {
    type: "system",
    message,
    timestamp: Date.now(),
  };
  update({
    activity: [...state.activity, item].slice(-ACTIVITY_LIMIT),
  });
}

export function addActivityItem(item: Omit<ActivityMoveItem, "type">) {
  const full: ActivityMoveItem = { ...item, type: "move" };
  update({
    activity: [...state.activity, full].slice(-ACTIVITY_LIMIT),
  });
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
