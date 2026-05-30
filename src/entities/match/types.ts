export type GameMode = "Подкидной" | "Переводной";
export type DeckSize = 24 | 36 | 52;

export type Room = {
  id: string;
  title: string;
  stakeUsd: number;
  players: number;
  playerIds: string[];
  maxPlayers: number;
  deck: DeckSize;
  mode: string;
  status?:
    | "waiting"
    | "starting"
    | "active"
    | "awaiting_stake_confirm"
    | "in_game"
    | "finished"
    | "cancelled";
  matchId?: string;
  readyPlayers?: number;
  readyUserIds: string[];
  stakeConfirmedPlayers?: number;
  stakeConfirmedUserIds: string[];
  stakeConfirmDeadline?: number;
};

export type CreateRoomInput = {
  stakeUsd: number;
  mode: string;
  deck: DeckSize;
  maxPlayers: number;
  title?: string;
};

export type MatchActionType =
  | "attack"
  | "defend"
  | "throw"
  | "shuler_play"
  | "take"
  | "pass"
  | "translate"
  | "shuler_report";

export type MatchAction = {
  type: MatchActionType;
  cardId?: string;
};
