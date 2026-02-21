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
  mode: GameMode;
  status?: "waiting" | "starting" | "active" | "in_game" | "finished" | "cancelled";
  matchId?: string;
  readyPlayers?: number;
  readyUserIds: string[];
};

export type CreateRoomInput = {
  stakeUsd: number;
  mode: GameMode;
  deck: DeckSize;
  maxPlayers: number;
  title?: string;
};

export type MatchActionType = "attack" | "defend" | "take" | "pass";

export type MatchAction = {
  type: MatchActionType;
  cardId?: string;
};
