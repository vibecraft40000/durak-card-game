import { useState } from "react";
import { Link } from "react-router-dom";
import { BackIcon } from "@/shared/ui/Icons";

export function LanguagePage() {
  const [language, setLanguage] = useState<"Русский" | "Украинский">("Русский");

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
        {(["Русский", "Украинский"] as const).map((item) => (
          <button
            key={item}
            className={`menu-item menu-item--choice ${language === item ? "menu-item--active" : ""}`}
            type="button"
            onClick={() => setLanguage(item)}
          >
            <span>{item}</span>
            <span className={`radio-dot ${language === item ? "radio-dot--active" : ""}`} />
          </button>
        ))}
      </div>
    </section>
  );
}
