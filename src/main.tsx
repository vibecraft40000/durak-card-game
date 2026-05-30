import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { App } from "@/app/App";
import { ErrorBoundary } from "@/shared/ui/ErrorBoundary";
import { ThemeProvider } from "@/shared/providers/ThemeProvider";
import { LanguageProvider } from "@/shared/providers/LanguageProvider";
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
            <LanguageProvider>
              <BrowserRouter>
                <App />
              </BrowserRouter>
            </LanguageProvider>
          </ThemeProvider>
        </ErrorBoundary>
      </React.StrictMode>,
    );
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    const crashEl = document.createElement("div");
    crashEl.style.padding = "20px";
    crashEl.style.background = "#010a1b";
    crashEl.style.color = "#ff5555";
    crashEl.style.fontFamily = "system-ui";
    crashEl.style.whiteSpace = "pre-wrap";
    crashEl.textContent = `Ошибка загрузки приложения:\n${msg}`;
    rootEl.replaceChildren(crashEl);
    console.error(err);
  }
}
