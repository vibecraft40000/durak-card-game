import { isMockApiEnabled, mockHttpRequest } from "@/mocks/mockApi";

const RAW_API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";

// Prefer same-origin calls on production domain to avoid CORS issues.
const API_BASE_URL =
  typeof window !== "undefined" &&
  (window.location.origin === "https://durakonline.duckdns.org" ||
    window.location.origin === "https://www.durakonline.duckdns.org")
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
};

export class HttpError extends Error {
  readonly status: number;
  readonly responseBody: unknown;

  constructor(message: string, status: number, responseBody: unknown) {
    super(message);
    this.name = "HttpError";
    this.status = status;
    this.responseBody = responseBody;
  }
}

export async function httpRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, headers, signal, skipAuth = false } = options;

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
    if (response.status === 401 && !skipAuth) {
      const retryToken = await refreshToken();
      if (retryToken) {
        return httpRequest<T>(path, { ...options, headers: { ...headers, Authorization: `Bearer ${retryToken}` } });
      }
    }
    throw new HttpError(`Request failed: ${response.status}`, response.status, parsed);
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

async function refreshToken(): Promise<string | null> {
  const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);
  if (!refreshToken) {
    return null;
  }
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
    return null;
  }
  localStorage.setItem(ACCESS_TOKEN_KEY, payload.accessToken);
  return payload.accessToken;
}

function tryParseJson(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}
