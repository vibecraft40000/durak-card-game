import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { getConfig } from "@/shared/api/config";
import { getProfile } from "@/shared/api/user";
import { createWithdraw } from "@/shared/api/withdraw";
import { HttpError } from "@/shared/api/http";
import { bootstrapTelegramAuth, clearTokens } from "@/shared/api/auth";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon, CryptoBotIcon } from "@/shared/ui/Icons";
import { AppCard } from "@/shared/ui/Card";
import { openTelegramLink } from "@/shared/lib/telegram";

const DEFAULT_CRYPTO_BOT = "CryptoBot";
const MIN_AMOUNT = 5;

export function WithdrawPage() {
  const navigate = useNavigate();
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [cryptoBotUsername, setCryptoBotUsername] = useState(DEFAULT_CRYPTO_BOT);
  const [balance, setBalance] = useState<number | null>(null);
  const [amountInput, setAmountInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const amount = parseFloat(amountInput.replace(",", ".")) || 0;
  const maxAvailable = balance != null ? Math.max(0, balance) : null;
  const exceedsBalance = maxAvailable != null && amount > maxAvailable;
  const canSubmit = amount >= MIN_AMOUNT && !exceedsBalance && !loading;

  useEffect(() => {
    getConfig()
      .then((cfg) => setCryptoBotUsername(cfg.cryptoBotUsername ?? DEFAULT_CRYPTO_BOT))
      .catch(() => undefined);
    getProfile()
      .then((r) => setBalance(r.balance))
      .catch(() => setBalance(0));
  }, []);

  async function handleWithdraw() {
    if (!canSubmit) return;
    setLoading(true);
    setError(null);
    try {
      await createWithdraw(amount);
      navigate("/profile");
      return;
    } catch (err: unknown) {
      const status = (err as { status?: number })?.status;
      if (status === 401) {
        clearTokens();
        try {
          await bootstrapTelegramAuth();
          await createWithdraw(amount);
          navigate("/profile");
          return;
        } catch {
          // fall through
        }
      }
      const msg =
        err instanceof HttpError && typeof err.responseBody === "string"
          ? err.responseBody
          : err instanceof Error
            ? err.message
            : String(err);
      setError(msg || tr("Не удалось выполнить вывод. Попробуйте позже.", "Не вдалося виконати виведення. Спробуйте пізніше."));
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="screen withdraw-page">
      <div className="page-header">
        <Link className="icon-button" to="/profile" aria-label={tr("Назад", "Назад")}>
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{tr("Вывод", "Виведення")}</h1>
        <div className="page-header__spacer" />
      </div>

      <AppCard>
        <div className="withdraw-method">
          <div className="withdraw-method__icon">
            <CryptoBotIcon size={40} />
          </div>
          <div className="withdraw-method__text">
            <span className="withdraw-method__name">Crypto Bot</span>
            <span className="withdraw-method__desc">{tr("Вывод через", "Виведення через")} @{cryptoBotUsername}</span>
          </div>
        </div>
        <p className="withdraw-hint">
          {tr(
            `После отправки заявки администратору приходит уведомление о выводе. Сумма указывается в USD, выплата в USDT через @${cryptoBotUsername}. При необходимости администратор свяжется с вами в Telegram и подтвердит вывод.`,
            `Після відправлення заявки адміністратор отримує сповіщення про виведення. Сума вказується в USD, виплата в USDT через @${cryptoBotUsername}. За потреби адміністратор зв'яжеться з вами в Telegram і підтвердить виведення.`,
          )}
        </p>
        <p className="withdraw-label">{tr("Сумма вывода (USD)", "Сума виведення (USD)")}</p>
        <input
          type="number"
          inputMode="decimal"
          min={MIN_AMOUNT}
          step={0.01}
          placeholder="0"
          value={amountInput}
          onChange={(e) => setAmountInput(e.target.value)}
          className={`withdraw-input ${exceedsBalance ? "withdraw-input--error" : ""}`}
          aria-describedby={exceedsBalance ? "withdraw-max-hint" : undefined}
        />
        {maxAvailable != null && (
          <p id="withdraw-max-hint" className={`withdraw-max ${exceedsBalance ? "withdraw-max--error" : ""}`}>
            {exceedsBalance
              ? tr(
                  `Максимально доступная сумма для вывода: ${maxAvailable.toFixed(2)} USD`,
                  `Максимально доступна сума для виведення: ${maxAvailable.toFixed(2)} USD`,
                )
              : tr(`Доступно: ${maxAvailable.toFixed(2)} USD`, `Доступно: ${maxAvailable.toFixed(2)} USD`)}
          </p>
        )}
        <p className="withdraw-min">{tr("Минимум: 5 USD (требование Crypto Pay)", "Мінімум: 5 USD (вимога Crypto Pay)")}</p>
        {error && (
          <>
            <p className="withdraw-error">{error}</p>
            {cryptoBotUsername === "CryptoTestnetBot" && (
              <button
                type="button"
                className="withdraw-open-bot"
                onClick={() => openTelegramLink("https://t.me/CryptoTestnetBot")}
              >
                {tr("Открыть", "Відкрити")} @CryptoTestnetBot
              </button>
            )}
          </>
        )}
        <button
          type="button"
          className="button button--primary withdraw-submit"
          onClick={handleWithdraw}
          disabled={!canSubmit}
        >
          {loading
            ? tr("Обработка...", "Обробка...")
            : amount > 0
              ? tr(`Вывести $${amount.toFixed(2)}`, `Вивести $${amount.toFixed(2)}`)
              : tr("Вывести", "Вивести")}
        </button>
      </AppCard>

      <style>{`
        .withdraw-page { padding: var(--space-16); }
        .withdraw-method {
          display: flex;
          align-items: center;
          gap: var(--space-12);
          margin-bottom: var(--space-16);
        }
        .withdraw-method__icon { flex-shrink: 0; }
        .withdraw-method__text {
          display: flex;
          flex-direction: column;
          gap: 2px;
        }
        .withdraw-method__name {
          font-size: var(--font-size-body);
          font-weight: var(--font-weight-semibold);
        }
        .withdraw-method__desc {
          font-size: var(--font-size-hint);
          color: var(--color-text-secondary);
        }
        .withdraw-hint {
          margin: 0 0 var(--space-16);
          font-size: var(--font-size-hint);
          color: var(--color-text-secondary);
          line-height: 1.4;
        }
        .withdraw-label {
          margin: 0 0 var(--space-8);
          font-size: var(--font-size-label);
          color: var(--color-text-secondary);
        }
        .withdraw-input {
          width: 100%;
          padding: var(--space-14);
          margin-bottom: var(--space-8);
          border: 1px solid var(--color-border);
          border-radius: var(--radius-input);
          background: var(--color-surface-card);
          color: var(--color-text-primary);
          font-size: var(--font-size-body);
          font-weight: var(--font-weight-semibold);
        }
        .withdraw-input:focus {
          outline: none;
          border-color: var(--color-accent);
        }
        .withdraw-input--error {
          border-color: var(--color-text-error, #e74c3c);
        }
        .withdraw-max {
          margin: 0 0 var(--space-8);
          font-size: var(--font-size-hint);
          color: var(--color-text-secondary);
        }
        .withdraw-max--error {
          color: var(--color-text-error, #e74c3c);
        }
        .withdraw-min {
          margin: 0 0 var(--space-16);
          font-size: var(--font-size-hint);
          color: var(--color-text-secondary);
        }
        .withdraw-error {
          margin: 0 0 var(--space-8);
          font-size: var(--font-size-label);
          color: var(--color-text-error, #e74c3c);
        }
        .withdraw-open-bot {
          margin-bottom: var(--space-12);
          padding: var(--space-10) var(--space-14);
          border: 1px solid var(--color-accent);
          border-radius: var(--radius-btn);
          background: transparent;
          color: var(--color-accent);
          font-size: var(--font-size-body);
          cursor: pointer;
        }
        .withdraw-submit { width: 100%; }
        .withdraw-submit:disabled { opacity: 0.7; }
      `}</style>
    </section>
  );
}
