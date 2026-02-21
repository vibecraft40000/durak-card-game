import { httpRequest } from "@/shared/api/http";

export type UserProfile = {
  id: string;
  telegram_id: number;
  username: string;
  first_name: string;
  last_name: string;
  photo_url: string;
  display_name: string;
  currency: "USD" | "RUB" | "UAH";
};

type ProfileResponse = {
  user: UserProfile;
  balance: number;
};

type SettingsResponse = {
  user: UserProfile;
  settings: {
    displayName: string;
    currency: "USD" | "RUB" | "UAH";
  };
};

export async function getProfile(): Promise<ProfileResponse> {
  return httpRequest<ProfileResponse>("/api/profile");
}

export async function getUserSettings(): Promise<SettingsResponse> {
  return httpRequest<SettingsResponse>("/api/user/settings");
}

export async function patchUserSettings(input: {
  displayName?: string;
  currency?: "USD" | "RUB" | "UAH";
}): Promise<SettingsResponse> {
  return httpRequest<SettingsResponse>("/api/user/settings", {
    method: "PATCH",
    body: input,
  });
}
