import { Link } from "react-router-dom";
import { getTelegramUser } from "@/shared/lib/telegram";
import { useLanguage } from "@/shared/providers/LanguageProvider";

export function HomePage() {
  const user = getTelegramUser();
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  return (
    <section className="screen">
      <h1 className="screen__title">Durak Online</h1>
      <p className="screen__subtitle">
        {tr(
          "Telegram Mini App стартовал. Это первая рабочая версия фронтенд-каркаса.",
          "Telegram Mini App запущено. Це перша робоча версія фронтенд-каркасу.",
        )}
      </p>

      <div className="card">
        <div className="card__label">{tr("Игрок", "Гравець")}</div>
        <div className="card__value">
          {user?.username
            ? `@${user.username}`
            : tr("Не авторизован в Telegram WebApp", "Не авторизований у Telegram WebApp")}
        </div>
      </div>

      <div className="action-list">
        <Link className="button button--primary" to="/lobby">
          {tr("Перейти в лобби", "Перейти до лобі")}
        </Link>
        <Link className="button" to="/play/create">
          {tr("Создать игру", "Створити гру")}
        </Link>
      </div>
    </section>
  );
}
