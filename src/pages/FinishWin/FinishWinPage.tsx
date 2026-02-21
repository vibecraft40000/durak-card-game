import { Link } from "react-router-dom";
import { hapticNotification } from "@/shared/lib/telegram";

export function FinishWinPage() {
  return (
    <section className="screen finish-screen">
      <div className="result-card result-card--win">
        <div className="result-card__title">Игра завершена!</div>
        <div className="result-card__message">Победа</div>
        <div className="result-card__amount result-card__amount--plus">+0</div>
        <Link className="button button--primary" to="/play" onClick={() => hapticNotification("success")}>
          Продолжить
        </Link>
      </div>
    </section>
  );
}
