import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { initTelegramWebApp, getTelegramStartParam } from "@/shared/lib/telegram";
import { bootstrapTelegramAuth } from "@/shared/api/auth";
import { AppRoutes } from "@/app/routes";

export function App() {
  const [authReady, setAuthReady] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    initTelegramWebApp();
    bootstrapTelegramAuth()
      .then(() => setAuthReady(true))
      .catch(() => setAuthReady(true));
  }, []);

  useEffect(() => {
    const startParam = getTelegramStartParam();
    if (startParam?.startsWith("room_")) {
      const roomId = startParam.slice(5);
      if (roomId) navigate(`/room/${roomId}`, { replace: true });
    }
  }, [navigate]);

  if (!authReady) {
    return (
      <div style={{ display: "flex", justifyContent: "center", alignItems: "center", minHeight: "100vh", color: "var(--color-text-secondary)" }}>
        Загрузка…
      </div>
    );
  }

  return <AppRoutes />;
}
