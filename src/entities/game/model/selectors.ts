import type { GameState } from "@/store/game.store";

/** Whether it is the current user's turn (phase-aware, status must be "playing") */
export function selectIsMyTurn(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || !currentUserId) return false;
  if (ms.status !== "playing") return false;
  return ms.turnPlayerId === currentUserId;
}

/** Whether the current user can attack (phase === "attack" and is my turn) */
export function selectCanAttack(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || ms.status !== "playing") return false;
  const phase = ms.phase ?? "attack";
  return phase === "attack" && selectIsMyTurn(state, currentUserId);
}

/** Whether the current user can defend (phase === "defend" and is my turn) */
export function selectCanDefend(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || ms.status !== "playing") return false;
  const phase = ms.phase ?? "attack";
  return phase === "defend" && selectIsMyTurn(state, currentUserId);
}

/** Whether the current user can take cards (phase === "defend" and is my turn) */
export function selectCanTake(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || ms.status !== "playing") return false;
  const phase = ms.phase ?? "attack";
  return phase === "defend" && selectIsMyTurn(state, currentUserId);
}

/** Whether the current user can pass (phase === "attack" and is my turn) */
export function selectCanPass(
  state: GameState,
  currentUserId: string | null,
): boolean {
  const ms = state.matchState;
  if (!ms || ms.status !== "playing") return false;
  const phase = ms.phase ?? "attack";
  return phase === "attack" && selectIsMyTurn(state, currentUserId);
}

/** Whether the user can perform any game action (ready, playing, my turn) */
export function selectCanAct(
  state: GameState,
  currentUserId: string | null,
): boolean {
  if (state.status !== "ready") return false;
  return selectIsMyTurn(state, currentUserId);
}

/** Current game phase from match state */
export function selectCurrentPhase(
  state: GameState,
): "attack" | "defend" | undefined {
  return state.matchState?.phase;
}
