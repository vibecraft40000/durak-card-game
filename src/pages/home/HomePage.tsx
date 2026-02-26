import { Link } from "react-router-dom";
import { getTelegramUser } from "@/shared/lib/telegram";

export function HomePage() {
  const user = getTelegramUser();

  return (
    <section className="screen">
      <h1 className="screen__title">Durak Online</h1>
      <p className="screen__subtitle">
        Telegram Mini App стартовал. Это первая рабочая версия фронтенд-каркаса.
      </p>

      <div className="card">
        <div className="card__label">Игрок</div>
        <div className="card__value">
          {user?.username ? `@${user.username}` : "Не авторизован в Telegram WebApp"}
        </div>
      </div>

      <div className="action-list">
        <Link className="button button--primary" to="/lobby">
          Перейти в лобби
        </Link>
        <Link className="button" to="/play/create">
          Создать игру
        </Link>
      </div>
    </section>
  );
}
