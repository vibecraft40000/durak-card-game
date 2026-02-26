import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { createDepositInvoice } from "@/shared/api/deposit";
import { createPayment } from "@/shared/api/payments";
import { bootstrapTelegramAuth, clearTokens } from "@/shared/api/auth";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { openTelegramLink } from "@/shared/lib/telegram";
import { BackIcon, CryptoBotIcon, DepositIcon, ChevronRightIcon } from "@/shared/ui/Icons";

const AMOUNTS = [5, 10, 25, 50] as const;

type DepositStep = "method" | "amount";
type DepositMethod = "crypto" | "card";

export function DepositPage() {
  const navigate = useNavigate();
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [step, setStep] = useState<DepositStep>("method");
  const [method, setMethod] = useState<DepositMethod>("crypto");
  const [amount, setAmount] = useState<number>(10);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function selectMethod(next: DepositMethod) {
    setMethod(next);
    setError(null);
    setStep("amount");
  }

  async function createCryptoPayment() {
    const { invoiceUrl } = await createDepositInvoice(amount);
    openTelegramLink(invoiceUrl);
    navigate("/profile");
  }

  async function createCardPayment() {
    const res = await createPayment(amount);
    openTelegramLink(res.directPayLink);
    navigate("/profile");
  }

  async function handleContinue() {
    if (loading) return;
    setLoading(true);
    setError(null);

    const runSelectedMethod = async () => {
      if (method === "crypto") {
        await createCryptoPayment();
      } else {
        await createCardPayment();
      }
    };

    try {
      await runSelectedMethod();
      return;
    } catch (firstErr: unknown) {
      const status = (firstErr as { status?: number })?.status;
      if (status === 401) {
        clearTokens();
        try {
          await bootstrapTelegramAuth();
          await runSelectedMethod();
          return;
        } catch {
          // ignore and show fallback text below
        }
      }
      const message =
        firstErr instanceof Error
          ? firstErr.message
          : String(firstErr ?? "");
      setError(
        message ||
          tr(
            "Не удалось создать платёж. Проверьте подключение или попробуйте позже.",
            "Не вдалося створити платіж. Перевірте з'єднання або спробуйте пізніше.",
          ),
      );
    } finally {
      setLoading(false);
    }
  }

  const methodTitle = method === "crypto" ? "Crypto Bot" : tr("Банковская карта", "Банківська картка");

  return (
    <section className="screen deposit-page">
      <div className="page-header">
        <Link className="icon-button" to="/profile" aria-label={tr("Назад", "Назад")}>
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{tr("Пополнение", "Поповнення")}</h1>
        <div className="page-header__spacer" />
      </div>

      {step === "method" && (
        <div className="deposit-content">
          <p className="deposit-hint">{tr("Выберите метод пополнения", "Оберіть метод поповнення")}</p>

          <button type="button" className="deposit-method" onClick={() => selectMethod("crypto")}>
            <div className="deposit-method__icon">
              <CryptoBotIcon size={40} />
            </div>
            <div className="deposit-method__text">
              <span className="deposit-method__name">Crypto Bot</span>
              <span className="deposit-method__desc">USDT</span>
            </div>
            <ChevronRightIcon size={20} />
          </button>

          <button type="button" className="deposit-method" onClick={() => selectMethod("card")}>
            <div className="deposit-method__icon">
              <DepositIcon size={40} />
            </div>
            <div className="deposit-method__text">
              <span className="deposit-method__name">{tr("Банковская карта", "Банківська картка")}</span>
              <span className="deposit-method__desc">Wallet Pay</span>
            </div>
            <ChevronRightIcon size={20} />
          </button>
        </div>
      )}

      {step === "amount" && (
        <div className="deposit-content">
          <button
            type="button"
            className="deposit-back"
            onClick={() => {
              setError(null);
              setStep("method");
            }}
          >
            {`< ${tr("Назад", "Назад")}`}
          </button>
          <p className="deposit-hint">{tr("Метод", "Метод")}: {methodTitle}</p>
          <p className="deposit-hint">{tr("Сумма пополнения (USD)", "Сума поповнення (USD)")}</p>
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
          <p className="deposit-min">{tr("от 1 USD", "від 1 USD")}</p>
          {error && <p className="deposit-error">{error}</p>}
          <button type="button" className="deposit-continue" onClick={handleContinue} disabled={loading}>
            {loading ? tr("Загрузка...", "Завантаження...") : tr("Продолжить", "Продовжити")}
          </button>
        </div>
      )}

      <style>{`
        .deposit-page { padding: var(--space-16); }
        .deposit-content { margin-top: var(--space-20); display: grid; gap: 10px; }
        .deposit-back {
          margin-bottom: var(--space-8);
          border: 0;
          background: none;
          color: var(--color-text-link);
          font-size: var(--font-size-body);
          cursor: pointer;
          width: fit-content;
        }
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
