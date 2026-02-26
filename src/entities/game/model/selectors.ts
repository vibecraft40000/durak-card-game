import type { GameState } from "@/store/game.store";
import type { MatchStatePayload, Seat } from "@/shared/api/ws/types";

function rawPhase(ms: MatchStatePayload | null): string {
  return typeof ms?.phase === "string" ? ms.phase : "";
}

function isPlayingPhase(phase: string): boolean {
  return phase === "playing" || phase === "attack" || phase === "defend";
}

function getTableCardCount(ms: MatchStatePayload | null): number {
  if (!ms) return 0;
  if (Array.isArray(ms.tablePiles) && ms.tablePiles.length > 0) return ms.tablePiles.length;
  if (Array.isArray(ms.tableCards) && ms.tableCards.length > 0) return ms.tableCards.length;
  return 0;
}

function getMyHandCount(ms: MatchStatePayload | null, currentUserId: string | null): number {
  if (!ms) return 0;
  if (currentUserId && Array.isArray(ms.players)) {
    const me = ms.players.find((p) => p.id === currentUserId);
    if (me) {
      if (typeof me.handCount === "number") return me.handCount;
      if (Array.isArray(me.hand)) return me.hand.length;
    }
  }
  if (Array.isArray(ms.seats) && typeof ms.mySeatIndex === "number") {
    const seat = ms.seats[ms.mySeatIndex];
    if (seat && typeof seat.cardCount === "number") return seat.cardCount;
  }
  return 0;
}

/** Current game phase from match state */
export function selectCurrentPhase(
  state: GameState,
): "betting" | "playing" | "result" | undefined {
  const ms = state.matchState;
  if (!ms) return undefined;
  const phase = rawPhase(ms);
  if (phase === "result" || ms.status === "finished") return "result";
  if (isPlayingPhase(phase)) return "playing";
  if (phase === "betting") return "betting";
  return undefined;
}

/** Index of current player's seat, or -1 if unknown */
export function selectMySeatIndex(state: GameState): number {
  const ms = state.matchState;
  if (!ms || typeof ms.mySeatIndex !== "number") return -1;
  return ms.mySeatIndex;
}

/** Whether current player is the attacker */
export function selectIsAttacker(state: GameState, currentUserId: string | null): boolean {
  const ms = state.matchState;
  if (!ms) return false;
  return selectIsMyTurn(state, currentUserId) && rawPhase(ms) === "attack";
}

/** Whether current player is the defender */
export function selectIsDefender(state: GameState, currentUserId: string | null): boolean {
  const ms = state.matchState;
  if (!ms) return false;
  return selectIsMyTurn(state, currentUserId) && rawPhase(ms) === "defend";
}

/** Whether it is the current user's turn */
export function selectIsMyTurn(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || !currentUserId) return false;
  if (selectCurrentPhase(state) !== "playing") return false;

  if (typeof ms.turnPlayerId === "string" && ms.turnPlayerId !== "") {
    return ms.turnPlayerId === currentUserId;
  }

  const myIndex = selectMySeatIndex(state);
  if (myIndex < 0) return false;
  if (typeof ms.turnSeatIndex !== "number") return false;
  return ms.turnSeatIndex === myIndex;
}

/** Whether the current user can attack */
export function selectCanAttack(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || selectCurrentPhase(state) !== "playing") return false;
  if (!selectIsAttacker(state, currentUserId)) return false;

  if (typeof ms.capacityOnTable === "number" && ms.capacityOnTable > 0) {
    if (getTableCardCount(ms) >= ms.capacityOnTable) return false;
  }

  return getMyHandCount(ms, currentUserId) > 0;
}

/** Whether the current user can defend */
export function selectCanDefend(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || selectCurrentPhase(state) !== "playing") return false;
  if (!selectIsDefender(state, currentUserId)) return false;

  if (getTableCardCount(ms) === 0) return false;
  return getMyHandCount(ms, currentUserId) > 0;
}

/** Whether the current user can take cards */
export function selectCanTake(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || selectCurrentPhase(state) !== "playing") return false;
  if (!selectIsDefender(state, currentUserId)) return false;
  return getTableCardCount(ms) > 0;
}

/** Whether the current user can pass */
export function selectCanPass(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || selectCurrentPhase(state) !== "playing") return false;
  if (!selectIsAttacker(state, currentUserId)) return false;
  return getTableCardCount(ms) > 0;
}

/** Whether the user can perform any game action */
export function selectCanAct(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (state.status !== "ready" || !ms || selectCurrentPhase(state) !== "playing") return false;

  return (
    selectCanAttack(state, currentUserId) ||
    selectCanDefend(state, currentUserId) ||
    selectCanTake(state, currentUserId) ||
    selectCanPass(state, currentUserId)
  );
}

/** Seats ordered so that current player is always first, others follow clockwise */
export function selectOrderedSeats(state: GameState, currentUserId: string | null): Seat[] {
  const ms = state.matchState;
  if (!ms) return [];

  let seats: Seat[] = [];
  if (Array.isArray(ms.seats) && ms.seats.length > 0) {
    seats = ms.seats;
  } else if (Array.isArray(ms.players) && ms.players.length > 0) {
    seats = ms.players.map((p) => ({
      id: p.id,
      name: p.displayName ?? p.username ?? p.id,
      cardCount: typeof p.handCount === "number" ? p.handCount : Array.isArray(p.hand) ? p.hand.length : 0,
      isReady: true,
      isConfirmed: true,
      avatarUrl: p.photoUrl,
    }));
  }

  if (seats.length === 0) return [];

  let mySeatIndex = typeof ms.mySeatIndex === "number" ? ms.mySeatIndex : -1;
  if (mySeatIndex < 0 && currentUserId) {
    mySeatIndex = seats.findIndex((s) => s.id === currentUserId);
  }
  if (mySeatIndex < 0 || mySeatIndex >= seats.length) {
    return seats;
  }
  return [...seats.slice(mySeatIndex), ...seats.slice(0, mySeatIndex)];
}
