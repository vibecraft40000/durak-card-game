import { useLanguage } from "@/shared/providers/LanguageProvider";
import { type Theme } from "@/shared/lib/theme";
import { MoonIcon, SunIcon } from "@/shared/ui/Icons";

type ThemeToggleProps = {
  theme: Theme;
  onToggle: () => void;
  className?: string;
};

export function ThemeToggle({ theme, onToggle, className }: ThemeToggleProps) {
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);
  const isLight = theme === "light";

  return (
    <button
      type="button"
      onClick={onToggle}
      className={`theme-toggle ${isLight ? "theme-toggle--light" : "theme-toggle--dark"} ${className ?? ""}`}
      title={isLight ? tr("Тёмная тема", "Темна тема") : tr("Светлая тема", "Світла тема")}
      aria-label={
        isLight
          ? tr("Переключить на тёмную тему", "Перемкнути на темну тему")
          : tr("Переключить на светлую тему", "Перемкнути на світлу тему")
      }
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
