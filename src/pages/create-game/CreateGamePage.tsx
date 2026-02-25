import type { FormEvent } from "react";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { CreateRoomInput, DeckSize, GameMode } from "@/entities/match/types";
import { createRoom } from "@/shared/api/rooms";
import {
  BackIcon,
  CheckCircleIcon,
  ClassicIcon,
  CheaterIcon,
  DrawIcon,
  FairPlayIcon,
  PodkidnoyIcon,
  TransferIcon,
} from "@/shared/ui/Icons";
import { ErrorStateBlock } from "@/shared/ui/StateBlocks";
import { AppCard } from "@/shared/ui/Card";

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

  const [fairness, setFairness] = useState<"Честная игра" | "Шулер" | "all">("all");
  const [style, setStyle] = useState<"Классика" | "Ничья" | "all">("all");

  const validationError =
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
      <p className="screen__subtitle">Выберите ставку, колоду, игроков и тип игры.</p>

      <form className="create-form" onSubmit={handleSubmit}>
        <AppCard className="card--compact">
          <div className="card__row">
            <span>Ваша ставка</span>
            <strong>${form.stakeUsd}</strong>
          </div>
          <input
            type="range"
            min={1}
            max={500}
            value={form.stakeUsd}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, stakeUsd: Number(event.target.value) || 1 }))
            }
          />
        </AppCard>

        <div className="filter-group">
          <span className="filter-group__label">Колода</span>
          <div className="pill-group">
            {[24, 36, 52].map((deck) => (
              <button
                key={deck}
                type="button"
                className={`pill ${form.deck === deck ? "pill--active" : ""}`}
                onClick={() => setForm((prev) => ({ ...prev, deck: deck as DeckSize }))}
              >
                {deck}
              </button>
            ))}
          </div>
        </div>

        <div className="filter-group">
          <span className="filter-group__label">Игроки</span>
          <div className="pill-group">
            {[2, 3, 4].map((players) => (
              <button
                key={players}
                type="button"
                className={`pill ${form.maxPlayers === players ? "pill--active" : ""}`}
                onClick={() => setForm((prev) => ({ ...prev, maxPlayers: players }))}
              >
                {players}
              </button>
            ))}
          </div>
        </div>

        <div className="filter-group">
          <span className="filter-group__label">Тип игры</span>
          <div className="filter-grid">
            {[
              {
                title: "Подкидной",
                group: "mode" as const,
                modeValue: "Подкидной" as const,
                icon: PodkidnoyIcon,
              },
              {
                title: "Честная игра",
                group: "fairness" as const,
                fairnessValue: "Честная игра" as const,
                icon: FairPlayIcon,
              },
              {
                title: "Классика",
                group: "style" as const,
                styleValue: "Классика" as const,
                icon: ClassicIcon,
              },
              {
                title: "Переводной",
                group: "mode" as const,
                modeValue: "Переводной" as const,
                icon: TransferIcon,
              },
              {
                title: "Шулер",
                group: "fairness" as const,
                fairnessValue: "Шулер" as const,
                icon: CheaterIcon,
              },
              {
                title: "Ничья",
                group: "style" as const,
                styleValue: "Ничья" as const,
                icon: DrawIcon,
              },
            ].map((item, index) => {
              const isActive =
                item.group === "mode"
                  ? form.mode === item.modeValue
                  : item.group === "fairness"
                    ? fairness === item.fairnessValue
                    : style === item.styleValue;

              return (
                <button
                  key={`${item.title}-${index}`}
                  type="button"
                  className={`filter-card ${isActive ? "filter-card--active" : ""}`}
                  onClick={() => {
                    if (item.group === "mode") {
                      const nextMode =
                        form.mode === item.modeValue ? (form.mode as GameMode) : item.modeValue;
                      setForm((prev) => ({ ...prev, mode: nextMode }));
                      return;
                    }

                    if (item.group === "fairness") {
                      const nextFairness =
                        fairness === item.fairnessValue ? "all" : item.fairnessValue;
                      setFairness(nextFairness);
                      return;
                    }

                    const nextStyle = style === item.styleValue ? "all" : item.styleValue;
                    setStyle(nextStyle);
                  }}
                >
                  <item.icon size={20} />
                  <span>{item.title}</span>
                  {isActive && (
                    <span className="filter-card__check">
                      <CheckCircleIcon size={20} />
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        </div>

        {(error || validationError) && (
          <ErrorStateBlock
            title={validationError ? "Проверьте данные" : "Не удалось создать стол"}
            message={validationError ?? error ?? ""}
          />
        )}

        <button
          type="submit"
          className="button button--primary create-game__submit"
          disabled={isSubmitting || !!validationError}
        >
          {isSubmitting ? "Создаем..." : "Создать игру"}
        </button>
      </form>
    </section>
  );
}
