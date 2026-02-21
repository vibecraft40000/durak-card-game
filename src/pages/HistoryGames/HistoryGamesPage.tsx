import { Link } from "react-router-dom";
import { BackIcon } from "@/shared/ui/Icons";

const HISTORY_ROWS = [
  { id: "1", stake: 500, players: "4p", deck: 24, status: "win", delta: "+250" },
  { id: "2", stake: 300, players: "3p", deck: 36, status: "lose", delta: "-150" },
  { id: "3", stake: 100, players: "2p", deck: 52, status: "win", delta: "+47" },
];

export function HistoryGamesPage() {
  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">История игр</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="action-list action-list--inline">
        <Link className="button" to="/profile/history/date">
          По дате
        </Link>
        <Link className="button" to="/profile/history/calendar">
          Календарь
        </Link>
        <Link className="button" to="/play">
          Играть
        </Link>
      </div>

      <div className="list">
        {HISTORY_ROWS.map((row) => (
          <article className="card" key={row.id}>
            <div className="card__row">
              <strong>${row.stake}</strong>
              <span>{row.players}</span>
            </div>
            <div className="card__hint">
              {row.deck} карт · {row.status === "win" ? "Победа" : "Поражение"}
            </div>
            <div className="card__row">
              <span>Результат</span>
              <strong>{row.delta}</strong>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}
