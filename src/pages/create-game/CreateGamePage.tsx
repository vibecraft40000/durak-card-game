import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { CreateRoomInput, DeckSize, GameMode } from "@/entities/match/types";
import { createRoom } from "@/shared/api/rooms";
import { HttpError } from "@/shared/api/http";
import { bootstrapTelegramAuth, clearTokens } from "@/shared/api/auth";
import { getProfile } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon, CheaterIcon } from "@/shared/ui/Icons";
import { ErrorStateBlock } from "@/shared/ui/StateBlocks";
import { AppCard } from "@/shared/ui/Card";

const STAKE_PRESETS = [10, 50, 100, 200];

type Step = 0 | 1 | 2;

type StepMeta = {
  title: string;
  subtitle: string;
};

function modeToRoomMode(baseMode: GameMode, shulerEnabled: boolean): string {
  if (!shulerEnabled) {
    return baseMode;
  }
  return `${baseMode} Шулер`;
}

export function CreateGamePage() {
  const navigate = useNavigate();
  const { t } = useLanguage();
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

  const stepMeta = useMemo<Record<Step, StepMeta>>(
    () => ({
      0: {
        title: t("create.step0.title"),
        subtitle: t("create.step0.subtitle"),
      },
      1: {
        title: t("create.step1.title"),
        subtitle: t("create.step1.subtitle"),
      },
      2: {
        title: t("create.step2.title"),
        subtitle: t("create.step2.subtitle"),
      },
    }),
    [t],
  );

  const mode = useMemo(() => modeToRoomMode(baseMode, shulerEnabled), [baseMode, shulerEnabled]);
  const baseModeLabel = baseMode === "Подкидной" ? t("create.mode.podkidnoy") : t("create.mode.perevodnoy");
  const modeLabel = shulerEnabled ? `${baseModeLabel} ${t("play.types.shuler")}` : baseModeLabel;

  const validationError = useMemo(() => {
    if (stakeUsd < 1 || stakeUsd > 500) {
      return t("create.error.range");
    }
    return null;
  }, [stakeUsd, t]);

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
      title: t("create.tableTitle", { stake: stakeUsd }),
    };

    setIsSubmitting(true);
    try {
      const room = await createRoom(payload);
      navigate(`/room/${room.id}`);
      return;
    } catch (err: unknown) {
      const status = (err as { status?: number })?.status;
      if (status === 401) {
        clearTokens();
        try {
          await bootstrapTelegramAuth();
          const room = await createRoom(payload);
          navigate(`/room/${room.id}`);
          return;
        } catch {
          // continue to user-facing error below
        }
      }
      if (err instanceof HttpError && typeof err.responseBody === "string" && err.responseBody.trim()) {
        setError(err.responseBody);
      } else if (err instanceof Error && err.message.trim()) {
        setError(err.message);
      } else {
        setError(t("create.error.failed"));
      }
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
        <h1 className="page-header__title">{stepMeta[step].title}</h1>
        <div className="page-header__spacer" />
      </div>

      <p className="screen__subtitle">{stepMeta[step].subtitle}</p>

      <div className="create-flow__progress" role="tablist" aria-label={t("create.progressAria")}>
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
              <span className="create-flow__step-title">{stepMeta[current].title}</span>
            </button>
          );
        })}
      </div>

      <form className="create-form create-form--wizard" onSubmit={handleCreate}>
        {step === 0 && (
          <>
            <AppCard className="card--compact">
              <div className="card__row">
                <span>{t("create.currentStake")}</span>
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
                <span>{t("create.balance")}</span>
                <strong>{balance == null ? "—" : `$${balance.toFixed(2)}`}</strong>
              </div>
              {balance != null && balance < stakeUsd && (
                <div className="card__hint card__hint--error">{t("create.insufficientFunds")}</div>
              )}
            </AppCard>
          </>
        )}

        {step === 1 && (
          <>
            <div className="filter-group">
              <span className="filter-group__label">{t("create.cardsCount")}</span>
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
              <span className="filter-group__label">{t("create.playersCount")}</span>
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
              <span className="filter-group__label">{t("create.gameType")}</span>
              <div className="create-flow__mode-grid">
                <button
                  type="button"
                  className={`filter-card ${baseMode === "Подкидной" ? "filter-card--active" : ""}`}
                  data-active={baseMode === "Подкидной"}
                  aria-pressed={baseMode === "Подкидной"}
                  onClick={() => setBaseMode("Подкидной")}
                >
                  <span>{t("create.mode.podkidnoy")}</span>
                </button>
                <button
                  type="button"
                  className={`filter-card ${baseMode === "Переводной" ? "filter-card--active" : ""}`}
                  data-active={baseMode === "Переводной"}
                  aria-pressed={baseMode === "Переводной"}
                  onClick={() => setBaseMode("Переводной")}
                >
                  <span>{t("create.mode.perevodnoy")}</span>
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
                  <span>{t("create.shulerMode")}</span>
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
              <div className="card__hint">{t("create.shulerHint")}</div>
            </AppCard>

            <AppCard className="card--compact">
              <div className="card__label">{t("create.summary")}</div>
              <div className="create-flow__summary">
                <div className="card__row">
                  <span>{t("create.summary.stake")}</span>
                  <strong>${stakeUsd}</strong>
                </div>
                <div className="card__row">
                  <span>{t("create.summary.deck")}</span>
                  <strong>
                    {deck} {t("common.cards")}
                  </strong>
                </div>
                <div className="card__row">
                  <span>{t("create.summary.players")}</span>
                  <strong>{maxPlayers}</strong>
                </div>
                <div className="card__row">
                  <span>{t("create.summary.mode")}</span>
                  <strong>{modeLabel}</strong>
                </div>
              </div>
            </AppCard>
          </>
        )}

        {(error || validationError) && (
          <ErrorStateBlock
            title={validationError ? t("create.error.checkStake") : t("create.error.title")}
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
              {t("common.back")}
            </button>
          )}

          {canGoForward && (
            <button
              type="button"
              className="button"
              onClick={() => setStep((prev) => (prev + 1) as Step)}
            >
              {t("common.next")}
            </button>
          )}

          <button
            type="submit"
            className="button button--primary create-game__submit"
            disabled={isSubmitting || !!validationError}
          >
            {isSubmitting ? t("create.button.creating") : t("create.button.create")}
          </button>
        </div>
      </form>
    </section>
  );
}
