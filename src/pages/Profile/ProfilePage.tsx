import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getProfile, type UserProfile } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { useTheme } from "@/shared/providers/ThemeProvider";
import { ChevronRightIcon, SettingsIcon, UsersIcon } from "@/shared/ui/Icons";
import { AppCard } from "@/shared/ui/Card";
import { AppAvatar } from "@/shared/ui/Avatar";
import { ThemeToggle } from "@/shared/ui/ThemeToggle";

export function ProfilePage() {
  const { theme, toggleTheme } = useTheme();
  const { language } = useLanguage();
  const [user, setUser] = useState<UserProfile | null>(null);
  const [profileError, setProfileError] = useState(false);

  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  useEffect(() => {
    setProfileError(false);
    void getProfile()
      .then((response) => {
        setUser(response.user);
      })
      .catch(() => setProfileError(true));
  }, []);

  const fullName =
    user?.display_name ||
    (user?.username ? `@${user.username}` : [user?.first_name, user?.last_name].filter(Boolean).join(" ")) ||
    tr("Игрок", "Гравець");
  const avatarLetter = (user?.first_name || user?.username || "P").slice(0, 1).toUpperCase();

  return (
    <section className="screen profile-screen">
      <div className="profile-head">
        <ThemeToggle theme={theme} onToggle={toggleTheme} className="profile-head__btn" />
        <Link className="profile-head__btn icon-button" to="/profile/settings" aria-label={tr("Настройки", "Налаштування")}>
          <SettingsIcon size={18} />
        </Link>
        <Link className="profile-head__btn icon-button" to="/profile/friends" aria-label={tr("Друзья", "Друзі")}>
          <UsersIcon size={18} />
        </Link>
      </div>

      <div className="profile-hero">
        <div className="avatar-badge">
          <AppAvatar name={fullName || avatarLetter} photoUrl={user?.photo_url} />
        </div>
        <div className="profile-name">{fullName}</div>
      </div>

      {profileError && (
        <AppCard>
          <div className="card__hint card__hint--error">{tr("Ошибка загрузки", "Помилка завантаження")}</div>
        </AppCard>
      )}

      <div className="list">
        <Link className="menu-item" to="/profile/friends">
          <span>{tr("Друзья", "Друзі")}</span>
          <ChevronRightIcon size={16} />
        </Link>

        <Link className="menu-item" to="/profile/history/games">
          <span>{tr("История игр", "Історія ігор")}</span>
          <ChevronRightIcon size={16} />
        </Link>

        <Link className="menu-item" to="/profile/history/transactions">
          <span>{tr("История транзакций", "Історія транзакцій")}</span>
          <ChevronRightIcon size={16} />
        </Link>
      </div>
    </section>
  );
}
