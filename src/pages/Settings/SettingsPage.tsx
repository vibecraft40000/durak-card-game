import { Link } from "react-router-dom";
import { useEffect, useState } from "react";
import { getProfile, getUserSettings } from "@/shared/api/user";
import { BackIcon, ChevronRightIcon } from "@/shared/ui/Icons";
import { ThemeToggle } from "@/shared/ui/ThemeToggle";
import { useTheme } from "@/shared/providers/ThemeProvider";

function languageLabel(code: string | undefined) {
  if (code === "uk") {
    return "Украинский";
  }
  return "Русский";
}

export function SettingsPage() {
  const { theme, toggleTheme } = useTheme();
  const [displayName, setDisplayName] = useState("Игрок");
  const [language, setLanguage] = useState("ru");

  useEffect(() => {
    void getUserSettings()
      .then((response) => {
        setDisplayName(response.settings.displayName || "Игрок");
      })
      .catch(() => undefined);
    void getProfile()
      .then((response) => {
        setLanguage(response.user.language ?? "ru");
      })
      .catch(() => undefined);
  }, []);

  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile">
          <BackIcon size={18} />
        </Link>
        <h1 className="page-header__title">Настройки</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="list">
        <div className="menu-item menu-item--theme">
          <span>Тема</span>
          <ThemeToggle theme={theme} onToggle={toggleTheme} />
        </div>
        <Link className="menu-item" to="/profile/settings/name">
          <span>Имя</span>
          <span className="settings-row__value">
            {displayName} <ChevronRightIcon size={14} />
          </span>
        </Link>
        <Link className="menu-item" to="/profile/settings/language">
          <span>Язык</span>
          <span className="settings-row__value">
            {languageLabel(language)} <ChevronRightIcon size={14} />
          </span>
        </Link>
      </div>
    </section>
  );
}
