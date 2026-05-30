import { httpRequest } from "./http";

export type AppConfig = {
  depositProvider?: "cryptopay";
  depositsEnabled?: boolean;
  cryptoBotUsername: string;
  walletPayEnabled?: boolean;
  withdrawalsEnabled?: boolean;
};

export async function getConfig(): Promise<AppConfig> {
  return httpRequest<AppConfig>("/api/config", { skipAuth: true });
}
