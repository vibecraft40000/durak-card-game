import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { Room } from "@/entities/match/types";
import { getRooms } from "@/shared/api/rooms";

export function LobbyPage() {
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
      setError("Не удалось загрузить столы. Проверьте API и попробуйте снова.");
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <section className="screen">
      <h1 className="screen__title">Лобби</h1>
      <p className="screen__subtitle">Выберите стол для входа в матч на ставку.</p>

      {isLoading && <div className="card__hint">Загрузка столов...</div>}

      {error && (
        <div className="card">
          <div className="card__hint">{error}</div>
          <button className="button" type="button" onClick={() => void loadRooms()}>
            Повторить
          </button>
        </div>
      )}

      {!isLoading && !error && rooms.length === 0 && (
        <div className="card">
          <div className="card__hint">Активных столов пока нет. Создайте первый.</div>
          <Link className="button button--primary" to="/play/create">
            Создать игру
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
              {room.mode} · {room.players}/{room.maxPlayers} · колода {room.deck}
            </div>
            <Link className="button button--primary" to={`/room/${room.id}`}>
              Играть
            </Link>
          </article>
        ))}
      </div>
    </section>
  );
}
