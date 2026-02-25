import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getProfile, type UserProfile } from "@/shared/api/user";
import { useTheme } from "@/shared/providers/ThemeProvider";
import { ChevronRightIcon, DepositIcon, SettingsIcon, UsersIcon, WithdrawIcon } from "@/shared/ui/Icons";
import { AppCard } from "@/shared/ui/Card";
import { AppAvatar } from "@/shared/ui/Avatar";
import { ThemeToggle } from "@/shared/ui/ThemeToggle";

const CURRENCY = "USD" as const;

export function ProfilePage() {
  const { theme, toggleTheme } = useTheme();
  const [user, setUser] = useState<UserProfile | null>(null);
  const [balance, setBalance] = useState<number | null>(null);
  const [profileError, setProfileError] = useState(false);

  useEffect(() => {
    setProfileError(false);
    void getProfile()
      .then((response) => {
        setUser(response.user);
        setBalance(response.balance);
      })
      .catch(() => setProfileError(true));
  }, []);

  const fullName =
    user?.display_name ||
    (user?.username ? `@${user.username}` : [user?.first_name, user?.last_name].filter(Boolean).join(" ")) ||
    "Игрок";
  const avatarLetter = (user?.first_name || user?.username || "P").slice(0, 1).toUpperCase();

  return (
    <section className="screen profile-screen">
      <div className="profile-head">
        <ThemeToggle theme={theme} onToggle={toggleTheme} className="profile-head__btn" />
        <Link className="profile-head__btn icon-button" to="/profile/settings" aria-label="Настройки">
          <SettingsIcon size={18} />
        </Link>
        <Link className="profile-head__btn icon-button" to="/profile/friends" aria-label="Друзья">
          <UsersIcon size={18} />
        </Link>
      </div>

      <div className="profile-hero">
        <div className="avatar-badge">
          <AppAvatar name={fullName || avatarLetter} photoUrl={user?.photo_url} />
        </div>
        <div className="profile-name">{fullName}</div>
      </div>

      <AppCard variant="balance" className="profile-balance">
        <div className="card__row profile-balance__top">
          <div className="profile-balance__value">
            {profileError
              ? `Ошибка загрузки — ${CURRENCY}`
              : balance != null && Number.isFinite(balance)
                ? `${balance.toFixed(3)} ${CURRENCY}`
                : `— ${CURRENCY}`}
          </div>
        </div>
        <div className="profile-balance__actions">
          <Link
            to="/profile/deposit"
            className="button button--deposit"
            style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 8, textDecoration: "none", color: "inherit" }}
          >
            <DepositIcon size={18} />
            Ввод
          </Link>
          <Link
            to="/profile/withdraw"
            className="button button--withdraw"
            style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 8, textDecoration: "none", color: "inherit" }}
          >
            <WithdrawIcon size={18} />
            Вывод
          </Link>
        </div>
      </AppCard>

      <div className="list">
        <Link className="menu-item" to="/profile/deposit">
          <span>Пополнение (Crypto Bot)</span>
          <ChevronRightIcon size={16} />
        </Link>
        <Link className="menu-item" to="/profile/friends">
          <span>Друзья</span>
          <ChevronRightIcon size={16} />
        </Link>

        <Link className="menu-item" to="/profile/history/games">
          <span>История игр</span>
          <ChevronRightIcon size={16} />
        </Link>

        <Link className="menu-item" to="/profile/history/transactions">
          <span>История транзакций</span>
          <ChevronRightIcon size={16} />
        </Link>
      </div>

    </section>
  );
}
