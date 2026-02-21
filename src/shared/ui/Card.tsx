import type { PropsWithChildren } from "react";

type AppCardProps = PropsWithChildren<{
  className?: string;
  /** Figma: Balance, List=Free, List=Busy, room-card, etc. */
  variant?: "default" | "balance" | "menu";
}>;

export function AppCard({
  className = "",
  variant = "default",
  children,
}: AppCardProps) {
  const variantClass = variant === "balance" ? "card--balance" : variant === "menu" ? "card--menu" : "";
  return <div className={`card ${variantClass} ${className}`.trim()}>{children}</div>;
}
