/**
 * Interaction Engine — central layer for card drop validation.
 * Decides: ACCEPT → play animation + dispatch; REJECT → shake; REJECT_SILENT → ignore.
 *
 * Flow: Card.tsx → gesture controller → interaction.engine.validate()
 *       → ACCEPT → animation.throw() → network.dispatcher.send()
 *
 * Does NOT know: version, backend, animation details.
 * Knows: zones, validation rules, interactionLocked.
 */
import type { Card } from "@/entities/card/types";
import type { MatchStatePayload } from "@/shared/api/ws/types";
import type { Point } from "./zones";
import { detectZone, type TableRect } from "./zones";
import { isValidMove } from "./validation";

export type DropResult =
  | { outcome: "accept"; action: "attack" | "defend"; cardId: string }
  | { outcome: "reject"; cardId: string }
  | { outcome: "reject_silent" };

export type ValidateDropContext = {
  card: Card;
  position: Point;
  tableRect: TableRect | null;
  matchState: MatchStatePayload | null;
  currentUserId: string | null;
  interactionLocked: boolean;
};

/**
 * Validates a card drop. Called by gesture layer (e.g. PlayerHandFan onDragEnd).
 */
export function validateCardDrop(ctx: ValidateDropContext): DropResult {
  const {
    card,
    position,
    tableRect,
    matchState,
    currentUserId,
    interactionLocked,
  } = ctx;

  if (interactionLocked) {
    return { outcome: "reject_silent" };
  }

  const zone = detectZone(position, tableRect);
  const validation = isValidMove(card, zone, matchState, currentUserId);

  if (!validation.valid) {
    return { outcome: "reject", cardId: card.id };
  }

  return {
    outcome: "accept",
    action: validation.action,
    cardId: card.id,
  };
}
