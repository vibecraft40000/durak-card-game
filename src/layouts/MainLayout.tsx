import { Outlet } from "react-router-dom";
import { BottomNav } from "@/shared/ui/BottomNav";

export function MainLayout() {
  return (
    <div className="app-shell">
      <main className="app-content">
        <Outlet />
      </main>

      <BottomNav />
    </div>
  );
}
