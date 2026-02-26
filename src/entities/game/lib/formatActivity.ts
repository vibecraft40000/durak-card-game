import type { ActivityItem, ActivityMoveItem } from "@/store/game.store";
import type { Player } from "@/entities/player/types";
import { trRuntime } from "@/shared/i18n/runtime";

function getPlayerName(players: Player[], playerId: string): string {
  const p = players.find((x) => x.id === playerId);
  return p?.displayName ?? p?.username ?? trRuntime("Игрок", "Гравець");
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
  const actionLabels: Record<string, string> = {
    attack: trRuntime("подкинул", "підкинув"),
    defend: trRuntime("побил", "побив"),
    take: trRuntime("взял", "взяв"),
    pass: trRuntime("пас", "пас"),
  };
  const actionLabel = actionLabels[move.action] ?? move.action;
  const cardPart = move.cardId ? trRuntime(" (карта)", " (карта)") : "";
  return `${name}: ${actionLabel}${cardPart}`;
}
