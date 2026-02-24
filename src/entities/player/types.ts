import type { Card } from "@/entities/card/types";

export type Player = {
  id: string;
  username: string;
  displayName?: string;
  photoUrl?: string;
  balanceUsd?: number;
  handCount: number;
  isCurrentTurn: boolean;
  hand?: Card[];
};
