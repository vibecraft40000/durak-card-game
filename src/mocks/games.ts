export type GameRoom = {
  id: string;
  title: string;
  stakeUsd: number;
  players: number;
  maxPlayers: number;
  deck: 24 | 36 | 52;
  mode: "Подкидной" | "Переводной";
};

export const MOCK_ROOMS: GameRoom[] = [
  {
    id: "room-101",
    title: "Быстрая игра",
    stakeUsd: 10,
    players: 2,
    maxPlayers: 2,
    deck: 36,
    mode: "Подкидной",
  },
  {
    id: "room-202",
    title: "Турнирный стол",
    stakeUsd: 50,
    players: 3,
    maxPlayers: 4,
    deck: 52,
    mode: "Переводной",
  },
  {
    id: "room-303",
    title: "Легкий вход",
    stakeUsd: 5,
    players: 2,
    maxPlayers: 4,
    deck: 24,
    mode: "Подкидной",
  },
];
