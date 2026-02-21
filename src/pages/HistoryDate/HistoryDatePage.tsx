import { Link } from "react-router-dom";
import { BackIcon } from "@/shared/ui/Icons";

export function HistoryDatePage() {
  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile/history/games">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">История игр</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card form-grid">
        <label className="field">
          <span>Дата с</span>
          <input type="date" defaultValue="2026-02-01" />
        </label>
        <label className="field">
          <span>Дата по</span>
          <input type="date" defaultValue="2026-02-18" />
        </label>
        <button className="button button--primary" type="button">
          Применить
        </button>
      </div>
    </section>
  );
}
