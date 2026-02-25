import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { App } from "@/app/App";
import { ErrorBoundary } from "@/shared/ui/ErrorBoundary";
import { ThemeProvider } from "@/shared/providers/ThemeProvider";
import "@/styles/tokens.css";
import "@/styles/global.css";

const rootEl = document.getElementById("root");
if (!rootEl) {
  document.body.innerHTML = "<div style='padding:20px;background:#010a1b;color:#ff5555;font-family:system-ui;'>Ошибка: не найден #root</div>";
} else {
  try {
    ReactDOM.createRoot(rootEl).render(
      <React.StrictMode>
        <ErrorBoundary>
          <ThemeProvider>
            <BrowserRouter>
              <App />
            </BrowserRouter>
          </ThemeProvider>
        </ErrorBoundary>
      </React.StrictMode>,
    );
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    rootEl.innerHTML = `<div style="padding:20px;background:#010a1b;color:#ff5555;font-family:system-ui;white-space:pre-wrap;">Ошибка загрузки приложения:\n${msg}</div>`;
    console.error(err);
  }
}
