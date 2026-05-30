import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { CreateRoomInput, DeckSize, GameMode } from "@/entities/match/types";
import { createRoom } from "@/shared/api/rooms";
import { HttpError } from "@/shared/api/http";
import { resetAndBootstrapTelegramAuth } from "@/shared/api/auth.lazy";
import { getProfile } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import {
  BackIcon,
  CheckCircleIcon,
  CheaterIcon,
  PlusIcon,
  PodkidnoyIcon,
  TransferIcon,
} from "@/shared/ui/Icons";
import { ErrorStateBlock } from "@/shared/ui/StateBlocks";
import { AppCard } from "@/shared/ui/Card";

const STAKE_PRESETS = [10, 50, 100, 200];
const STAKE_MARKS = [1, 50, 100, 250, 500];
const DECK_OPTIONS = [24, 36, 52] as const;
const PLAYER_OPTIONS = [2, 3, 4] as const;

type Step = 0 | 1 | 2;

type StepMeta = {
  title: string;
  subtitle: string;
};

function formatUsd(value: number | null) {
  if (value == null) {
    return "—";
  }
  if (Number.isInteger(value)) {
    return `$${value}`;
  }
  return `$${value.toFixed(2)}`;
}

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
  const insufficientFunds = balance != null && balance < stakeUsd;

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
        try {
          await resetAndBootstrapTelegramAuth();
          const room = await createRoom(payload);
          navigate(`/room/${room.id}`);
          return;
        } catch {
          // fall through to user-facing error
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
      <div className="create-wizard__header">
        <Link className="icon-button create-wizard__back" to="/play" aria-label={t("common.back")}>
          <BackIcon size={17} />
        </Link>
        <div className="create-wizard__heading">
          <h1 className="create-wizard__title">{stepMeta[step].title}</h1>
          <p className="create-wizard__subtitle">{stepMeta[step].subtitle}</p>
        </div>
        <div className="create-wizard__header-spacer" />
      </div>

      <div className="create-wizard__balance-chip">
        <span>{t("create.balance")}</span>
        <strong>{formatUsd(balance)}</strong>
      </div>

      <div className="create-wizard__steps" role="tablist" aria-label={t("create.progressAria")}>
        {[0, 1, 2].map((index) => {
          const current = index as Step;
          const isActive = current === step;
          const isDone = current < step;
          return (
            <button
              key={current}
              type="button"
              className={`create-wizard__step ${isActive ? "create-wizard__step--active" : ""} ${isDone ? "create-wizard__step--done" : ""}`}
              onClick={() => setStep(current)}
              aria-current={isActive ? "step" : undefined}
              aria-label={stepMeta[current].title}
            >
              <span className="create-wizard__step-index">
                {isDone ? <CheckCircleIcon size={18} /> : current + 1}
              </span>
            </button>
          );
        })}
      </div>

      <form className="create-form create-form--wizard create-wizard__form" onSubmit={handleCreate}>
        {step === 0 && (
          <AppCard className="card--compact create-wizard__panel create-wizard__panel--stake">
            <div className="create-wizard__section-head">
              <div className="create-wizard__section-copy">
                <span className="create-wizard__section-label">{t("create.currentStake")}</span>
              </div>
              <div className="create-wizard__value-pill">{formatUsd(stakeUsd)}</div>
            </div>

            <div className="create-wizard__slider-wrap">
              <input
                className="create-wizard__slider"
                type="range"
                min={1}
                max={500}
                value={stakeUsd}
                onChange={(event) => setStakeUsd(Number(event.target.value) || 1)}
              />
              <div className="create-wizard__slider-scale" aria-hidden="true">
                {STAKE_MARKS.map((value) => (
                  <span key={value}>{value}</span>
                ))}
              </div>
            </div>

            <div className="create-wizard__chips">
              {STAKE_PRESETS.map((value) => (
                <button
                  key={value}
                  type="button"
                  className={`create-wizard__chip ${stakeUsd === value ? "create-wizard__chip--active" : ""}`}
                  onClick={() => setStakeUsd(value)}
                >
                  ${value}
                </button>
              ))}
            </div>

            <div className="create-wizard__panel-divider" />

            <div className="create-wizard__balance-card">
              <div className="create-wizard__balance-row">
                <span className="create-wizard__section-label">{t("create.balance")}</span>
                <strong>{formatUsd(balance)}</strong>
              </div>
              {insufficientFunds && <div className="create-wizard__warning">{t("create.insufficientFunds")}</div>}
            </div>
          </AppCard>
        )}

        {step === 1 && (
          <AppCard className="card--compact create-wizard__panel">
            <div className="create-wizard__group">
              <span className="create-wizard__group-title">{t("create.cardsCount")}</span>
              <div className="create-wizard__segments">
                {DECK_OPTIONS.map((size) => (
                  <button
                    key={size}
                    type="button"
                    className={`create-wizard__segment ${deck === size ? "create-wizard__segment--active" : ""}`}
                    onClick={() => setDeck(size)}
                  >
                    <span>{size}</span>
                    {deck === size && <CheckCircleIcon size={18} />}
                  </button>
                ))}
              </div>
            </div>

            <div className="create-wizard__group">
              <span className="create-wizard__group-title">{t("create.playersCount")}</span>
              <div className="create-wizard__segments">
                {PLAYER_OPTIONS.map((count) => (
                  <button
                    key={count}
                    type="button"
                    className={`create-wizard__segment ${maxPlayers === count ? "create-wizard__segment--active" : ""}`}
                    onClick={() => setMaxPlayers(count)}
                  >
                    <span>{count}</span>
                    {maxPlayers === count && <CheckCircleIcon size={18} />}
                  </button>
                ))}
              </div>
            </div>

            <div className="create-wizard__group">
              <span className="create-wizard__group-title">{t("create.gameType")}</span>
              <div className="create-wizard__mode-grid">
                <button
                  type="button"
                  className={`create-wizard__mode-card ${baseMode === "Подкидной" ? "create-wizard__mode-card--active" : ""}`}
                  onClick={() => setBaseMode("Подкидной")}
                >
                  {baseMode === "Подкидной" && <CheckCircleIcon className="create-wizard__mode-check" size={20} />}
                  <PodkidnoyIcon size={22} />
                  <span>{t("create.mode.podkidnoy")}</span>
                </button>
                <button
                  type="button"
                  className={`create-wizard__mode-card ${baseMode === "Переводной" ? "create-wizard__mode-card--active" : ""}`}
                  onClick={() => setBaseMode("Переводной")}
                >
                  {baseMode === "Переводной" && <CheckCircleIcon className="create-wizard__mode-check" size={20} />}
                  <TransferIcon size={22} />
                  <span>{t("create.mode.perevodnoy")}</span>
                </button>
              </div>
            </div>
          </AppCard>
        )}

        {step === 2 && (
          <AppCard className="card--compact create-wizard__panel create-wizard__panel--shuler">
            <div className="create-wizard__shuler-toggle">
              <div className="create-wizard__shuler-copy">
                <div className="create-wizard__shuler-icon">
                  <CheaterIcon size={18} />
                </div>
                <div className="create-wizard__shuler-text">
                  <span className="create-wizard__group-title">{t("create.shulerMode")}</span>
                  <span className="create-wizard__hint">{t("create.shulerHint")}</span>
                </div>
              </div>
              <button
                type="button"
                className={`create-wizard__switch ${shulerEnabled ? "create-wizard__switch--on" : ""}`}
                aria-pressed={shulerEnabled}
                onClick={() => setShulerEnabled((prev) => !prev)}
              >
                <span className="create-wizard__switch-knob" />
              </button>
            </div>

            <div className="create-wizard__summary-card">
              <div className="create-wizard__section-label create-wizard__section-label--accent">{t("create.summary")}</div>
              <div className="create-wizard__summary">
                <div className="create-wizard__summary-row">
                  <span>{t("create.summary.stake")}</span>
                  <strong>{formatUsd(stakeUsd)}</strong>
                </div>
                <div className="create-wizard__summary-row">
                  <span>{t("create.summary.deck")}</span>
                  <strong>
                    {deck} {t("common.cards")}
                  </strong>
                </div>
                <div className="create-wizard__summary-row">
                  <span>{t("create.summary.players")}</span>
                  <strong>{maxPlayers}</strong>
                </div>
                <div className="create-wizard__summary-row">
                  <span>{t("create.summary.mode")}</span>
                  <strong>{modeLabel}</strong>
                </div>
              </div>
            </div>
          </AppCard>
        )}

        {(error || validationError) && (
          <ErrorStateBlock
            title={validationError ? t("create.error.checkStake") : t("create.error.title")}
            message={validationError ?? error ?? ""}
          />
        )}

        <div className="create-wizard__actions">
          {(canGoBack || canGoForward) && (
            <div
              className={`create-wizard__nav-row ${
                canGoBack && canGoForward ? "create-wizard__nav-row--two" : "create-wizard__nav-row--single"
              }`}
            >
              {canGoBack && (
                <button
                  type="button"
                  className="button create-wizard__nav-button"
                  onClick={() => setStep((prev) => (prev - 1) as Step)}
                >
                  {t("common.back")}
                </button>
              )}

              {canGoForward && (
                <button
                  type="button"
                  className="button create-wizard__nav-button"
                  onClick={() => setStep((prev) => (prev + 1) as Step)}
                >
                  {t("common.next")}
                </button>
              )}
            </div>
          )}

          <button
            type="submit"
            className="button button--primary create-wizard__create-button"
            disabled={isSubmitting || !!validationError}
          >
            <PlusIcon size={16} />
            {isSubmitting ? t("create.button.creating") : t("create.button.create")}
          </button>
        </div>
      </form>
    </section>
  );
}
