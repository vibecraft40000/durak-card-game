import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { initTelegramWebApp, getTelegramStartParam } from "@/shared/lib/telegram";
import { ensureAuthSession } from "@/shared/api/auth";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { AppRoutes } from "@/app/routes";

export function App() {
  const [authReady, setAuthReady] = useState(false);
  const [devAuth, setDevAuth] = useState(false);
  const navigate = useNavigate();
  const { syncLanguageFromProfile, t } = useLanguage();

  useEffect(() => {
    initTelegramWebApp();

    const controller = new AbortController();
    const timeoutId = setTimeout(() => {
      setDevAuth(localStorage.getItem("durak_dev_auth") === "true");
      setAuthReady(true);
    }, 12000);

    ensureAuthSession(controller.signal)
      .then(() => {
        setDevAuth(localStorage.getItem("durak_dev_auth") === "true");
        setAuthReady(true);
      })
      .catch((err) => {
        console.warn("[App] ensureAuthSession failed", err);
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

  useEffect(() => {
    if (!authReady) {
      return;
    }
    void syncLanguageFromProfile();
  }, [authReady, syncLanguageFromProfile]);

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
        {t("app.loading")}
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
