import type { ButtonHTMLAttributes, PropsWithChildren } from "react";

type AppButtonProps = PropsWithChildren<
  ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: "primary" | "default" | "secondary" | "ghost" | "inactive";
  }
>;

export function AppButton({
  variant = "default",
  className = "",
  children,
  ...props
}: AppButtonProps) {
  const variantClass =
    variant === "primary"
      ? "button--primary"
      : variant === "secondary"
        ? "button--secondary"
        : variant === "ghost"
          ? "button--ghost"
          : variant === "inactive"
            ? "button--inactive"
            : "";
  return (
    <button className={`button ${variantClass} ${className}`.trim()} {...props}>
      {children}
    </button>
  );
}
