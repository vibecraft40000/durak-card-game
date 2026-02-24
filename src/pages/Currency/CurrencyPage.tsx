import { Navigate } from "react-router-dom";

/** Валюта: только USD. Редирект в настройки. */
export function CurrencyPage() {
  return <Navigate to="/profile/settings" replace />;
}
