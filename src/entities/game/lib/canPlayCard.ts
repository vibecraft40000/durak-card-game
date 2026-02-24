/**
 * Local validation for playing a card (mirrors server rules).
 * Used for drag & drop — reject invalid drops before WS send.
 */
import type { Card } from "@/entities/card/types";
import type { MatchStatePayload } from "@/shared/api/ws/types";

const RANK_ORDER: Record<string, number> = {
  "6": 6,
  "7": 7,
  "8": 8,
  "9": 9,
  "10": 10,
  J: 11,
  Q: 12,
  K: 13,
  A: 14,
};

function rankValue(rank: string): number {
  return RANK_ORDER[rank] ?? 0;
}

type CardLike = { suit: string; rank: string };

/** Card A beats card B (A defends against B). Trump beats non-trump; same suit higher rank. */
function cardBeats(attack: CardLike, defend: CardLike, trumpSuit: string): boolean {
  if (defend.suit === trumpSuit && attack.suit !== trumpSuit) return true;
  if (attack.suit !== trumpSuit && defend.suit === trumpSuit) return false;
  if (defend.suit !== attack.suit) return false;
  return rankValue(defend.rank) > rankValue(attack.rank);
}

/** Attack card rank matches any attack card on table (for подкидывание). */
function rankMatchesTable(card: CardLike, tableCards: CardLike[]): boolean {
  for (let i = 0; i < tableCards.length; i += 2) {
    if (tableCards[i] && tableCards[i].rank === card.rank) return true;
  }
  return false;
}

export function canPlayCard(
  card: Card,
  state: MatchStatePayload | null,
  currentUserId: string | null,
): { valid: boolean; action?: "attack" | "defend" } {
  if (!state || !currentUserId || state.status !== "playing") return { valid: false };
  if (state.turnPlayerId !== currentUserId) return { valid: false };

  const phase = state.phase ?? "attack";
  const tableCards = state.tableCards ?? [];
  const trumpSuit = state.trumpSuit ?? "";

  if (phase === "attack") {
    const numPairs = Math.floor(tableCards.length / 2);
    const hasUnpairedAttack = tableCards.length % 2 === 1;
    if (tableCards.length === 0 || (numPairs > 0 && !hasUnpairedAttack)) {
      return { valid: true, action: "attack" };
    }
    if (hasUnpairedAttack) {
      return rankMatchesTable(card, tableCards) ? { valid: true, action: "attack" } : { valid: false };
    }
  }

  if (phase === "defend") {
    const lastAttack = tableCards[tableCards.length - 1];
    if (!lastAttack) return { valid: false };
    return cardBeats(lastAttack, card, trumpSuit) ? { valid: true, action: "defend" } : { valid: false };
  }

  return { valid: false };
}
