import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { CreateRoomInput, DeckSize, GameMode } from "@/entities/match/types";
import { createRoom } from "@/shared/api/rooms";
import { getProfile } from "@/shared/api/user";
import { BackIcon, CheaterIcon } from "@/shared/ui/Icons";
import { ErrorStateBlock } from "@/shared/ui/StateBlocks";
import { AppCard } from "@/shared/ui/Card";

const STAKE_PRESETS = [10, 50, 100, 200];

type Step = 0 | 1 | 2;

type StepMeta = {
  title: string;
  subtitle: string;
};

const STEP_META: Record<Step, StepMeta> = {
  0: {
    title: "Выберите ставку",
    subtitle: "Укажите сумму ставки и проверьте доступный баланс.",
  },
  1: {
    title: "Настройки игры",
    subtitle: "Выберите колоду, количество игроков и тип игры.",
  },
  2: {
    title: "Режим 'Шулер'",
    subtitle: "Включите режим при необходимости и создайте игру.",
  },
};

function modeToRoomMode(baseMode: GameMode, shulerEnabled: boolean): string {
  if (!shulerEnabled) {
    return baseMode;
  }
  return `${baseMode} Шулер`;
}

export function CreateGamePage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<Step>(0);
  const [stakeUsd, setStakeUsd] = useState(10);
  const [deck, setDeck] = useState<DeckSize>(36);
  const [maxPlayers, setMaxPlayers] = useState<2 | 3 | 4>(2);
  const [baseMode, setBaseMode] = useState<GameMode>("Подкидной");
  const [shulerEnabled, setShulerEnabled] = useState(false);
  const [balance, setBalance] = useState<number | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void getProfile()
      .then((response) => setBalance(response.balance))
      .catch(() => undefined);
  }, []);

  const mode = useMemo(() => modeToRoomMode(baseMode, shulerEnabled), [baseMode, shulerEnabled]);

  const validationError = useMemo(() => {
    if (stakeUsd < 1 || stakeUsd > 500) {
      return "Ставка должна быть в диапазоне 1-500 USD";
    }
    return null;
  }, [stakeUsd]);

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    if (validationError) {
      setError(validationError);
      return;
    }

    const payload: CreateRoomInput = {
      stakeUsd,
      mode,
      deck,
      maxPlayers,
      title: `Стол $${stakeUsd}`,
    };

    setIsSubmitting(true);
    try {
      const room = await createRoom(payload);
      navigate(`/room/${room.id}`);
    } catch {
      setError("Не удалось создать игру. Проверьте подключение и повторите попытку.");
    } finally {
      setIsSubmitting(false);
    }
  }

  const canGoBack = step > 0;
  const canGoForward = step < 2;

  return (
    <section className="screen create-screen create-screen--wizard">
      <div className="page-header">
        <Link className="icon-button" to="/play">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{STEP_META[step].title}</h1>
        <div className="page-header__spacer" />
      </div>

      <p className="screen__subtitle">{STEP_META[step].subtitle}</p>

      <div className="create-flow__progress" role="tablist" aria-label="Шаги создания игры">
        {[0, 1, 2].map((index) => {
          const current = index as Step;
          const isActive = current === step;
          const isDone = current < step;
          return (
            <button
              key={current}
              type="button"
              className={`create-flow__step ${isActive ? "create-flow__step--active" : ""} ${isDone ? "create-flow__step--done" : ""}`}
              onClick={() => setStep(current)}
            >
              <span className="create-flow__step-index">{current + 1}</span>
              <span className="create-flow__step-title">{STEP_META[current].title}</span>
            </button>
          );
        })}
      </div>

      <form className="create-form create-form--wizard" onSubmit={handleCreate}>
        {step === 0 && (
          <>
            <AppCard className="card--compact">
              <div className="card__row">
                <span>Текущая ставка</span>
                <strong>${stakeUsd}</strong>
              </div>
              <input
                type="range"
                min={1}
                max={500}
                value={stakeUsd}
                onChange={(event) => setStakeUsd(Number(event.target.value) || 1)}
              />
              <div className="create-flow__chips">
                {STAKE_PRESETS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    className={`pill ${stakeUsd === value ? "pill--active" : ""}`}
                    onClick={() => setStakeUsd(value)}
                  >
                    ${value}
                  </button>
                ))}
              </div>
            </AppCard>

            <AppCard className="card--compact">
              <div className="card__row">
                <span>Ваш баланс</span>
                <strong>{balance == null ? "—" : `$${balance.toFixed(2)}`}</strong>
              </div>
              {balance != null && balance < stakeUsd && (
                <div className="card__hint card__hint--error">Недостаточно средств для текущей ставки.</div>
              )}
            </AppCard>
          </>
        )}

        {step === 1 && (
          <>
            <div className="filter-group">
              <span className="filter-group__label">Количество карт</span>
              <div className="pill-group">
                {[24, 36, 52].map((size) => (
                  <button
                    key={size}
                    type="button"
                    className={`pill ${deck === size ? "pill--active" : ""}`}
                    onClick={() => setDeck(size as DeckSize)}
                  >
                    {size}
                  </button>
                ))}
              </div>
            </div>

            <div className="filter-group">
              <span className="filter-group__label">Количество игроков</span>
              <div className="pill-group">
                {[2, 3, 4].map((count) => (
                  <button
                    key={count}
                    type="button"
                    className={`pill ${maxPlayers === count ? "pill--active" : ""}`}
                    onClick={() => setMaxPlayers(count as 2 | 3 | 4)}
                  >
                    {count}
                  </button>
                ))}
              </div>
            </div>

            <div className="filter-group">
              <span className="filter-group__label">Тип игры</span>
              <div className="create-flow__mode-grid">
                <button
                  type="button"
                  className={`filter-card ${baseMode === "Подкидной" ? "filter-card--active" : ""}`}
                  onClick={() => setBaseMode("Подкидной")}
                >
                  <span>Подкидной</span>
                </button>
                <button
                  type="button"
                  className={`filter-card ${baseMode === "Переводной" ? "filter-card--active" : ""}`}
                  onClick={() => setBaseMode("Переводной")}
                >
                  <span>Переводной</span>
                </button>
              </div>
            </div>
          </>
        )}

        {step === 2 && (
          <>
            <AppCard className="card--compact create-flow__shuler-card">
              <div className="card__row">
                <div className="create-flow__shuler-title">
                  <CheaterIcon size={18} />
                  <span>Режим «Шулер»</span>
                </div>
                <button
                  type="button"
                  className={`create-flow__switch ${shulerEnabled ? "create-flow__switch--on" : ""}`}
                  aria-pressed={shulerEnabled}
                  onClick={() => setShulerEnabled((prev) => !prev)}
                >
                  <span className="create-flow__switch-knob" />
                </button>
              </div>
              <div className="card__hint">При включении система фиксирует неверные ходы.</div>
            </AppCard>

            <AppCard className="card--compact">
              <div className="card__label">Параметры игры</div>
              <div className="create-flow__summary">
                <div className="card__row">
                  <span>Ставка</span>
                  <strong>${stakeUsd}</strong>
                </div>
                <div className="card__row">
                  <span>Колода</span>
                  <strong>{deck} карт</strong>
                </div>
                <div className="card__row">
                  <span>Игроки</span>
                  <strong>{maxPlayers}</strong>
                </div>
                <div className="card__row">
                  <span>Режим</span>
                  <strong>{mode}</strong>
                </div>
              </div>
            </AppCard>
          </>
        )}

        {(error || validationError) && (
          <ErrorStateBlock
            title={validationError ? "Проверьте ставку" : "Не удалось создать игру"}
            message={validationError ?? error ?? ""}
          />
        )}

        <div className="create-flow__actions">
          {canGoBack && (
            <button
              type="button"
              className="button"
              onClick={() => setStep((prev) => (prev - 1) as Step)}
            >
              Назад
            </button>
          )}

          {canGoForward && (
            <button
              type="button"
              className="button"
              onClick={() => setStep((prev) => (prev + 1) as Step)}
            >
              Далее
            </button>
          )}

          <button
            type="submit"
            className="button button--primary create-game__submit"
            disabled={isSubmitting || !!validationError}
          >
            {isSubmitting ? "Создаем..." : "Создать игру"}
          </button>
        </div>
      </form>
    </section>
  );
}
