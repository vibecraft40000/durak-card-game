import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getProfile, patchUserSettings, type UserProfile } from "@/shared/api/user";
import { ChevronRightIcon, DepositIcon, SettingsIcon, UsersIcon, WithdrawIcon } from "@/shared/ui/Icons";
import { ThemeToggle } from "@/shared/ui/ThemeToggle";
import { useTheme } from "@/shared/providers/ThemeProvider";
import { AppCard } from "@/shared/ui/Card";
import { AppAvatar } from "@/shared/ui/Avatar";
import { AppButton } from "@/shared/ui/Button";

const CURRENCIES = ["USD", "RUB", "UAH"] as const;
type Currency = (typeof CURRENCIES)[number];

export function ProfilePage() {
  const { theme, toggleTheme } = useTheme();
  const [user, setUser] = useState<UserProfile | null>(null);
  const [balance, setBalance] = useState(0);
  const [currencySaving, setCurrencySaving] = useState(false);

  useEffect(() => {
    void getProfile()
      .then((response) => {
        setUser(response.user);
        setBalance(response.balance);
      })
      .catch(() => undefined);
  }, []);

  const currency = (user?.currency ?? "USD") as Currency;

  async function changeCurrency(next: Currency) {
    if (next === currency || currencySaving) return;
    setCurrencySaving(true);
    try {
      await patchUserSettings({ currency: next });
      setUser((prev) => (prev ? { ...prev, currency: next } : null));
    } finally {
      setCurrencySaving(false);
    }
  }

  const fullName =
    user?.display_name ||
    (user?.username ? `@${user.username}` : [user?.first_name, user?.last_name].filter(Boolean).join(" ")) ||
    "Игрок";
  const avatarLetter = (user?.first_name || user?.username || "P").slice(0, 1).toUpperCase();

  return (
    <section className="screen profile-screen">
      <div className="profile-head">
        <Link className="profile-head__btn icon-button" to="/profile/settings" aria-label="Настройки">
          <SettingsIcon size={18} />
        </Link>
        <ThemeToggle theme={theme} onToggle={toggleTheme} className="profile-head__btn" />
        <Link className="profile-head__btn icon-button" to="/profile/friends" aria-label="Друзья">
          <UsersIcon size={18} />
        </Link>
      </div>

      <div className="profile-hero">
        <div className="avatar-badge">
          <AppAvatar name={fullName || avatarLetter} photoUrl={user?.photo_url} />
          <span className="avatar-badge__plus">＋</span>
        </div>
        <div className="profile-name">{fullName}</div>
      </div>

      <AppCard variant="balance" className="profile-balance">
        <div className="card__row profile-balance__top">
          <div className="profile-balance__value">
            {Number.isFinite(balance) ? `${balance.toFixed(3)} ${currency}` : `0.000 ${currency}`}
          </div>
        </div>
        <div className="profile-balance__currency-row">
          <div className="currency-selector">
            {CURRENCIES.map((c) => (
              <button
                key={c}
                type="button"
                className={`currency-selector__item ${currency === c ? "currency-selector__item--active" : ""}`}
                onClick={() => void changeCurrency(c)}
                disabled={currencySaving}
              >
                {c}
              </button>
            ))}
          </div>
        </div>
        <div className="profile-balance__actions">
          <AppButton variant="deposit" type="button">
            <DepositIcon size={18} />
            Пополнение
          </AppButton>
          <AppButton variant="withdraw" type="button">
            <WithdrawIcon size={18} />
            Вывод
          </AppButton>
        </div>
      </AppCard>

      <div className="list">
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
