const LANGUAGE_STORAGE_KEY = "durak.language";

function normalizeLanguageCode(value: string | undefined | null): "ru" | "uk" {
  const normalized = String(value ?? "")
    .trim()
    .toLowerCase();
  if (normalized === "uk" || normalized === "ua") {
    return "uk";
  }
  return "ru";
}

export function getRuntimeLanguage(): "ru" | "uk" {
  if (typeof window === "undefined") {
    return "ru";
  }
  const stored = window.localStorage.getItem(LANGUAGE_STORAGE_KEY);
  return normalizeLanguageCode(stored);
}

export function trRuntime(ru: string, uk: string): string {
  return getRuntimeLanguage() === "uk" ? uk : ru;
}
