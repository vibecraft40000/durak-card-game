import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getUserSettings, patchUserSettings } from "@/shared/api/user";
import { BackIcon } from "@/shared/ui/Icons";

export function CurrencyPage() {
  const [currency, setCurrency] = useState<"UAH" | "RUB" | "USD">("USD");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    void getUserSettings()
      .then((response) => setCurrency(response.settings.currency || "USD"))
      .catch(() => undefined);
  }, []);

  async function changeCurrency(next: "UAH" | "RUB" | "USD") {
    setCurrency(next);
    setSaving(true);
    try {
      await patchUserSettings({ currency: next });
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile/settings">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">Валюта</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card list">
        {(["UAH", "RUB", "USD"] as const).map((item) => (
          <button
            key={item}
            className={`menu-item menu-item--choice ${currency === item ? "menu-item--active" : ""}`}
            type="button"
            onClick={() => void changeCurrency(item)}
            disabled={saving}
          >
            <span>{item}</span>
            <span className={`radio-dot ${currency === item ? "radio-dot--active" : ""}`} />
          </button>
        ))}
      </div>
    </section>
  );
}
