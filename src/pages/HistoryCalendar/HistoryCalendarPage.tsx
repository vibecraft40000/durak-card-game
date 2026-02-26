import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { getHistoryCalendar, type HistoryCalendarDay } from "@/shared/api/history";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon } from "@/shared/ui/Icons";

function monthKey(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  return `${year}-${month}`;
}

function shiftMonth(base: Date, delta: number) {
  return new Date(base.getFullYear(), base.getMonth() + delta, 1);
}

function daysInMonth(date: Date) {
  return new Date(date.getFullYear(), date.getMonth() + 1, 0).getDate();
}

function dateString(date: Date, day: number) {
  const month = String(date.getMonth() + 1).padStart(2, "0");
  return `${date.getFullYear()}-${month}-${String(day).padStart(2, "0")}`;
}

export function HistoryCalendarPage() {
  const navigate = useNavigate();
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [activeMonth, setActiveMonth] = useState(() => new Date(new Date().getFullYear(), new Date().getMonth(), 1));
  const [items, setItems] = useState<HistoryCalendarDay[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void loadMonth(activeMonth);
  }, [activeMonth]);

  async function loadMonth(date: Date) {
    setIsLoading(true);
    setError(null);
    try {
      const days = await getHistoryCalendar(monthKey(date));
      setItems(days);
    } catch {
      setError(tr("Не удалось загрузить календарь истории.", "Не вдалося завантажити календар історії."));
    } finally {
      setIsLoading(false);
    }
  }

  const dayMap = useMemo(() => {
    const map = new Map<string, HistoryCalendarDay>();
    for (const item of items) {
      map.set(item.date, item);
    }
    return map;
  }, [items]);

  const monthTitle = useMemo(
    () =>
      activeMonth.toLocaleDateString(language === "uk" ? "uk-UA" : "ru-RU", {
        month: "long",
        year: "numeric",
      }),
    [activeMonth, language],
  );

  const firstWeekday = useMemo(() => {
    const value = new Date(activeMonth.getFullYear(), activeMonth.getMonth(), 1).getDay();
    return value === 0 ? 6 : value - 1;
  }, [activeMonth]);

  const days = daysInMonth(activeMonth);
  const cells = useMemo(() => {
    const result: Array<{ day: number | null; date?: string }> = [];
    for (let i = 0; i < firstWeekday; i += 1) {
      result.push({ day: null });
    }
    for (let day = 1; day <= days; day += 1) {
      result.push({ day, date: dateString(activeMonth, day) });
    }
    return result;
  }, [activeMonth, days, firstWeekday]);

  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile/history/games">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{tr("История игр", "Історія ігор")}</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card card--compact card__row">
        <button type="button" className="button" onClick={() => setActiveMonth((prev) => shiftMonth(prev, -1))}>
          {tr("< Назад", "< Назад")}
        </button>
        <strong>{monthTitle}</strong>
        <button type="button" className="button" onClick={() => setActiveMonth((prev) => shiftMonth(prev, 1))}>
          {tr("Вперёд >", "Вперед >")}
        </button>
      </div>

      {isLoading && <div className="card__hint">{tr("Загрузка...", "Завантаження...")}</div>}
      {error && <div className="card__hint card__hint--error">{error}</div>}

      <div className="card">
        <div className="calendar-grid">
          {cells.map((cell, index) => {
            if (cell.day == null || !cell.date) {
              return <div key={`empty-${index}`} />;
            }
            const dayStats = dayMap.get(cell.date);
            const hasGames = Boolean(dayStats && dayStats.games > 0);
            return (
              <button
                className={`calendar-day ${hasGames ? "calendar-day--active" : ""}`}
                type="button"
                key={cell.date}
                onClick={() => navigate(`/profile/history/date?from=${cell.date}&to=${cell.date}`)}
                title={
                  dayStats
                    ? tr(
                        `Игр: ${dayStats.games}, результат: ${dayStats.profit.toFixed(2)}`,
                        `Ігор: ${dayStats.games}, результат: ${dayStats.profit.toFixed(2)}`,
                      )
                    : tr("Игр нет", "Ігор немає")
                }
              >
                <span>{cell.day}</span>
              </button>
            );
          })}
        </div>
      </div>
    </section>
  );
}
