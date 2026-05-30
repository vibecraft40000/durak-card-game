/**
 * Local validation for card actions — mirrors server rules.
 * Used by Interaction Engine before ACCEPT; server remains source of truth.
 */
import type { Card } from "@/entities/card/types";
import type { MatchStatePayload } from "@/shared/api/ws/types";
import type { DropZone } from "./zones";
import { canPlayCard } from "@/entities/game/lib/canPlayCard";

export type ValidationResult =
  | { valid: true; action: "attack" | "defend" | "throw" | "shuler_play" }
  | { valid: false };

/**
 * Validates whether a card drop to the given zone is a legal move.
 */
export function isValidMove(
  card: Card,
  zone: DropZone | null,
  matchState: MatchStatePayload | null,
  currentUserId: string | null,
): ValidationResult {
  if (!zone || zone !== "table") return { valid: false };

  const result = canPlayCard(card, matchState, currentUserId);
  if (!result.valid || !result.action) return { valid: false };
  return { valid: true, action: result.action };
}
