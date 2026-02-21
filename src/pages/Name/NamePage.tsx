import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getUserSettings, patchUserSettings } from "@/shared/api/user";
import { BackIcon } from "@/shared/ui/Icons";

export function NamePage() {
  const [name, setName] = useState("Игрок");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    void getUserSettings()
      .then((response) => setName(response.settings.displayName || "Игрок"))
      .catch(() => undefined);
  }, []);

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
        <h1 className="page-header__title">Имя</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card form-grid">
        <label className="field">
          <span>Имя</span>
          <input value={name} onChange={(event) => setName(event.target.value)} />
        </label>
        <button className="button button--primary" type="button" onClick={() => void save()} disabled={saving}>
          {saving ? "Сохраняем..." : "Сохранить"}
        </button>
      </div>
    </section>
  );
}
