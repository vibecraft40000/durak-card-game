export type Suit = "hearts" | "diamonds" | "clubs" | "spades";

export const CARD_RANK_ORDER = [
  "2",
  "3",
  "4",
  "5",
  "6",
  "7",
  "8",
  "9",
  "10",
  "J",
  "Q",
  "K",
  "A",
] as const;

export const FACE_CARD_RANKS = ["J", "Q", "K", "A"] as const;

export type Rank = (typeof CARD_RANK_ORDER)[number];

export type Card = {
  id: string;
  suit: Suit;
  rank: Rank;
};
