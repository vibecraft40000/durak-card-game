import { Link } from "react-router-dom";
import { hapticNotification } from "@/shared/lib/telegram";

export function FinishLosePage() {
  return (
    <section className="screen finish-screen">
      <div className="result-card result-card--lose">
        <div className="result-card__title">Игра завершена!</div>
        <div className="result-card__message">Поражение</div>
        <div className="result-card__amount result-card__amount--minus">-0</div>
        <Link className="button button--primary" to="/play" onClick={() => hapticNotification("warning")}>
          Продолжить
        </Link>
      </div>
    </section>
  );
}
