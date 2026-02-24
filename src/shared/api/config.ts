import { httpRequest } from "./http";

export type AppConfig = {
  cryptoBotUsername: string;
};

export async function getConfig(): Promise<AppConfig> {
  return httpRequest<AppConfig>("/api/config", { skipAuth: true });
}
