import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { getProfileTransactions, type ProfileTransactionItem } from "@/shared/api/transactions";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon } from "@/shared/ui/Icons";

function formatAmount(value: number) {
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(2)}`;
}

export function TransactionsPage() {
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [items, setItems] = useState<ProfileTransactionItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void loadTransactions();
  }, []);

  async function loadTransactions() {
    setIsLoading(true);
    setError(null);
    try {
      const data = await getProfileTransactions(80, 0);
      setItems(data);
    } catch {
      setError(tr("Не удалось загрузить транзакции.", "Не вдалося завантажити транзакції."));
    } finally {
      setIsLoading(false);
    }
  }

  const labels = useMemo(
    () => ({
      deposit: tr("Ввод", "Внесення"),
      withdraw: tr("Вывод", "Виведення"),
      bet_hold: tr("Ставка", "Ставка"),
      win: tr("Выигрыш", "Виграш"),
      commission: tr("Комиссия", "Комісія"),
      admin_adjust: tr("Корректировка", "Коригування"),
    }),
    [language],
  );

  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{tr("История транзакций", "Історія транзакцій")}</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="list">
        {isLoading && <div className="card__hint">{tr("Загрузка...", "Завантаження...")}</div>}
        {error && <div className="card__hint card__hint--error">{error}</div>}
        {!isLoading && !error && items.length === 0 && (
          <div className="card__hint">{tr("Транзакций пока нет.", "Транзакцій поки немає.")}</div>
        )}

        {!isLoading &&
          !error &&
          items.map((item) => {
            const typeLabel = labels[item.type as keyof typeof labels] ?? item.type;
            const dateLabel = new Date(item.created_at).toLocaleString(language === "uk" ? "uk-UA" : "ru-RU");
            const extra = item.match_id
              ? `${tr("Матч", "Матч")} ${item.match_id.slice(0, 8)}`
              : `${tr("Статус", "Статус")}: ${item.status}`;
            return (
              <article className="card" key={item.id}>
                <div className="card__row">
                  <strong>{typeLabel}</strong>
                  <strong>{formatAmount(item.amount)} USD</strong>
                </div>
                <div className="card__hint">{extra}</div>
                <div className="card__hint">{dateLabel}</div>
              </article>
            );
          })}
      </div>
    </section>
  );
}
