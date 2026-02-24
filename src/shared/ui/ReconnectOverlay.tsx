import { useEffect, useState } from "react";
import { subscribeSocketStore } from "@/store/socket.store";

export function ReconnectOverlay() {
  const [isReconnecting, setIsReconnecting] = useState(false);

  useEffect(() => {
    return subscribeSocketStore((state) => setIsReconnecting(state.isReconnecting));
  }, []);

  if (!isReconnecting) return null;

  return (
    <div
      className="reconnect-overlay"
      role="alert"
      aria-live="polite"
    >
      <div className="reconnect-overlay__content">
        <p className="reconnect-overlay__text">
          Соединение потеряно… Пытаемся восстановить
        </p>
      </div>
    </div>
  );
}
