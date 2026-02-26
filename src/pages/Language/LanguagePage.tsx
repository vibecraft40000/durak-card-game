import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getProfile, patchProfileLanguage } from "@/shared/api/user";
import { BackIcon } from "@/shared/ui/Icons";

type LanguageCode = "ru" | "uk";

const LANGUAGE_OPTIONS: Array<{ code: LanguageCode; label: string }> = [
  { code: "ru", label: "Русский" },
  { code: "uk", label: "Украинский" },
];

function normalizeLanguage(value: string | undefined): LanguageCode {
  return value === "uk" ? "uk" : "ru";
}

export function LanguagePage() {
  const [language, setLanguage] = useState<LanguageCode>("ru");
  const [savingCode, setSavingCode] = useState<LanguageCode | null>(null);

  useEffect(() => {
    void getProfile()
      .then((response) => {
        setLanguage(normalizeLanguage(response.user.language));
      })
      .catch(() => undefined);
  }, []);

  async function handleSelect(nextLanguage: LanguageCode) {
    if (nextLanguage === language || savingCode) {
      return;
    }

    const previous = language;
    setLanguage(nextLanguage);
    setSavingCode(nextLanguage);

    try {
      await patchProfileLanguage(nextLanguage);
    } catch {
      setLanguage(previous);
    } finally {
      setSavingCode(null);
    }
  }

  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile/settings">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">Язык</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card list">
        {LANGUAGE_OPTIONS.map((item) => (
          <button
            key={item.code}
            className={`menu-item menu-item--choice ${language === item.code ? "menu-item--active" : ""}`}
            type="button"
            disabled={savingCode !== null}
            onClick={() => {
              void handleSelect(item.code);
            }}
          >
            <span>{item.label}</span>
            <span className={`radio-dot ${language === item.code ? "radio-dot--active" : ""}`} />
          </button>
        ))}
      </div>
    </section>
  );
}
