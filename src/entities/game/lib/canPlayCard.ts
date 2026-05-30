/**
 * Local validation for playing a card (mirrors server rules).
 * Used for drag & drop to reject invalid drops before WS send.
 */
import { CARD_RANK_ORDER, type Card } from "@/entities/card/types";
import type { MatchStatePayload } from "@/shared/api/ws/types";

function rankValue(rank: string): number {
  return CARD_RANK_ORDER.indexOf(rank as Card["rank"]);
}

type CardLike = { suit: string; rank: string };
type TablePileLike = NonNullable<MatchStatePayload["tablePiles"]>[number];

function getDefenseCard(pile: TablePileLike): Card | undefined {
  return pile.defend ?? pile.defense;
}

function includesCard(cardIds: string[] | undefined, cardId: string): boolean {
  return Array.isArray(cardIds) && cardIds.includes(cardId);
}

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
): { valid: boolean; action?: "attack" | "defend" | "throw" | "shuler_play" } {
  if (!state || !currentUserId) return { valid: false };
  const phase = typeof state.phase === "string" ? state.phase : "";
  if (phase !== "attack" && phase !== "defend" && phase !== "playing") {
    return { valid: false };
  }
  const affordances = state.affordances;
  if (affordances) {
    if (phase === "defend") {
      if (includesCard(affordances.defendCardIds, card.id)) {
        return { valid: true, action: "defend" };
      }
      if (includesCard(affordances.shulerPlayCardIds, card.id)) {
        return { valid: true, action: "shuler_play" };
      }
      return { valid: false };
    }

    if (state.turnPlayerId && state.turnPlayerId !== currentUserId) {
      return includesCard(affordances.throwInCardIds, card.id)
        ? { valid: true, action: "throw" }
        : { valid: false };
    }

    return includesCard(affordances.attackCardIds, card.id)
      ? { valid: true, action: "attack" }
      : { valid: false };
  }

  const piles = state.tablePiles ?? buildTablePilesFromLegacy(state.tableCards ?? []);
  const trumpSuit = state.trumpSuit ?? "";
  const defenderPlayerId =
    typeof state.defenderPlayerId === "string" ? state.defenderPlayerId : null;

  // Defender logic.
  if (phase === "defend") {
    if (state.turnPlayerId && state.turnPlayerId !== currentUserId) {
      return { valid: false };
    }

    const openAttacks = piles.filter((pile) => !getDefenseCard(pile));
    if (openAttacks.length === 0) return { valid: false };
    const target = openAttacks[openAttacks.length - 1].attack;
    if (cardBeats(target, card, trumpSuit)) {
      return { valid: true, action: "defend" };
    }
    const shulerActivePlayers = state.shuler?.activePlayers ?? [];
    if (shulerActivePlayers.includes(currentUserId)) {
      return { valid: true, action: "shuler_play" };
    }
    return { valid: false };
  }

  // Throw-in by non-defender players during attacker phase.
  if (phase === "attack" && state.turnPlayerId && state.turnPlayerId !== currentUserId) {
    if (defenderPlayerId && defenderPlayerId === currentUserId) return { valid: false };
    if (typeof state.capacityOnTable === "number" && state.capacityOnTable > 0) {
      if (piles.length >= state.capacityOnTable) return { valid: false };
    }
    if (piles.length === 0) return { valid: false };
    const hasRankOnTable = piles.some((pile) => {
      const defenseCard = getDefenseCard(pile);
      return (
        pile.attack.rank === card.rank ||
        defenseCard?.rank === card.rank
      );
    });
    return hasRankOnTable ? { valid: true, action: "throw" } : { valid: false };
  }

  // Attacker logic.
  if (state.turnPlayerId && state.turnPlayerId !== currentUserId) {
    return { valid: false };
  }
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
