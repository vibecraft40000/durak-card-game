import { Link } from "react-router-dom";
import { useEffect, useState } from "react";
import { getUserSettings } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon, ChevronRightIcon } from "@/shared/ui/Icons";
import { ThemeToggle } from "@/shared/ui/ThemeToggle";
import { useTheme } from "@/shared/providers/ThemeProvider";

export function SettingsPage() {
  const { theme, toggleTheme } = useTheme();
  const { language, t } = useLanguage();
  const [displayName, setDisplayName] = useState(t("settings.playerDefault"));

  useEffect(() => {
    void getUserSettings()
      .then((response) => {
        setDisplayName(response.settings.displayName || t("settings.playerDefault"));
      })
      .catch(() => undefined);
  }, [t]);

  const languageLabel = language === "uk" ? t("settings.language.uk") : t("settings.language.ru");

  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile">
          <BackIcon size={18} />
        </Link>
        <h1 className="page-header__title">{t("settings.title")}</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="list">
        <div className="menu-item menu-item--theme">
          <span>{t("settings.theme")}</span>
          <ThemeToggle theme={theme} onToggle={toggleTheme} />
        </div>
        <Link className="menu-item" to="/profile/settings/name">
          <span>{t("settings.name")}</span>
          <span className="settings-row__value">
            {displayName} <ChevronRightIcon size={14} />
          </span>
        </Link>
        <Link className="menu-item" to="/profile/settings/language">
          <span>{t("settings.language")}</span>
          <span className="settings-row__value">
            {languageLabel} <ChevronRightIcon size={14} />
          </span>
        </Link>
      </div>
    </section>
  );
}
