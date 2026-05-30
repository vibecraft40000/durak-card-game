import { isMockApiEnabled, mockHttpRequest } from "@/mocks/mockApi";

const RAW_API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";

// Prefer same-origin calls on production domain to avoid CORS issues.
const API_BASE_URL =
  typeof window !== "undefined" &&
  (window.location.origin === "https://your-domain.example" ||
    window.location.origin === "https://www.your-domain.example")
    ? ""
    : RAW_API_BASE_URL;
const ACCESS_TOKEN_KEY = "durak_access_token";
const REFRESH_TOKEN_KEY = "durak_refresh_token";

type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

type RequestOptions = {
  method?: HttpMethod;
  body?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
  skipAuth?: boolean;
  retryAuth?: boolean;
};

type ApiErrorEnvelope = {
  code?: string;
  message?: string;
  details?: unknown;
  request_id?: string;
};

export class HttpError extends Error {
  readonly status: number;
  readonly responseBody: unknown;
  readonly payload: unknown;
  readonly code?: string;
  readonly details?: unknown;
  readonly requestId?: string;

  constructor(
    message: string,
    status: number,
    responseBody: unknown,
    payload: unknown = responseBody,
    code?: string,
    details?: unknown,
    requestId?: string,
  ) {
    super(message);
    this.name = "HttpError";
    this.status = status;
    this.responseBody = responseBody;
    this.payload = payload;
    this.code = code;
    this.details = details;
    this.requestId = requestId;
  }
}

export async function httpRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, headers, signal, skipAuth = false, retryAuth = true } = options;

  if (isMockApiEnabled()) {
    const result = await mockHttpRequest(path, method, body, signal);
    return result as T;
  }

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method,
      signal,
      headers: {
        "Content-Type": "application/json",
        ...getAuthHeader(skipAuth),
        ...headers,
      },
      body: body ? JSON.stringify(body) : undefined,
    });
  } catch (err) {
    throw err;
  }

  const raw = await response.text();
  const parsed = raw ? tryParseJson(raw) : null;

  if (!response.ok) {
    if (response.status === 401 && !skipAuth && retryAuth) {
      const retryToken = await refreshToken();
      if (retryToken) {
        return httpRequest<T>(path, {
          ...options,
          headers: { ...headers, Authorization: `Bearer ${retryToken}` },
          retryAuth: false,
        });
      }
      const reAuthed = await bootstrapAuthOnce(signal);
      if (reAuthed) {
        return httpRequest<T>(path, { ...options, retryAuth: false });
      }
    }
    const envelope = parseApiErrorEnvelope(parsed);
    const message = envelope?.message?.trim() ? envelope.message : `Request failed: ${response.status}`;
    const responseBody = envelope?.message ?? parsed;
    throw new HttpError(
      message,
      response.status,
      responseBody,
      parsed,
      envelope?.code,
      envelope?.details,
      envelope?.request_id,
    );
  }

  return parsed as T;
}

function getAuthHeader(skipAuth: boolean) {
  if (skipAuth) {
    return {} as Record<string, string>;
  }
  const token = localStorage.getItem(ACCESS_TOKEN_KEY);
  if (!token) {
    return {} as Record<string, string>;
  }
  return { Authorization: `Bearer ${token}` } as Record<string, string>;
}

let bootstrapPromise: Promise<boolean> | null = null;

async function bootstrapAuthOnce(signal?: AbortSignal): Promise<boolean> {
  if (bootstrapPromise) {
    return bootstrapPromise;
  }
  bootstrapPromise = (async () => {
    try {
      const authModule = await import("@/shared/api/auth");
      await authModule.bootstrapTelegramAuth(signal);
      return Boolean(authModule.getAccessToken());
    } catch {
      return false;
    } finally {
      bootstrapPromise = null;
    }
  })();
  return bootstrapPromise;
}

async function refreshToken(): Promise<string | null> {
  const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);
  if (!refreshToken) {
    return null;
  }
  try {
    const response = await fetch(`${API_BASE_URL}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken }),
    });
    if (!response.ok) {
      localStorage.removeItem(ACCESS_TOKEN_KEY);
      localStorage.removeItem(REFRESH_TOKEN_KEY);
      return null;
    }
    const payload = (await response.json()) as { accessToken?: string };
    if (!payload.accessToken) {
      localStorage.removeItem(ACCESS_TOKEN_KEY);
      localStorage.removeItem(REFRESH_TOKEN_KEY);
      return null;
    }
    localStorage.setItem(ACCESS_TOKEN_KEY, payload.accessToken);
    return payload.accessToken;
  } catch {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
    localStorage.removeItem(REFRESH_TOKEN_KEY);
    return null;
  }
}

function tryParseJson(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}

function parseApiErrorEnvelope(value: unknown): ApiErrorEnvelope | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const candidate = value as Record<string, unknown>;
  const code = typeof candidate.code === "string" ? candidate.code : undefined;
  const message = typeof candidate.message === "string" ? candidate.message : undefined;
  if (!code && !message) {
    return null;
  }
  return {
    code,
    message,
    details: candidate.details,
    request_id: typeof candidate.request_id === "string" ? candidate.request_id : undefined,
  };
}
