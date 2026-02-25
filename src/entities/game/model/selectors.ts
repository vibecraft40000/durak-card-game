import type { GameState } from "@/store/game.store";

/** Current game phase from match state */
export function selectCurrentPhase(
  state: GameState,
): "betting" | "playing" | "result" | undefined {
  return state.matchState?.phase;
}

/** Index of current player's seat, or -1 if unknown */
export function selectMySeatIndex(state: GameState): number {
  const ms = state.matchState;
  if (!ms || typeof ms.mySeatIndex !== "number") return -1;
  return ms.mySeatIndex;
}

/** Whether current player is the attacker */
export function selectIsAttacker(state: GameState): boolean {
  const ms = state.matchState;
  if (!ms) return false;
  const my = selectMySeatIndex(state);
  if (my < 0) return false;
  return ms.attackerSeat === my;
}

/** Whether current player is the defender */
export function selectIsDefender(state: GameState): boolean {
  const ms = state.matchState;
  if (!ms) return false;
  const my = selectMySeatIndex(state);
  if (my < 0) return false;
  return ms.defenderSeat === my;
}

/** Whether it is the current user's turn (coarse: any playable phase for now) */
export function selectIsMyTurn(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || !currentUserId) return false;
  if (ms.phase !== "playing") return false;
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
  if (!ms || ms.phase !== "playing") return false;
  if (!selectIsAttacker(state)) return false;

  const piles = ms.tablePiles ?? [];
  if (piles.length >= ms.capacityOnTable) return false;

  // Rough check: assume myHand length is encoded via seats[mySeatIndex].cardCount
  const myIndex = selectMySeatIndex(state);
  if (myIndex < 0 || !ms.seats?.[myIndex]) return false;
  const mySeat = ms.seats[myIndex];
  if (mySeat.cardCount <= 0) return false;

  return true;
}

/** Whether the current user can defend */
export function selectCanDefend(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || ms.phase !== "playing") return false;
  if (!selectIsDefender(state)) return false;

  const piles = ms.tablePiles ?? [];
  if (!piles.some((p) => !p.defend)) return false;

  const myIndex = selectMySeatIndex(state);
  if (myIndex < 0 || !ms.seats?.[myIndex]) return false;
  const mySeat = ms.seats[myIndex];
  if (mySeat.cardCount <= 0) return false;

  return true;
}

/** Whether the current user can take cards */
export function selectCanTake(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || ms.phase !== "playing") return false;
  if (!selectIsDefender(state)) return false;

  const piles = ms.tablePiles ?? [];
  if (piles.length === 0) return false;

  return true;
}

/** Whether the current user can pass */
export function selectCanPass(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || ms.phase !== "playing") return false;
  if (!selectIsAttacker(state)) return false;

  const piles = ms.tablePiles ?? [];
  if (piles.length === 0) return false;

  return true;
}

/** Whether the user can perform any game action */
export function selectCanAct(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (state.status !== "ready" || !ms || ms.phase !== "playing") return false;

  return (
    selectCanAttack(state, currentUserId) ||
    selectCanDefend(state, currentUserId) ||
    selectCanTake(state, currentUserId) ||
    selectCanPass(state, currentUserId)
  );
}

/** Seats ordered so that current player is always first, others follow clockwise */
export function selectOrderedSeats(state: GameState) {
  const ms = state.matchState;
  if (!ms || !ms.seats || ms.seats.length === 0) return [];
  const { seats, mySeatIndex } = ms;
  if (mySeatIndex == null || mySeatIndex < 0 || mySeatIndex >= seats.length) {
    return seats;
  }
  return [...seats.slice(mySeatIndex), ...seats.slice(0, mySeatIndex)];
}
