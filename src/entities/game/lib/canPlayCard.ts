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

export function canPlayCard(
  card: Card,
  state: MatchStatePayload | null,
  currentUserId: string | null,
): { valid: boolean; action?: "attack" | "defend" } {
  if (!state || !currentUserId) return { valid: false };
  if (state.phase !== "playing") return { valid: false };

  const myIndex = typeof state.mySeatIndex === "number" ? state.mySeatIndex : -1;
  if (myIndex < 0 || !state.seats || !state.seats[myIndex]) {
    return { valid: false };
  }

  const piles = state.tablePiles ?? [];
  const trumpSuit = state.trumpSuit ?? "";

  const isAttacker = state.attackerSeat === myIndex;
  const isDefender = state.defenderSeat === myIndex;

  // Defender logic: can we beat at least one open attack on table?
  if (isDefender) {
    const openAttacks = piles.filter((p) => !p.defend);
    if (!openAttacks.length) return { valid: false };

    // For now, target the last open attack (closest to current interaction)
    const target = openAttacks[openAttacks.length - 1].attack;
    return cardBeats(target, card, trumpSuit) ? { valid: true, action: "defend" } : { valid: false };
  }

  // Attacker logic: first throw or подкидывание by rank
  if (isAttacker) {
    if (piles.length >= state.capacityOnTable) return { valid: false };

    // First attack: any card is allowed
    if (piles.length === 0) {
      return { valid: true, action: "attack" };
    }

    // Subsequent throws: rank must be allowed by server rules
    const allowedRanks = state.allowedRanks ?? [];
    if (allowedRanks.includes(card.rank)) {
      return { valid: true, action: "attack" };
    }

    return { valid: false };
  }

  // Neither attacker nor defender: cannot play a card
  return { valid: false };
}
