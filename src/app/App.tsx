import { Suspense, lazy, useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { initTelegramWebApp, getTelegramStartParam } from "@/shared/lib/telegram";
import { SubscriptionRequiredError } from "@/shared/api/auth";
import { ensureAuthSessionLazy } from "@/shared/api/auth.lazy";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { SubscriptionGate } from "@/shared/ui/SubscriptionGate";

const AppRoutes = lazy(async () => {
  const module = await import("@/app/routes");
  return { default: module.AppRoutes };
});

export function App() {
  const [authReady, setAuthReady] = useState(false);
  const [devAuth, setDevAuth] = useState(false);
  const [subscriptionRequired, setSubscriptionRequired] = useState<{ channelLink: string } | null>(null);
  const [isRetryingSubscription, setIsRetryingSubscription] = useState(false);
  const navigate = useNavigate();
  const { syncLanguageFromProfile, t } = useLanguage();

  useEffect(() => {
    initTelegramWebApp();

    const controller = new AbortController();
    const timeoutId = setTimeout(() => {
      setDevAuth(localStorage.getItem("durak_dev_auth") === "true");
      setAuthReady(true);
    }, 12000);

    ensureAuthSessionLazy(controller.signal)
      .then(() => {
        setDevAuth(localStorage.getItem("durak_dev_auth") === "true");
        setAuthReady(true);
        setSubscriptionRequired(null);
      })
      .catch((err) => {
        if (err instanceof SubscriptionRequiredError) {
          setSubscriptionRequired({ channelLink: err.channelLink });
          setAuthReady(true);
          return;
        }
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

  const retrySubscriptionCheck = useCallback(async () => {
    if (isRetryingSubscription) return;
    setIsRetryingSubscription(true);
    try {
      const auth = await import("@/shared/api/auth");
      auth.clearTokens();
      await auth.bootstrapTelegramAuth();
      setSubscriptionRequired(null);
      setAuthReady(true);
    } catch (err) {
      if (err instanceof SubscriptionRequiredError) {
        setSubscriptionRequired({ channelLink: err.channelLink });
      }
    } finally {
      setIsRetryingSubscription(false);
    }
  }, [isRetryingSubscription]);

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

  const loadingScreen = (
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

  if (!authReady) {
    return loadingScreen;
  }

  if (subscriptionRequired) {
    return (
      <SubscriptionGate
        channelLink={subscriptionRequired.channelLink}
        onRetry={retrySubscriptionCheck}
        isRetrying={isRetryingSubscription}
      />
    );
  }

  return (
    <>
      <Suspense fallback={loadingScreen}>
        <AppRoutes />
      </Suspense>
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
