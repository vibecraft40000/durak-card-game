import type { ActivityItem, ActivityMoveItem } from "@/store/game.store";
import type { Player } from "@/entities/player/types";

const ACTION_LABELS: Record<string, string> = {
  attack: "подкинул",
  defend: "побил",
  take: "взял",
  pass: "пас",
};

function getPlayerName(players: Player[], playerId: string): string {
  const p = players.find((x) => x.id === playerId);
  return p?.displayName ?? p?.username ?? "Игрок";
}

export function formatActivityItem(
  item: ActivityItem,
  players: Player[],
): string {
  if (item.type === "system") {
    return item.message;
  }
  const move = item as ActivityMoveItem;
  const name = getPlayerName(players, move.playerId);
  const actionLabel = ACTION_LABELS[move.action] ?? move.action;
  const cardPart = move.cardId ? " (карта)" : "";
  return `${name}: ${actionLabel}${cardPart}`;
}
