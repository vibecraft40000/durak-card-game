import { getTelegramUser, waitForTelegramInitData } from "@/shared/lib/telegram";
import { httpRequest } from "@/shared/api/http";

type AuthUser = {
  id: string;
  telegram_id: number;
  username: string;
  referral_code: string;
};

type TelegramAuthResponse = {
  user: AuthUser;
  accessToken: string;
  refreshToken: string;
};

type RefreshResponse = {
  accessToken: string;
};

const ACCESS_TOKEN_KEY = "durak_access_token";
const REFRESH_TOKEN_KEY = "durak_refresh_token";
const DEV_IDENTITY_KEY = "durak_dev_identity";
const DEV_AUTH_FLAG_KEY = "durak_dev_auth";
const FORCE_DEV_AUTH = String(import.meta.env.VITE_FORCE_DEV_AUTH).toLowerCase() === "true";
const IS_PROD_DOMAIN =
  typeof window !== "undefined" &&
  (window.location.origin === "https://durakonline.duckdns.org" ||
    window.location.origin === "https://www.durakonline.duckdns.org");

export function getAccessToken() {
  return localStorage.getItem(ACCESS_TOKEN_KEY);
}

export function getRefreshToken() {
  return localStorage.getItem(REFRESH_TOKEN_KEY);
}

export function setTokens(accessToken: string, refreshToken: string) {
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
}

export function clearTokens() {
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
}

export async function ensureAuthSession(signal?: AbortSignal): Promise<void> {
  if (getAccessToken()) {
    return;
  }

  const refreshed = await refreshAccessToken();
  if (refreshed) {
    return;
  }

  await bootstrapTelegramAuth(signal);
}

export async function bootstrapTelegramAuth(signal?: AbortSignal): Promise<void> {
  const opts = { method: "POST" as const, skipAuth: true as const, signal };
  const authorize = async (initData: string, dev: boolean) => {
    const response = await httpRequest<TelegramAuthResponse>("/auth/telegram", {
      ...opts,
      body: { initData },
    });
    setTokens(response.accessToken, response.refreshToken);
    if (dev) {
      localStorage.setItem(DEV_AUTH_FLAG_KEY, "true");
    } else {
      localStorage.removeItem(DEV_AUTH_FLAG_KEY);
    }
  };

  if (FORCE_DEV_AUTH) {
    await authorize(buildDevInitData(), true);
    return;
  }

  const initData = (await waitForTelegramInitData({ signal, timeoutMs: 3000 })).trim();
  if (initData) {
    try {
      await authorize(initData, false);
      return;
    } catch (error) {
      if (IS_PROD_DOMAIN) {
        throw error;
      }
    }
  }

  if (IS_PROD_DOMAIN) {
    throw new Error("Telegram initData is missing in production environment");
  }

  await authorize(buildDevInitData(), true);
}

export async function refreshAccessToken(): Promise<string | null> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) {
    return null;
  }
  try {
    const response = await httpRequest<RefreshResponse>("/auth/refresh", {
      method: "POST",
      body: { refreshToken },
      skipAuth: true,
    });
    localStorage.setItem(ACCESS_TOKEN_KEY, response.accessToken);
    return response.accessToken;
  } catch {
    clearTokens();
    return null;
  }
}

export function getCurrentTelegramUserName() {
  return getTelegramUser()?.username ?? "player";
}

function buildDevInitData(): string {
  const identity = getOrCreateDevIdentity();
  const user = encodeURIComponent(
    JSON.stringify({
      id: identity.id,
      username: identity.username,
      first_name: identity.firstName,
      last_name: identity.lastName,
    }),
  );
  return `auth_date=1999999999&user=${user}&hash=dev`;
}

function getOrCreateDevIdentity(): { id: number; username: string; firstName: string; lastName: string } {
  const saved = localStorage.getItem(DEV_IDENTITY_KEY);
  if (saved) {
    try {
      const parsed = JSON.parse(saved) as {
        id: number;
        username: string;
        firstName: string;
        lastName: string;
      };
      if (parsed.id && parsed.username) {
        return parsed;
      }
    } catch {
      // ignore corrupted local dev identity
    }
  }
  const id = 700000000 + Math.floor(Math.random() * 100000000);
  const identity = { id, username: `tg${id}`, firstName: "Dev", lastName: String(id).slice(-4) };
  localStorage.setItem(DEV_IDENTITY_KEY, JSON.stringify(identity));
  return identity;
}
