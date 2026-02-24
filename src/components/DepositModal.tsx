import { useState, useEffect } from "react";
import { createPortal } from "react-dom";
import { createDepositInvoice } from "@/shared/api/deposit";
import { createPayment } from "@/shared/api/payments";
import { openTelegramLink } from "@/shared/lib/telegram";
import { CloseIcon, CryptoBotIcon, ChevronRightIcon } from "@/shared/ui/Icons";

const AMOUNTS = [5, 10, 25, 50] as const;

type DepositStep = "method" | "amount";

type Props = {
  open: boolean;
  onClose: () => void;
};

export function DepositModal({ open, onClose }: Props) {
  const [step, setStep] = useState<DepositStep>("method");
  const [amount, setAmount] = useState<number>(10);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      document.body.style.overflow = "hidden";
    }
    return () => {
      document.body.style.overflow = "";
    };
  }, [open]);

  if (!open) return null;

  function handleChooseCryptoBot() {
    setError(null);
    setStep("amount");
  }

  function handleBack() {
    setError(null);
    setStep("method");
  }

  async function handleContinue() {
    if (loading) return;
    setLoading(true);
    setError(null);
    try {
      const { invoiceUrl } = await createDepositInvoice(amount);
      openTelegramLink(invoiceUrl);
      onClose();
    } catch {
      try {
        const res = await createPayment(amount);
        openTelegramLink(res.directPayLink);
        onClose();
      } catch {
        setError("Не удалось создать платёж. Попробуйте позже.");
      }
    } finally {
      setLoading(false);
    }
  }

  const content = (
    <div className="deposit-modal" role="dialog" aria-modal="true" aria-labelledby="deposit-modal-title">
      <div className="deposit-modal__backdrop" onClick={onClose} aria-hidden="true" />
      <div className="deposit-modal__panel">
        <div className="deposit-modal__header">
          {step === "amount" ? (
            <button type="button" className="deposit-modal__back" onClick={handleBack} aria-label="Назад">
              ← Назад
            </button>
          ) : (
            <span />
          )}
          <h2 id="deposit-modal-title" className="deposit-modal__title">
            Пополнение
          </h2>
          <button type="button" className="deposit-modal__close" onClick={onClose} aria-label="Закрыть">
            <CloseIcon size={20} />
          </button>
        </div>

        {step === "method" && (
          <div className="deposit-modal__body">
            <p className="deposit-modal__hint">Выбери метод пополнения</p>
            <button
              type="button"
              className="deposit-modal__method"
              onClick={handleChooseCryptoBot}
              aria-label="Crypto Bot"
            >
              <div className="deposit-modal__method-icon">
                <CryptoBotIcon size={40} />
              </div>
              <div className="deposit-modal__method-text">
                <span className="deposit-modal__method-name">Crypto Bot</span>
                <span className="deposit-modal__method-desc">Telegram-bot</span>
              </div>
              <ChevronRightIcon size={20} className="deposit-modal__method-chevron" />
            </button>
          </div>
        )}

        {step === "amount" && (
          <div className="deposit-modal__body">
            <p className="deposit-modal__hint">Сумма пополнения (USD)</p>
            <div className="deposit-modal__amounts">
              {AMOUNTS.map((a) => (
                <button
                  key={a}
                  type="button"
                  className={`deposit-modal__amount ${amount === a ? "deposit-modal__amount--active" : ""}`}
                  onClick={() => setAmount(a)}
                >
                  ${a}
                </button>
              ))}
            </div>
            <p className="deposit-modal__min">от 1 USD</p>
            {error && <p className="deposit-modal__error">{error}</p>}
            <button
              type="button"
              className="deposit-modal__continue"
              onClick={handleContinue}
              disabled={loading}
            >
              {loading ? "..." : "Продолжить"}
            </button>
          </div>
        )}
      </div>

      <style>{`
        .deposit-modal {
          position: fixed;
          inset: 0;
          z-index: 99999;
          display: flex;
          align-items: flex-end;
          justify-content: center;
          padding: 0 var(--space-14);
        }
        .deposit-modal__backdrop {
          position: absolute;
          inset: 0;
          background: rgba(0,0,0,0.6);
        }
        .deposit-modal__panel {
          position: relative;
          width: 100%;
          max-width: 390px;
          max-height: 85vh;
          overflow-y: auto;
          background: var(--color-surface-card);
          border-radius: var(--radius-card) var(--radius-card) 0 0;
          padding: var(--space-16);
          padding-bottom: calc(var(--space-16) + env(safe-area-inset-bottom, 0px));
        }
        .deposit-modal__header {
          display: grid;
          grid-template-columns: 1fr auto 1fr;
          align-items: center;
          margin-bottom: var(--space-20);
        }
        .deposit-modal__back {
          border: 0;
          background: none;
          color: var(--color-text-link);
          font-size: var(--font-size-body);
          cursor: pointer;
        }
        .deposit-modal__title {
          margin: 0;
          font-size: var(--font-size-title);
          font-weight: var(--font-weight-semibold);
        }
        .deposit-modal__close {
          border: 0;
          background: none;
          color: var(--color-text-secondary);
          padding: var(--space-4);
          cursor: pointer;
          margin-left: auto;
        }
        .deposit-modal__hint {
          margin: 0 0 var(--space-16);
          color: var(--color-text-secondary);
          font-size: var(--font-size-label);
        }
        .deposit-modal__method {
          display: flex;
          align-items: center;
          gap: var(--space-12);
          width: 100%;
          padding: var(--space-14);
          border: 1px solid var(--color-border);
          border-radius: var(--radius-input);
          background: var(--color-bg-primary);
          color: var(--color-text-primary);
          cursor: pointer;
        }
        .deposit-modal__method-icon {
          flex-shrink: 0;
        }
        .deposit-modal__method-text {
          flex: 1;
          text-align: left;
          display: flex;
          flex-direction: column;
          gap: 2px;
        }
        .deposit-modal__method-name {
          font-size: var(--font-size-body);
          font-weight: var(--font-weight-semibold);
        }
        .deposit-modal__method-desc {
          font-size: var(--font-size-hint);
          color: var(--color-text-secondary);
        }
        .deposit-modal__method-chevron {
          color: var(--color-text-secondary);
        }
        .deposit-modal__amounts {
          display: grid;
          grid-template-columns: repeat(4, 1fr);
          gap: var(--space-8);
          margin-bottom: var(--space-8);
        }
        .deposit-modal__amount {
          padding: var(--space-12);
          border: 1px solid var(--color-border);
          border-radius: var(--radius-input);
          background: var(--color-bg-primary);
          color: var(--color-text-primary);
          font-size: var(--font-size-body);
          font-weight: var(--font-weight-semibold);
          cursor: pointer;
        }
        .deposit-modal__amount--active {
          border-color: var(--color-accent);
          background: var(--color-btn-secondary);
        }
        .deposit-modal__min {
          margin: 0 0 var(--space-16);
          font-size: var(--font-size-hint);
          color: var(--color-text-secondary);
        }
        .deposit-modal__error {
          margin: 0 0 var(--space-12);
          font-size: var(--font-size-label);
          color: var(--color-text-error, #e74c3c);
        }
        .deposit-modal__continue {
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
        .deposit-modal__continue:disabled {
          opacity: 0.7;
        }
      `}</style>
    </div>
  );

  return createPortal(content, document.body);
}
