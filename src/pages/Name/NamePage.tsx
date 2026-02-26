import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getUserSettings, patchUserSettings } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon } from "@/shared/ui/Icons";

export function NamePage() {
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [name, setName] = useState(tr("Игрок", "Гравець"));
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    void getUserSettings()
      .then((response) => setName(response.settings.displayName || tr("Игрок", "Гравець")))
      .catch(() => undefined);
  }, [language]);

  async function save() {
    setSaving(true);
    try {
      await patchUserSettings({ displayName: name });
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
        <h1 className="page-header__title">{tr("Имя", "Ім'я")}</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card form-grid">
        <label className="field">
          <span>{tr("Имя", "Ім'я")}</span>
          <input value={name} onChange={(event) => setName(event.target.value)} />
        </label>
        <button className="button button--primary" type="button" onClick={() => void save()} disabled={saving}>
          {saving ? tr("Сохраняем...", "Зберігаємо...") : tr("Сохранить", "Зберегти")}
        </button>
      </div>
    </section>
  );
}
