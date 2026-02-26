/**
 * Local validation for playing a card (mirrors server rules).
 * Used for drag & drop to reject invalid drops before WS send.
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

/** Card A beats card B (A defends against B). */
function cardBeats(attack: CardLike, defend: CardLike, trumpSuit: string): boolean {
  if (defend.suit === trumpSuit && attack.suit !== trumpSuit) return true;
  if (attack.suit !== trumpSuit && defend.suit === trumpSuit) return false;
  if (defend.suit !== attack.suit) return false;
  return rankValue(defend.rank) > rankValue(attack.rank);
}

export function canPlayCard(
  card: Card,
  state: MatchStatePayload | null,
  currentUserId: string | null,
): { valid: boolean; action?: "attack" | "defend" } {
  if (!state || !currentUserId) return { valid: false };
  const phase = typeof state.phase === "string" ? state.phase : "";
  if (phase !== "attack" && phase !== "defend" && phase !== "playing") {
    return { valid: false };
  }
  if (state.turnPlayerId && state.turnPlayerId !== currentUserId) {
    return { valid: false };
  }

  const piles = state.tablePiles ?? buildTablePilesFromLegacy(state.tableCards ?? []);
  const trumpSuit = state.trumpSuit ?? "";

  // Defender logic.
  if (phase === "defend") {
    const openAttacks = piles.filter((p) => !p.defend);
    if (openAttacks.length === 0) return { valid: false };
    const target = openAttacks[openAttacks.length - 1].attack;
    return cardBeats(target, card, trumpSuit)
      ? { valid: true, action: "defend" }
      : { valid: false };
  }

  // Attacker logic.
  if (typeof state.capacityOnTable === "number" && state.capacityOnTable > 0) {
    if (piles.length >= state.capacityOnTable) return { valid: false };
  }
  if (piles.length === 0) {
    return { valid: true, action: "attack" };
  }

  const allowedRanks = state.allowedRanks ?? [];
  if (allowedRanks.length === 0 || allowedRanks.includes(card.rank)) {
    return { valid: true, action: "attack" };
  }

  return { valid: false };
}

function buildTablePilesFromLegacy(cards: Card[]): Array<{ attack: Card; defend?: Card }> {
  if (!Array.isArray(cards) || cards.length === 0) {
    return [];
  }
  const out: Array<{ attack: Card; defend?: Card }> = [];
  for (let index = 0; index < cards.length; index += 2) {
    out.push({
      attack: cards[index],
      defend: cards[index + 1],
    });
  }
  return out;
}
