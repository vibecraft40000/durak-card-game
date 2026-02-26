import { useEffect, useState } from "react";
import { subscribeSocketStore } from "@/store/socket.store";
import { useLanguage } from "@/shared/providers/LanguageProvider";

export function ReconnectOverlay() {
  const { language } = useLanguage();
  const [isReconnecting, setIsReconnecting] = useState(false);

  useEffect(() => {
    return subscribeSocketStore((state) => setIsReconnecting(state.isReconnecting));
  }, []);

  if (!isReconnecting) return null;

  return (
    <div className="reconnect-overlay" role="alert" aria-live="polite">
      <div className="reconnect-overlay__content">
        <p className="reconnect-overlay__text">
          {language === "uk" ? "З'єднання втрачено… Відновлюємо" : "Соединение потеряно… Пытаемся восстановить"}
        </p>
      </div>
    </div>
  );
}
