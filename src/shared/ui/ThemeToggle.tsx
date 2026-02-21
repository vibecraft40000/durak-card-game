import { type Theme } from "@/shared/lib/theme";
import { MoonIcon, SunIcon } from "@/shared/ui/Icons";

type ThemeToggleProps = {
  theme: Theme;
  onToggle: () => void;
  className?: string;
};

export function ThemeToggle({ theme, onToggle, className }: ThemeToggleProps) {
  const isLight = theme === "light";

  return (
    <button
      type="button"
      onClick={onToggle}
      className={`theme-toggle ${isLight ? "theme-toggle--light" : "theme-toggle--dark"} ${className ?? ""}`}
      title={isLight ? "Тёмная тема" : "Светлая тема"}
      aria-label={isLight ? "Переключить на тёмную тему" : "Переключить на светлую тему"}
    >
      <span className="theme-toggle__track" />
      <span className="theme-toggle__icon theme-toggle__icon--sun">
        <SunIcon size={14} />
      </span>
      <span className="theme-toggle__icon theme-toggle__icon--moon">
        <MoonIcon size={14} />
      </span>
      <span className={`theme-toggle__thumb ${isLight ? "theme-toggle__thumb--right" : ""}`} />
    </button>
  );
}
