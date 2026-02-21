import { Link } from "react-router-dom";
import { BackIcon } from "@/shared/ui/Icons";

const DAYS = Array.from({ length: 28 }, (_, index) => index + 1);

export function HistoryCalendarPage() {
  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile/history/games">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">История игр</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card">
        <div className="calendar-grid">
          {DAYS.map((day) => (
            <button className={`calendar-day ${day === 18 ? "calendar-day--active" : ""}`} type="button" key={day}>
              {day}
            </button>
          ))}
        </div>
      </div>
    </section>
  );
}
