import { memo } from "react";
import type { Suit } from "@/entities/card/types";

const SUIT_SYMBOLS: Record<string, string> = {
  hearts: "♥",
  diamonds: "♦",
  clubs: "♣",
  spades: "♠",
};

const FACE_RANKS = new Set(["J", "Q", "K", "A"]);

function getSuitSymbol(suit: string): string {
  return SUIT_SYMBOLS[suit] ?? "?";
}

function getSuitModifier(suit: string): string {
  return suit === "hearts" || suit === "diamonds" ? "playing-card--red" : "playing-card--dark";
}

type PlayingCardProps = {
  rank?: string;
  suit?: Suit | string;
  faceUp?: boolean;
  variant?: "table" | "hand" | "mini";
  selected?: boolean;
  placeholder?: boolean;
  interactive?: boolean;
  dimmed?: boolean;
  onClick?: () => void;
};

export const PlayingCard = memo(function PlayingCard({
  rank,
  suit,
  faceUp = true,
  variant = "hand",
  selected = false,
  placeholder = false,
  interactive = false,
  dimmed = false,
  onClick,
}: PlayingCardProps) {
  const baseClass = "playing-card";
  const variantClass = variant === "table" ? "playing-card--table" : variant === "mini" ? "playing-card--mini" : "";
  const suitClass = suit ? getSuitModifier(suit) : "";
  const selectedClass = selected ? "playing-card--selected" : "";
  const dimmedClass = dimmed ? "playing-card--dimmed" : "";

  if (placeholder) {
    return <div className={`${baseClass} playing-card--placeholder ${variantClass}`} aria-hidden />;
  }

  if (!faceUp) {
    return (
      <div className={`${baseClass} playing-card--back ${variantClass}`} aria-label="card back">
        <span className="playing-card__back-pattern" />
      </div>
    );
  }

  const symbol = suit ? getSuitSymbol(suit) : "";
  const center = rank && FACE_RANKS.has(rank) ? rank : symbol;

  const content = (
    <>
      <span className="playing-card__corner playing-card__corner--top">
        <span>{rank ?? ""}</span>
        <span>{symbol}</span>
      </span>
      <span className="playing-card__center">{center}</span>
      <span className="playing-card__corner playing-card__corner--bottom" aria-hidden>
        <span>{rank ?? ""}</span>
        <span>{symbol}</span>
      </span>
    </>
  );

  const className = [baseClass, variantClass, suitClass, selectedClass, dimmedClass].filter(Boolean).join(" ");

  if (interactive && onClick) {
    return (
      <button type="button" className={className} onClick={onClick} aria-label={`card ${rank ?? ""} ${symbol}`}>
        {content}
      </button>
    );
  }

  return <div className={className}>{content}</div>;
});
