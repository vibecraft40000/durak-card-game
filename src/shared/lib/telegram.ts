type TelegramWebApp = {
  ready: () => void;
  expand: () => void;
  setHeaderColor?: (color: string) => void;
  setBackgroundColor?: (color: string) => void;
  colorScheme?: "light" | "dark";
  initDataUnsafe?: {
    start_param?: string;
    user?: {
      id: number;
      username?: string;
      first_name?: string;
      last_name?: string;
      photo_url?: string;
      language_code?: string;
    };
  };
  initData?: string;
};

declare global {
  interface Window {
    Telegram?: {
      WebApp?: TelegramWebApp;
    };
  }
}

export function initTelegramWebApp() {
  const webApp = window.Telegram?.WebApp;
  if (!webApp) {
    return;
  }

  webApp.ready();
  webApp.expand();
  webApp.setHeaderColor?.("#050b2a");
  webApp.setBackgroundColor?.("#050b2a");
  applySafeAreaInsets(webApp);
}

function applySafeAreaInsets(webApp: TelegramWebApp) {
  const insets = (webApp as { safeAreaInsets?: { top?: number; bottom?: number; left?: number; right?: number } }).safeAreaInsets;
  if (!insets) return;
  const doc = document.documentElement.style;
  if (insets.top != null) doc.setProperty("--tg-safe-area-top", `${insets.top}px`);
  if (insets.bottom != null) doc.setProperty("--tg-safe-area-bottom", `${insets.bottom}px`);
  if (insets.left != null) doc.setProperty("--tg-safe-area-left", `${insets.left}px`);
  if (insets.right != null) doc.setProperty("--tg-safe-area-right", `${insets.right}px`);
}

export function getTelegramUser() {
  return window.Telegram?.WebApp?.initDataUnsafe?.user;
}

export function getTelegramInitData() {
  const direct = window.Telegram?.WebApp?.initData;
  if (direct && direct.length > 0) {
    return direct;
  }

  // Telegram Web / Desktop can pass initData via tgWebAppData query parameter.
  try {
    const params = new URLSearchParams(window.location.search);
    const raw = params.get("tgWebAppData") || params.get("tg_data");
    if (raw && raw.length > 0) {
      try {
        return decodeURIComponent(raw);
      } catch {
        return raw;
      }
    }
  } catch {
    // ignore URL parsing errors
  }

  return "";
}

/** Start param when opened via t.me/Bot/app?startapp=... */
export function getTelegramStartParam(): string | undefined {
  return window.Telegram?.WebApp?.initDataUnsafe?.start_param;
}

/** Opens a t.me link in the Telegram app. */
export function openTelegramLink(url: string) {
  const fn = (window.Telegram?.WebApp as { openTelegramLink?: (url: string) => void })?.openTelegramLink;
  if (fn) {
    fn(url);
  } else if (url.startsWith("https://t.me/")) {
    window.open(url, "_blank");
  } else {
    window.open(url, "_blank");
  }
}

type HapticType = "light" | "medium" | "heavy" | "rigid" | "soft";

export function hapticImpact(style: HapticType = "light") {
  const fb = (window.Telegram?.WebApp as { HapticFeedback?: { impactOccurred: (s: HapticType) => void } })?.HapticFeedback;
  fb?.impactOccurred?.(style);
}

export function hapticSelection() {
  (window.Telegram?.WebApp as { HapticFeedback?: { selectionChanged: () => void } })?.HapticFeedback?.selectionChanged?.();
}

export function hapticNotification(type: "error" | "success" | "warning") {
  (window.Telegram?.WebApp as { HapticFeedback?: { notificationOccurred: (t: "error" | "success" | "warning") => void } })
    ?.HapticFeedback?.notificationOccurred?.(type);
}
