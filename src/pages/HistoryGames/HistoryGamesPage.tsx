import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getHistory, type HistoryItem } from "@/shared/api/history";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon } from "@/shared/ui/Icons";

export function HistoryGamesPage() {
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [items, setItems] = useState<HistoryItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void loadHistory();
  }, []);

  async function loadHistory() {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getHistory({ limit: 50 });
      setItems(response.items);
    } catch {
      setError(tr("Не удалось загрузить историю игр.", "Не вдалося завантажити історію ігор."));
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{tr("История игр", "Історія ігор")}</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="action-list action-list--inline">
        <Link className="button" to="/profile/history/date">
          {tr("По дате", "За датою")}
        </Link>
        <Link className="button" to="/profile/history/calendar">
          {tr("Календарь", "Календар")}
        </Link>
        <Link className="button" to="/play">
          {tr("Игры", "Ігри")}
        </Link>
      </div>

      <div className="list">
        {isLoading && <div className="card__hint">{tr("Загрузка...", "Завантаження...")}</div>}
        {error && <div className="card__hint card__hint--error">{error}</div>}

        {!isLoading && !error && items.length === 0 && (
          <div className="card__hint">{tr("История игр пуста.", "Історія ігор порожня.")}</div>
        )}

        {!isLoading &&
          !error &&
          items.map((item) => {
            const delta = `${item.profit >= 0 ? "+" : ""}${item.profit.toFixed(2)}`;
            const createdAt = new Date(item.createdAt).toLocaleString(language === "uk" ? "uk-UA" : "ru-RU");
            return (
              <article className="card" key={`${item.matchId}-${item.createdAt}`}>
                <div className="card__row">
                  <strong>${item.stake.toFixed(2)}</strong>
                  <span>{item.matchId.slice(0, 8)}</span>
                </div>
                <div className="card__hint">
                  {item.result === "win" ? tr("Победа", "Перемога") : tr("Поражение", "Поразка")}
                </div>
                <div className="card__row">
                  <span>{tr("Результат", "Результат")}</span>
                  <strong>{delta}</strong>
                </div>
                <div className="card__hint">{createdAt}</div>
              </article>
            );
          })}
      </div>
    </section>
  );
}
