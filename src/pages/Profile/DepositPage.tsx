import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { createDepositInvoice } from "@/shared/api/deposit";
import { createPayment } from "@/shared/api/payments";
import { bootstrapTelegramAuth, clearTokens } from "@/shared/api/auth";
import { openTelegramLink } from "@/shared/lib/telegram";
import { BackIcon, CryptoBotIcon, ChevronRightIcon } from "@/shared/ui/Icons";

const AMOUNTS = [5, 10, 25, 50] as const;

type DepositStep = "method" | "amount";

export function DepositPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<DepositStep>("method");
  const [amount, setAmount] = useState<number>(10);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function handleChooseCryptoBot() {
    setError(null);
    setStep("amount");
  }

  async function handleContinue() {
    if (loading) return;
    setLoading(true);
    setError(null);
    try {
      const { invoiceUrl } = await createDepositInvoice(amount);
      openTelegramLink(invoiceUrl);
      navigate("/profile");
      return;
    } catch (cryptoErr: unknown) {
      const cryptoStatus = (cryptoErr as { status?: number })?.status;
      if (cryptoStatus === 401) {
        clearTokens();
        try {
          await bootstrapTelegramAuth();
          const { invoiceUrl } = await createDepositInvoice(amount);
          openTelegramLink(invoiceUrl);
          navigate("/profile");
          return;
        } catch {
          /* fall through */
        }
      }
      try {
        const res = await createPayment(amount);
        openTelegramLink(res.directPayLink);
        navigate("/profile");
        return;
      } catch (walletErr: unknown) {
        const walletStatus = (walletErr as { status?: number })?.status;
        if (walletStatus === 401) {
          clearTokens();
          try {
            await bootstrapTelegramAuth();
            const res = await createPayment(amount);
            openTelegramLink(res.directPayLink);
            navigate("/profile");
            return;
          } catch {
            /* fall through */
          }
        }
        const cryptoMsg = cryptoErr instanceof Error ? (cryptoErr as Error).message : String(cryptoErr);
        const walletMsg = walletErr instanceof Error ? (walletErr as Error).message : String(walletErr);
        const hint =
          cryptoStatus === 401 || walletStatus === 401
            ? "Ошибка авторизации. Откройте приложение заново из Telegram."
            : cryptoMsg || walletMsg || "Не удалось создать платёж. Проверьте подключение или попробуйте позже.";
        setError(hint);
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="screen deposit-page">
      <div className="page-header">
        <Link className="icon-button" to="/profile" aria-label="Назад">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">Пополнение</h1>
        <div className="page-header__spacer" />
      </div>

      {step === "method" && (
        <div className="deposit-content">
          <p className="deposit-hint">Выбери метод пополнения</p>
          <button
            type="button"
            className="deposit-method"
            onClick={handleChooseCryptoBot}
            aria-label="Crypto Bot"
          >
            <div className="deposit-method__icon">
              <CryptoBotIcon size={40} />
            </div>
            <div className="deposit-method__text">
              <span className="deposit-method__name">Crypto Bot</span>
              <span className="deposit-method__desc">Telegram-bot</span>
            </div>
            <ChevronRightIcon size={20} />
          </button>
        </div>
      )}

      {step === "amount" && (
        <div className="deposit-content">
          <button type="button" className="deposit-back" onClick={() => { setError(null); setStep("method"); }}>
            ← Назад
          </button>
          <p className="deposit-hint">Сумма пополнения (USD)</p>
          <div className="deposit-amounts">
            {AMOUNTS.map((a) => (
              <button
                key={a}
                type="button"
                className={`deposit-amount ${amount === a ? "deposit-amount--active" : ""}`}
                onClick={() => setAmount(a)}
              >
                ${a}
              </button>
            ))}
          </div>
            <p className="deposit-min">от 1 USD</p>
          {error && <p className="deposit-error">{error}</p>}
          <button
            type="button"
            className="deposit-continue"
            onClick={handleContinue}
            disabled={loading}
          >
            {loading ? "Загрузка..." : "Продолжить"}
          </button>
        </div>
      )}

      <style>{`
        .deposit-page { padding: var(--space-16); }
        .deposit-content { margin-top: var(--space-20); }
        .deposit-back {
          margin-bottom: var(--space-16);
          border: 0;
          background: none;
          color: var(--color-text-link);
          font-size: var(--font-size-body);
          cursor: pointer;
        }
        .deposit-hint {
          margin: 0 0 var(--space-16);
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
          cursor: pointer;
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
          margin: 0 0 var(--space-16);
          font-size: var(--font-size-hint);
          color: var(--color-text-secondary);
        }
        .deposit-error {
          margin: 0 0 var(--space-12);
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
