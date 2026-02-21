type TelegramWebApp = {
  ready: () => void;
  expand: () => void;
  setHeaderColor?: (color: string) => void;
  setBackgroundColor?: (color: string) => void;
  colorScheme?: "light" | "dark";
  initDataUnsafe?: {
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
  return window.Telegram?.WebApp?.initData ?? "";
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
