import { httpRequest } from "./http";

export type AppConfig = Record<string, never>;

export async function getConfig(): Promise<AppConfig> {
  return httpRequest<AppConfig>("/api/config", { skipAuth: true });
}
