import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { Room } from "@/entities/match/types";
import { getRooms } from "@/shared/api/rooms";
import { useLanguage } from "@/shared/providers/LanguageProvider";

export function LobbyPage() {
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [rooms, setRooms] = useState<Room[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    void loadRooms(controller.signal);
    return () => controller.abort();
  }, []);

  async function loadRooms(signal?: AbortSignal) {
    setIsLoading(true);
    setError(null);
    try {
      const data = await getRooms(signal);
      setRooms(data);
    } catch {
      setError(tr("Не удалось загрузить столы. Проверьте API и попробуйте снова.", "Не вдалося завантажити столи. Перевірте API і спробуйте ще раз."));
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <section className="screen">
      <h1 className="screen__title">{tr("Лобби", "Лобі")}</h1>
      <p className="screen__subtitle">{tr("Выберите стол для входа в матч на ставку.", "Оберіть стіл для входу в матч на ставку.")}</p>

      {isLoading && <div className="card__hint">{tr("Загрузка столов...", "Завантаження столів...")}</div>}

      {error && (
        <div className="card">
          <div className="card__hint">{error}</div>
          <button className="button" type="button" onClick={() => void loadRooms()}>
            {tr("Повторить", "Повторити")}
          </button>
        </div>
      )}

      {!isLoading && !error && rooms.length === 0 && (
        <div className="card">
          <div className="card__hint">{tr("Активных столов пока нет. Создайте первый.", "Активних столів поки немає. Створіть перший.")}</div>
          <Link className="button button--primary" to="/play/create">
            {tr("Создать игру", "Створити гру")}
          </Link>
        </div>
      )}

      <div className="list">
        {rooms.map((room) => (
          <article className="card" key={room.id}>
            <div className="card__row">
              <strong>{room.title}</strong>
              <span>${room.stakeUsd}</span>
            </div>
            <div className="card__hint">
              {room.mode} · {room.players}/{room.maxPlayers} · {tr("колода", "колода")} {room.deck}
            </div>
            <Link className="button button--primary" to={`/room/${room.id}`}>
              {tr("Играть", "Грати")}
            </Link>
          </article>
        ))}
      </div>
    </section>
  );
}
