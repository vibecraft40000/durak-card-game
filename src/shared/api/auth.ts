import { getTelegramUser, waitForTelegramInitData } from "@/shared/lib/telegram";
import { httpRequest, HttpError } from "@/shared/api/http";

/** Thrown when backend returns 403 subscription_required (user must join channel before using the app). */
export class SubscriptionRequiredError extends Error {
  readonly channelLink: string;

  constructor(channelLink: string) {
    super("Subscription to the channel is required to access the app");
    this.name = "SubscriptionRequiredError";
    this.channelLink = channelLink;
  }
}

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
const IS_LOCAL_DEV_HOST =
  typeof window !== "undefined" &&
  (window.location.hostname === "localhost" ||
    window.location.hostname === "127.0.0.1" ||
    window.location.hostname === "::1");

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

function getChannelLinkFromError(err: HttpError): string {
  if (
    err.details &&
    typeof err.details === "object" &&
    "channelLink" in err.details &&
    typeof (err.details as { channelLink?: string }).channelLink === "string"
  ) {
    return (err.details as { channelLink: string }).channelLink;
  }
  return "https://t.me/+P_rbSS0y5N9jM2Qy";
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

  const authorizeWithSubscriptionCheck = async (initData: string, dev: boolean) => {
    try {
      await authorize(initData, dev);
    } catch (err) {
      if (
        err instanceof HttpError &&
        err.status === 403 &&
        err.code === "subscription_required"
      ) {
        throw new SubscriptionRequiredError(getChannelLinkFromError(err));
      }
      throw err;
    }
  };

  if (FORCE_DEV_AUTH) {
    await authorizeWithSubscriptionCheck(buildDevInitData(), true);
    return;
  }

  const initData = (await waitForTelegramInitData({ signal, timeoutMs: 3000 })).trim();
  if (initData) {
    await authorizeWithSubscriptionCheck(initData, false);
    return;
  }

  if (!IS_LOCAL_DEV_HOST) {
    throw new Error("Telegram initData is missing outside local development");
  }

  await authorizeWithSubscriptionCheck(buildDevInitData(), true);
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
