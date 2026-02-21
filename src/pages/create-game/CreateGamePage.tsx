import type { FormEvent } from "react";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { CreateRoomInput, DeckSize, GameMode } from "@/entities/match/types";
import { createRoom } from "@/shared/api/rooms";
import { BackIcon } from "@/shared/ui/Icons";
import { ErrorStateBlock } from "@/shared/ui/StateBlocks";

export function CreateGamePage() {
  const navigate = useNavigate();
  const [form, setForm] = useState<CreateRoomInput>({
    stakeUsd: 10,
    mode: "Подкидной",
    deck: 36,
    maxPlayers: 2,
    title: "Новый стол",
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const validationError =
    !form.title?.trim() ? "Введите название стола" :
    (form.title?.length ?? 0) > 50 ? "Название не более 50 символов" :
    form.stakeUsd < 1 || form.stakeUsd > 500 ? "Ставка от 1 до 500" : null;

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    if (validationError) {
      setError(validationError);
      return;
    }
    setIsSubmitting(true);

    try {
      const room = await createRoom(form);
      navigate(`/room/${room.id}`);
    } catch {
      setError("Не удалось создать стол. Проверьте API и попробуйте снова.");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <section className="screen create-screen">
      <div className="page-header">
        <Link className="icon-button" to="/play">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">Создать игру</h1>
        <div className="page-header__spacer" />
      </div>
      <p className="screen__subtitle">Задайте ставку, режим и параметры комнаты.</p>

      <form className="card" onSubmit={handleSubmit}>
        <div className="form-grid">
          <label className="field">
            <span>Название стола</span>
            <input
              type="text"
              value={form.title}
              maxLength={50}
              placeholder="Новый стол"
              onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))}
            />
          </label>

          <label className="field">
            <span>Ставка (USD)</span>
            <input
              type="number"
              value={form.stakeUsd}
              min={1}
              max={500}
              onChange={(event) =>
                setForm((prev) => ({ ...prev, stakeUsd: Number(event.target.value) || 1 }))
              }
            />
          </label>

          <label className="field">
            <span>Режим</span>
            <select
              value={form.mode}
              onChange={(event) =>
                setForm((prev) => ({ ...prev, mode: event.target.value as GameMode }))
              }
            >
              <option>Подкидной</option>
              <option>Переводной</option>
            </select>
          </label>

          <label className="field">
            <span>Колода</span>
            <select
              value={form.deck}
              onChange={(event) =>
                setForm((prev) => ({ ...prev, deck: Number(event.target.value) as DeckSize }))
              }
            >
              <option value={24}>24</option>
              <option value={36}>36</option>
              <option value={52}>52</option>
            </select>
          </label>

          <label className="field">
            <span>Игроков максимум</span>
            <select
              value={form.maxPlayers}
              onChange={(event) =>
                setForm((prev) => ({ ...prev, maxPlayers: Number(event.target.value) }))
              }
            >
              <option value={2}>2</option>
              <option value={3}>3</option>
              <option value={4}>4</option>
            </select>
          </label>
        </div>

        {(error || validationError) && (
          <ErrorStateBlock
            title={validationError ? "Проверьте данные" : "Не удалось создать стол"}
            message={validationError ?? error ?? ""}
          />
        )}

        <button type="submit" className="button button--primary" disabled={isSubmitting || !!validationError}>
          {isSubmitting ? "Создаем..." : "Создать стол"}
        </button>
      </form>
    </section>
  );
}
