import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { initTelegramWebApp, getTelegramStartParam } from "@/shared/lib/telegram";
import { bootstrapTelegramAuth } from "@/shared/api/auth";
import { AppRoutes } from "@/app/routes";

export function App() {
  const [authReady, setAuthReady] = useState(false);
  const [devAuth, setDevAuth] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    initTelegramWebApp();
    const controller = new AbortController();
    const timeoutId = setTimeout(() => {
      controller.abort();
      setDevAuth(localStorage.getItem("durak_dev_auth") === "true");
      setAuthReady(true);
    }, 12000);

    bootstrapTelegramAuth(controller.signal)
      .then(() => {
        setDevAuth(localStorage.getItem("durak_dev_auth") === "true");
        setAuthReady(true);
      })
      .catch((err) => {
        console.warn("[App] bootstrapTelegramAuth failed", err);
        setDevAuth(localStorage.getItem("durak_dev_auth") === "true");
        setAuthReady(true);
      })
      .finally(() => {
        clearTimeout(timeoutId);
      });

    return () => {
      clearTimeout(timeoutId);
      controller.abort();
    };
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
      <div
        style={{
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          minHeight: "100dvh",
          color: "var(--color-text-secondary, #8d8d93)",
          background: "var(--color-bg-primary, #010a1b)",
        }}
      >
        Загрузка…
      </div>
    );
  }

  return (
    <>
      <AppRoutes />
      {devAuth && (
        <div
          style={{
            position: "fixed",
            bottom: 8,
            right: 8,
            padding: "2px 6px",
            borderRadius: 4,
            background: "rgba(255, 140, 0, 0.9)",
            color: "#111",
            fontSize: 10,
            letterSpacing: 0.5,
            zIndex: 9999,
          }}
        >
          DEV AUTH
        </div>
      )}
    </>
  );
}
