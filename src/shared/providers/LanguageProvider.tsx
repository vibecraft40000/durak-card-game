import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { type AppLanguage, translate } from "@/shared/i18n/messages";
import { getProfile } from "@/shared/api/user";

const LANGUAGE_STORAGE_KEY = "durak.language";

function readStoredLanguage(): AppLanguage {
  if (typeof window === "undefined") {
    return "ru";
  }
  const raw = window.localStorage.getItem(LANGUAGE_STORAGE_KEY);
  return normalizeLanguageCode(raw);
}

export function normalizeLanguageCode(value: string | undefined | null): AppLanguage {
  const normalized = String(value ?? "")
    .trim()
    .toLowerCase();
  if (normalized === "uk" || normalized === "ua") {
    return "uk";
  }
  return "ru";
}

type LanguageContextValue = {
  language: AppLanguage;
  setLanguage: (value: AppLanguage) => void;
  syncLanguageFromProfile: () => Promise<void>;
  t: (key: string, params?: Record<string, string | number>) => string;
};

const LanguageContext = createContext<LanguageContextValue | null>(null);

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<AppLanguage>(readStoredLanguage);

  useEffect(() => {
    if (typeof document !== "undefined") {
      document.documentElement.lang = language;
    }
    if (typeof window !== "undefined") {
      window.localStorage.setItem(LANGUAGE_STORAGE_KEY, language);
    }
  }, [language]);

  const setLanguage = useCallback((value: AppLanguage) => {
    setLanguageState(value);
  }, []);

  const syncLanguageFromProfile = useCallback(async () => {
    try {
      const response = await getProfile();
      setLanguageState(normalizeLanguageCode(response.user.language));
    } catch {
      // keep local preference
    }
  }, []);

  const t = useCallback(
    (key: string, params?: Record<string, string | number>) => translate(language, key, params),
    [language],
  );

  const value = useMemo<LanguageContextValue>(
    () => ({
      language,
      setLanguage,
      syncLanguageFromProfile,
      t,
    }),
    [language, setLanguage, syncLanguageFromProfile, t],
  );

  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

export function useLanguage(): LanguageContextValue {
  const ctx = useContext(LanguageContext);
  if (!ctx) {
    throw new Error("useLanguage must be used within LanguageProvider");
  }
  return ctx;
}
