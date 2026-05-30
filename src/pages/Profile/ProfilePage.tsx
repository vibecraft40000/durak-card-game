import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getConfig } from "@/shared/api/config";
import { getProfile, type UserProfile } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { useTheme } from "@/shared/providers/ThemeProvider";
import { ChevronRightIcon, DepositIcon, SettingsIcon, UsersIcon, WithdrawIcon } from "@/shared/ui/Icons";
import { AppCard } from "@/shared/ui/Card";
import { AppAvatar } from "@/shared/ui/Avatar";
import { ThemeToggle } from "@/shared/ui/ThemeToggle";

const CURRENCY = "USD" as const;

export function ProfilePage() {
  const { theme, toggleTheme } = useTheme();
  const { language } = useLanguage();
  const [user, setUser] = useState<UserProfile | null>(null);
  const [balance, setBalance] = useState<number | null>(null);
  const [profileError, setProfileError] = useState(false);
  const [depositsEnabled, setDepositsEnabled] = useState(false);
  const [withdrawalsEnabled, setWithdrawalsEnabled] = useState(false);

  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  useEffect(() => {
    setProfileError(false);
    void getProfile()
      .then((response) => {
        setUser(response.user);
        setBalance(response.balance);
      })
      .catch(() => setProfileError(true));
    void getConfig()
      .then((config) => {
        setDepositsEnabled(config.depositsEnabled === true);
        setWithdrawalsEnabled(config.withdrawalsEnabled === true);
      })
      .catch(() => {
        setDepositsEnabled(false);
        setWithdrawalsEnabled(false);
      });
  }, []);

  const fullName =
    user?.display_name ||
    (user?.username ? `@${user.username}` : [user?.first_name, user?.last_name].filter(Boolean).join(" ")) ||
    tr("Игрок", "Гравець");
  const avatarLetter = (user?.first_name || user?.username || "P").slice(0, 1).toUpperCase();
  const actionButtonStyle = {
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
    textDecoration: "none",
    color: "inherit",
  } as const;

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

      <AppCard variant="balance" className="profile-balance">
        <div className="card__row profile-balance__top">
          <div className="profile-balance__value">
            {profileError
              ? `${tr("Ошибка загрузки", "Помилка завантаження")} — ${CURRENCY}`
              : balance != null && Number.isFinite(balance)
                ? `${balance.toFixed(3)} ${CURRENCY}`
                : `— ${CURRENCY}`}
          </div>
        </div>
        <div className="profile-balance__actions">
          {depositsEnabled ? (
            <Link
              to="/profile/deposit"
              className="button button--deposit"
              style={actionButtonStyle}
            >
              <DepositIcon size={18} />
              {tr("Ввод", "Внесення")}
            </Link>
          ) : (
            <button
              type="button"
              className="button button--deposit"
              disabled
              title={tr("Пополнение временно недоступно в публичной бете", "Поповнення тимчасово недоступне в публічній beta")}
              style={{ ...actionButtonStyle, opacity: 0.55, cursor: "not-allowed" }}
            >
              <DepositIcon size={18} />
              {tr("Ввод скоро", "Внесення згодом")}
            </button>
          )}
          {withdrawalsEnabled ? (
            <Link to="/profile/withdraw" className="button button--withdraw" style={actionButtonStyle}>
              <WithdrawIcon size={18} />
              {tr("Вывод", "Виведення")}
            </Link>
          ) : (
            <button
              type="button"
              className="button button--withdraw"
              disabled
              title={tr("Вывод временно недоступен в публичной бете", "Виведення тимчасово недоступне в публічній beta")}
              style={{ ...actionButtonStyle, opacity: 0.55, cursor: "not-allowed" }}
            >
              <WithdrawIcon size={18} />
              {tr("Вывод скоро", "Виведення згодом")}
            </button>
          )}
        </div>
      </AppCard>

      <div className="list">
        {depositsEnabled ? (
          <Link className="menu-item" to="/profile/deposit">
            <span>{tr("Пополнение (Crypto Bot)", "Поповнення (Crypto Bot)")}</span>
            <ChevronRightIcon size={16} />
          </Link>
        ) : (
          <button
            type="button"
            className="menu-item"
            disabled
            title={tr("Пополнение временно недоступно в публичной бете", "Поповнення тимчасово недоступне в публічній beta")}
            style={{ opacity: 0.55, cursor: "not-allowed", textAlign: "left" }}
          >
            <span>{tr("Пополнение скоро", "Поповнення згодом")}</span>
          </button>
        )}
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
