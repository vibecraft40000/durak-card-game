import { useEffect } from "react";
import { initTelegramWebApp } from "@/shared/lib/telegram";
import { bootstrapTelegramAuth } from "@/shared/api/auth";
import { AppRoutes } from "@/app/routes";

export function App() {
  useEffect(() => {
    initTelegramWebApp();
    void bootstrapTelegramAuth();
  }, []);

  return <AppRoutes />;
}
