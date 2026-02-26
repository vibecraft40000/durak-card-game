import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { patchProfileLanguage } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon } from "@/shared/ui/Icons";

type LanguageCode = "ru" | "uk";

export function LanguagePage() {
  const { language, setLanguage, t, syncLanguageFromProfile } = useLanguage();
  const [savingCode, setSavingCode] = useState<LanguageCode | null>(null);

  useEffect(() => {
    void syncLanguageFromProfile();
  }, [syncLanguageFromProfile]);

  const languageOptions: Array<{ code: LanguageCode; label: string }> = [
    { code: "ru", label: t("language.ru") },
    { code: "uk", label: t("language.uk") },
  ];

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
        <h1 className="page-header__title">{t("language.title")}</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card list">
        {languageOptions.map((item) => (
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
