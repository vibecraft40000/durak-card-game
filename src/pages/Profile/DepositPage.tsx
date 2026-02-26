import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { createDepositInvoice } from "@/shared/api/deposit";
import { bootstrapTelegramAuth, clearTokens } from "@/shared/api/auth";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { openTelegramLink } from "@/shared/lib/telegram";
import { BackIcon, CryptoBotIcon } from "@/shared/ui/Icons";

const AMOUNTS = [5, 10, 25, 50] as const;

export function DepositPage() {
  const navigate = useNavigate();
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [amount, setAmount] = useState<number>(10);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function createCryptoPayment() {
    const { invoiceUrl } = await createDepositInvoice(amount);
    openTelegramLink(invoiceUrl);
    navigate("/profile");
  }

  async function handleContinue() {
    if (loading) {
      return;
    }
    setLoading(true);
    setError(null);

    try {
      await createCryptoPayment();
      return;
    } catch (firstErr: unknown) {
      const status = (firstErr as { status?: number })?.status;
      if (status === 401) {
        clearTokens();
        try {
          await bootstrapTelegramAuth();
          await createCryptoPayment();
          return;
        } catch {
          // fall through and show message below
        }
      }
      const message = firstErr instanceof Error ? firstErr.message : String(firstErr ?? "");
      setError(message || tr("Не удалось создать платеж. Проверьте подключение и попробуйте снова.", "Не вдалося створити платіж. Перевірте з'єднання і спробуйте знову."));
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="screen deposit-page">
      <div className="page-header">
        <Link className="icon-button" to="/profile" aria-label={tr("Назад", "Назад")}>
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{tr("Пополнение", "Поповнення")}</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="deposit-content">
        <button type="button" className="deposit-method" disabled>
          <div className="deposit-method__icon">
            <CryptoBotIcon size={40} />
          </div>
          <div className="deposit-method__text">
            <span className="deposit-method__name">Crypto Bot</span>
            <span className="deposit-method__desc">USDT</span>
          </div>
        </button>

        <p className="deposit-hint">
          {tr("Пополнение выполняется через @CryptoBot в USD.", "Поповнення виконується через @CryptoBot у USD.")}
        </p>

        <p className="deposit-hint">{tr("Сумма пополнения (USD)", "Сума поповнення (USD)")}</p>

        <div className="deposit-amounts">
          {AMOUNTS.map((value) => (
            <button
              key={value}
              type="button"
              className={`deposit-amount ${amount === value ? "deposit-amount--active" : ""}`}
              onClick={() => setAmount(value)}
            >
              ${value}
            </button>
          ))}
        </div>

        <p className="deposit-min">{tr("Минимум: 1 USD", "Мінімум: 1 USD")}</p>

        {error && <p className="deposit-error">{error}</p>}

        <button type="button" className="deposit-continue" onClick={handleContinue} disabled={loading}>
          {loading ? tr("Создаем...", "Створюємо...") : tr("Продолжить", "Продовжити")}
        </button>
      </div>

      <style>{`
        .deposit-page { padding: var(--space-16); }
        .deposit-content { margin-top: var(--space-20); display: grid; gap: 10px; }
        .deposit-hint {
          margin: 0;
          color: var(--color-text-secondary);
          font-size: var(--font-size-label);
        }
        .deposit-method {
          display: flex;
          align-items: center;
          gap: var(--space-12);
          width: 100%;
          padding: var(--space-14);
          border: 1px solid var(--color-border);
          border-radius: var(--radius-input);
          background: var(--color-surface-card);
          color: var(--color-text-primary);
          text-align: left;
        }
        .deposit-method__icon { flex-shrink: 0; }
        .deposit-method__text {
          flex: 1;
          display: flex;
          flex-direction: column;
          gap: 2px;
        }
        .deposit-method__name {
          font-size: var(--font-size-body);
          font-weight: var(--font-weight-semibold);
        }
        .deposit-method__desc {
          font-size: var(--font-size-hint);
          color: var(--color-text-secondary);
        }
        .deposit-amounts {
          display: grid;
          grid-template-columns: repeat(4, 1fr);
          gap: var(--space-8);
          margin-bottom: var(--space-8);
        }
        .deposit-amount {
          padding: var(--space-12);
          border: 1px solid var(--color-border);
          border-radius: var(--radius-input);
          background: var(--color-surface-card);
          color: var(--color-text-primary);
          font-size: var(--font-size-body);
          font-weight: var(--font-weight-semibold);
          cursor: pointer;
        }
        .deposit-amount--active {
          border-color: var(--color-accent);
          background: var(--color-btn-secondary);
        }
        .deposit-min {
          margin: 0 0 var(--space-8);
          font-size: var(--font-size-hint);
          color: var(--color-text-secondary);
        }
        .deposit-error {
          margin: 0;
          font-size: var(--font-size-label);
          color: var(--color-text-error, #e74c3c);
        }
        .deposit-continue {
          width: 100%;
          padding: var(--space-14);
          border: 0;
          border-radius: var(--radius-btn);
          background: var(--color-btn-deposit);
          color: white;
          font-size: var(--font-size-body);
          font-weight: var(--font-weight-semibold);
          cursor: pointer;
        }
        .deposit-continue:disabled { opacity: 0.7; }
      `}</style>
    </section>
  );
}
