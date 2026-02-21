import type { ButtonHTMLAttributes, PropsWithChildren } from "react";

type AppButtonProps = PropsWithChildren<
  ButtonHTMLAttributes<HTMLButtonElement> & {
    /** Figma: primary, secondary, not active */
    variant?: "primary" | "default" | "secondary" | "ghost" | "inactive" | "deposit" | "withdraw";
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
            : variant === "deposit"
              ? "button--deposit"
              : variant === "withdraw"
                ? "button--withdraw"
                : "";
  return (
    <button className={`button ${variantClass} ${className}`.trim()} {...props}>
      {children}
    </button>
  );
}
