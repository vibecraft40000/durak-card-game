import { Outlet } from "react-router-dom";

export function GameLayout() {
  return (
    <div className="app-shell app-shell--game">
      <main className="app-content">
        <Outlet />
      </main>
    </div>
  );
}
