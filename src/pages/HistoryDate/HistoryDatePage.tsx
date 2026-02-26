import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { getHistory, type HistoryItem } from "@/shared/api/history";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon } from "@/shared/ui/Icons";

function formatDateInput(value: Date) {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function toRangeStart(value: string) {
  return new Date(`${value}T00:00:00`).toISOString();
}

function toRangeEnd(value: string) {
  return new Date(`${value}T23:59:59`).toISOString();
}

export function HistoryDatePage() {
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);
  const [searchParams] = useSearchParams();

  const today = new Date();
  const weekAgo = new Date();
  weekAgo.setDate(today.getDate() - 7);

  const [fromDate, setFromDate] = useState(searchParams.get("from") ?? formatDateInput(weekAgo));
  const [toDate, setToDate] = useState(searchParams.get("to") ?? formatDateInput(today));
  const [items, setItems] = useState<HistoryItem[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void loadRange(fromDate, toDate);
  }, []);

  async function loadRange(from: string, to: string) {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getHistory({
        from: toRangeStart(from),
        to: toRangeEnd(to),
        limit: 100,
      });
      setItems(response.items);
    } catch {
      setError(tr("Не удалось загрузить историю за период.", "Не вдалося завантажити історію за період."));
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile/history/games">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{tr("История игр", "Історія ігор")}</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card form-grid">
        <label className="field">
          <span>{tr("Дата с", "Дата від")}</span>
          <input type="date" value={fromDate} onChange={(event) => setFromDate(event.target.value)} />
        </label>
        <label className="field">
          <span>{tr("Дата по", "Дата до")}</span>
          <input type="date" value={toDate} onChange={(event) => setToDate(event.target.value)} />
        </label>
        <button className="button button--primary" type="button" onClick={() => void loadRange(fromDate, toDate)}>
          {tr("Применить", "Застосувати")}
        </button>
      </div>

      <div className="list">
        {isLoading && <div className="card__hint">{tr("Загрузка...", "Завантаження...")}</div>}
        {error && <div className="card__hint card__hint--error">{error}</div>}
        {!isLoading && !error && items.length === 0 && (
          <div className="card__hint">{tr("За выбранный период игр нет.", "За обраний період ігор немає.")}</div>
        )}
        {!isLoading &&
          !error &&
          items.map((item) => {
            const delta = `${item.profit >= 0 ? "+" : ""}${item.profit.toFixed(2)}`;
            return (
              <article className="card" key={`${item.matchId}-${item.createdAt}`}>
                <div className="card__row">
                  <strong>${item.stake.toFixed(2)}</strong>
                  <span>{new Date(item.createdAt).toLocaleDateString(language === "uk" ? "uk-UA" : "ru-RU")}</span>
                </div>
                <div className="card__row">
                  <span>{item.result === "win" ? tr("Победа", "Перемога") : tr("Поражение", "Поразка")}</span>
                  <strong>{delta}</strong>
                </div>
              </article>
            );
          })}
      </div>
    </section>
  );
}
