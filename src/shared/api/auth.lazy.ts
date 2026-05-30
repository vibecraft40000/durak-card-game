type AuthModule = typeof import("./auth");

const ACCESS_TOKEN_KEY = "durak_access_token";

async function loadAuthModule(): Promise<AuthModule> {
  return import("./auth");
}

export async function ensureAuthSessionLazy(signal?: AbortSignal): Promise<void> {
  if (localStorage.getItem(ACCESS_TOKEN_KEY)) {
    return;
  }

  const auth = await loadAuthModule();
  await auth.ensureAuthSession(signal);
}

export async function resetAndBootstrapTelegramAuth(signal?: AbortSignal): Promise<void> {
  const auth = await loadAuthModule();
  auth.clearTokens();
  await auth.bootstrapTelegramAuth(signal);
}
