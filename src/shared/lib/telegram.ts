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

  // Telegram clients can pass initData via query or hash params.
  const fromLocation = getTelegramInitDataFromLocation();
  if (fromLocation) {
    return fromLocation;
  }

  return "";
}

export async function waitForTelegramInitData(options?: {
  signal?: AbortSignal;
  timeoutMs?: number;
  intervalMs?: number;
}): Promise<string> {
  const signal = options?.signal;
  const timeoutMs = options?.timeoutMs ?? 2500;
  const intervalMs = options?.intervalMs ?? 80;
  const deadline = Date.now() + timeoutMs;

  let initData = getTelegramInitData().trim();
  if (initData) {
    return initData;
  }

  while (Date.now() < deadline) {
    if (signal?.aborted) {
      return "";
    }
    await wait(intervalMs, signal);
    if (signal?.aborted) {
      return "";
    }
    initData = getTelegramInitData().trim();
    if (initData) {
      return initData;
    }
  }

  return "";
}

function getTelegramInitDataFromLocation() {
  if (typeof window === "undefined") {
    return "";
  }

  const fromSearch = getTelegramInitDataFromParams(window.location.search);
  if (fromSearch) {
    return fromSearch;
  }

  const hash = window.location.hash.startsWith("#") ? window.location.hash.slice(1) : window.location.hash;
  if (!hash) {
    return "";
  }

  // Handles both "#tgWebAppData=..." and hash routers like "#/route?tgWebAppData=...".
  const queryPart = hash.includes("?") ? hash.slice(hash.indexOf("?") + 1) : hash;
  return getTelegramInitDataFromParams(queryPart);
}

function getTelegramInitDataFromParams(rawParams: string) {
  if (!rawParams) {
    return "";
  }

  try {
    const params = new URLSearchParams(rawParams);
    const initDataRaw = params.get("tgWebAppData") || params.get("tg_data");
    if (initDataRaw && initDataRaw.length > 0) {
      // URLSearchParams already decodes query params.
      // Additional decodeURIComponent can corrupt "+" into spaces during backend ParseQuery.
      if (initDataRaw.includes("hash=")) {
        return initDataRaw;
      }
      try {
        const decoded = decodeURIComponent(initDataRaw);
        if (decoded.includes("hash=")) {
          return decoded;
        }
      } catch {
        // keep raw
      }
      return initDataRaw;
    }
  } catch {
    // ignore URL parsing errors
  }
  return "";
}

function wait(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const timeoutId = window.setTimeout(() => {
      resolve();
    }, ms);
    if (!signal) {
      return;
    }
    const onAbort = () => {
      window.clearTimeout(timeoutId);
      signal.removeEventListener("abort", onAbort);
      resolve();
    };
    signal.addEventListener("abort", onAbort);
  });
}

/** Start param when opened via t.me/Bot/app?startapp=... */
export function getTelegramStartParam(): string | undefined {
  return window.Telegram?.WebApp?.initDataUnsafe?.start_param;
}

export function getTelegramBotUsername(): string {
  const raw = String(import.meta.env.VITE_TELEGRAM_BOT_USERNAME ?? "").trim();
  const normalized = raw.startsWith("@") ? raw.slice(1) : raw;
  if (!normalized || normalized === "replace_with_bot_username") {
    return "durakton777_bot";
  }
  return normalized;
}

export function getTelegramMiniAppShortName(): string {
  return String(import.meta.env.VITE_TELEGRAM_MINIAPP_SHORT_NAME ?? "")
    .trim()
    .replace(/^\/+/, "");
}

export function buildTelegramMiniAppLink(startParam?: string): string {
  const botUsername = getTelegramBotUsername();
  const shortName = getTelegramMiniAppShortName();
  const base = shortName.length > 0 ? `https://t.me/${botUsername}/${shortName}` : `https://t.me/${botUsername}`;
  if (!startParam) {
    return base;
  }
  return `${base}?startapp=${encodeURIComponent(startParam)}`;
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
