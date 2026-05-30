import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { GameMode, Room } from "@/entities/match/types";
import { getRooms } from "@/shared/api/rooms";
import { getProfile } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { AppCard } from "@/shared/ui/Card";
import { AppButton } from "@/shared/ui/Button";
import {
  ArrowRightThinIcon,
  CheckCircleIcon,
  CheaterIcon,
  DeckIcon,
  FairPlayIcon,
  PodkidnoyIcon,
  TransferIcon,
  UserIcon,
} from "@/shared/ui/Icons";
import { CardSkeleton, EmptyStateBlock, ErrorStateBlock } from "@/shared/ui/StateBlocks";

type FilterState = {
  mode: GameMode | "all";
  fairness: "Честная игра" | "Шулер" | "all";
  maxStake: number;
  deck: 24 | 36 | 52 | "all";
  maxPlayers: 2 | 3 | 4 | "all";
};

const FILTERS_STORAGE_KEY = "play.filters.v1";

const INITIAL_FILTERS: FilterState = {
  mode: "all",
  fairness: "all",
  maxStake: 500,
  deck: "all",
  maxPlayers: "all",
};

function isValidMode(value: unknown): value is GameMode | "all" {
  return value === "all" || value === "Подкидной" || value === "Переводной";
}

function isValidFairness(value: unknown): value is FilterState["fairness"] {
  return value === "all" || value === "Честная игра" || value === "Шулер";
}

function isValidDeck(value: unknown): value is FilterState["deck"] {
  return value === "all" || value === 24 || value === 36 || value === 52;
}

function isValidMaxPlayers(value: unknown): value is FilterState["maxPlayers"] {
  return value === "all" || value === 2 || value === 3 || value === 4;
}

function normalizeFilters(partial: Partial<FilterState>): FilterState {
  const maxStake =
    typeof partial.maxStake === "number" && partial.maxStake >= 1 && partial.maxStake <= 500
      ? partial.maxStake
      : INITIAL_FILTERS.maxStake;

  const mode = isValidMode(partial.mode) ? partial.mode : INITIAL_FILTERS.mode;
  const fairness = isValidFairness(partial.fairness) ? partial.fairness : INITIAL_FILTERS.fairness;
  const deck = isValidDeck(partial.deck) ? partial.deck : INITIAL_FILTERS.deck;
  const maxPlayers = isValidMaxPlayers(partial.maxPlayers)
    ? partial.maxPlayers
    : INITIAL_FILTERS.maxPlayers;

  return {
    mode,
    fairness,
    maxStake,
    deck,
    maxPlayers,
  };
}

function parseFiltersFromSearch(search: string): FilterState | null {
  if (typeof window === "undefined" || !search) return null;
  const params = new URLSearchParams(search);
  const hasAny =
    params.has("mode") ||
    params.has("fairness") ||
    params.has("deck") ||
    params.has("maxPlayers") ||
    params.has("maxStake");

  if (!hasAny) return null;

  const modeParam = params.get("mode");
  const fairnessParam = params.get("fairness");
  const deckParam = params.get("deck");
  const maxPlayersParam = params.get("maxPlayers");
  const maxStakeParam = params.get("maxStake");

  const partial: Partial<FilterState> = {};

  if (modeParam) {
    partial.mode = modeParam as GameMode | "all";
  }
  if (fairnessParam) {
    partial.fairness = fairnessParam as FilterState["fairness"];
  }
  if (deckParam) {
    const n = Number(deckParam);
    partial.deck = (n === 24 || n === 36 || n === 52 ? n : "all") as FilterState["deck"];
  }
  if (maxPlayersParam) {
    const n = Number(maxPlayersParam);
    partial.maxPlayers = (n === 2 || n === 3 || n === 4 ? n : "all") as FilterState["maxPlayers"];
  }
  if (maxStakeParam) {
    const n = Number(maxStakeParam);
    if (!Number.isNaN(n)) {
      partial.maxStake = n;
    }
  }

  return normalizeFilters(partial);
}

function parseFiltersFromStorage(): FilterState | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(FILTERS_STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Partial<FilterState>;
    return normalizeFilters(parsed);
  } catch {
    return null;
  }
}

function roomMatchesMode(roomMode: string, modeFilter: FilterState["mode"]) {
  if (modeFilter === "all") {
    return true;
  }
  const raw = roomMode.toLowerCase();
  if (modeFilter === "Подкидной") {
    return raw.includes("подкид") || raw.includes("podkid");
  }
  return raw.includes("перевод") || raw.includes("perevod");
}

function roomMatchesFairness(roomMode: string, fairnessFilter: FilterState["fairness"]) {
  if (fairnessFilter === "all") {
    return true;
  }
  const raw = roomMode.toLowerCase();
  const isShuler = raw.includes("шулер") || raw.includes("shuler");
  if (fairnessFilter === "Шулер") {
    return isShuler;
  }
  return !isShuler;
}

function formatModeLabel(mode: string, t: (key: string, params?: Record<string, string | number>) => string) {
  const raw = mode.toLowerCase();
  const isPodkidnoy = raw.includes("подкид") || raw.includes("podkid");
  const isPerevodnoy = raw.includes("перевод") || raw.includes("perevod");
  const isShuler = raw.includes("шулер") || raw.includes("shuler");

  const baseLabel = isPodkidnoy
    ? t("play.types.podkidnoy")
    : isPerevodnoy
      ? t("play.types.perevodnoy")
      : mode;

  if (!isShuler) {
    return baseLabel;
  }
  return `${baseLabel} ${t("play.types.shuler")}`;
}







function matchRoomModeStable(roomMode: string, modeFilter: FilterState["mode"]) {
  if (modeFilter === "all") {
    return true;
  }
  const raw = roomMode.toLowerCase();
  const requestedMode = String(modeFilter).toLowerCase();
  const isPodkidnoy = raw.includes("\u043f\u043e\u0434\u043a\u0438\u0434") || raw.includes("podkid");
  const isPerevodnoy = raw.includes("\u043f\u0435\u0440\u0435\u0432\u043e\u0434") || raw.includes("perevod");
  const wantsPodkidnoy =
    requestedMode.includes("\u043f\u043e\u0434\u043a\u0438\u0434") || requestedMode.includes("podkid");
  return wantsPodkidnoy ? isPodkidnoy : isPerevodnoy;
}

function matchRoomFairnessStable(roomMode: string, fairnessFilter: FilterState["fairness"]) {
  if (fairnessFilter === "all") {
    return true;
  }
  const raw = roomMode.toLowerCase();
  const requestedFairness = String(fairnessFilter).toLowerCase();
  const isShuler = raw.includes("\u0448\u0443\u043b\u0435\u0440") || raw.includes("shuler");
  const wantsShuler =
    requestedFairness.includes("\u0448\u0443\u043b\u0435\u0440") || requestedFairness.includes("shuler");
  return wantsShuler ? isShuler : !isShuler;
}

export function PlayPage() {
  const navigate = useNavigate();
  const { t } = useLanguage();
  const [rooms, setRooms] = useState<Room[]>([]);
  const [filters, setFilters] = useState<FilterState>(() => {
    if (typeof window === "undefined") return INITIAL_FILTERS;
    const fromSearch = parseFiltersFromSearch(window.location.search);
    if (fromSearch) return fromSearch;
    const fromStorage = parseFiltersFromStorage();
    if (fromStorage) return fromStorage;
    return INITIAL_FILTERS;
  });
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [joinRoomId, setJoinRoomId] = useState("");
  const [joinRoomError, setJoinRoomError] = useState<string | null>(null);
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [balance, setBalance] = useState<number | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    void loadRooms(controller.signal);
    return () => controller.abort();
  }, []);

  useEffect(() => {
    // РђРІС‚РѕРѕР±РЅРѕРІР»РµРЅРёРµ СЃРїРёСЃРєР° РєРѕРјРЅР°С‚ РєР°Р¶РґС‹Рµ 5 СЃРµРєСѓРЅРґ
    const intervalId = window.setInterval(() => {
      void loadRooms();
    }, 5000);
    return () => {
      window.clearInterval(intervalId);
    };
  }, []);

  useEffect(() => {
    void getProfile()
      .then((r) => setBalance(r.balance))
      .catch(() => undefined);
  }, []);

  async function loadRooms(signal?: AbortSignal) {
    setIsLoading(true);
    setError(null);
    try {
      const data = await getRooms(signal);
      setRooms(data);
    } catch {
      setError(t("play.loadingError"));
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    if (typeof window === "undefined") return;

    const params = new URLSearchParams();

    if (filters.mode !== "all") {
      params.set("mode", filters.mode);
    }
    if (filters.fairness !== "all") {
      params.set("fairness", filters.fairness);
    }
    if (filters.deck !== "all") {
      params.set("deck", String(filters.deck));
    }
    if (filters.maxPlayers !== "all") {
      params.set("maxPlayers", String(filters.maxPlayers));
    }
    if (filters.maxStake !== INITIAL_FILTERS.maxStake) {
      params.set("maxStake", String(filters.maxStake));
    }

    const search = params.toString();
    const { pathname, hash } = window.location;
    const newUrl = search ? `${pathname}?${search}${hash}` : `${pathname}${hash}`;

    window.history.replaceState(null, "", newUrl);
    window.localStorage.setItem(FILTERS_STORAGE_KEY, JSON.stringify(filters));
  }, [filters]);

  const filteredRooms = useMemo(() => {
    return rooms.filter((room) => {
      const byMode = matchRoomModeStable(room.mode, filters.mode);
      const byFairness = matchRoomFairnessStable(room.mode, filters.fairness);
      const byStake = room.stakeUsd <= filters.maxStake;
      const byDeck = filters.deck === "all" ? true : room.deck === filters.deck;
      const byPlayers = filters.maxPlayers === "all" ? true : room.maxPlayers === filters.maxPlayers;
      return byMode && byFairness && byStake && byDeck && byPlayers;
    });
  }, [filters.deck, filters.fairness, filters.maxPlayers, filters.maxStake, filters.mode, rooms]);

  const quickRoom = useMemo(
    () => filteredRooms.find((room) => room.players < room.maxPlayers) ?? null,
    [filteredRooms],
  );

  function handleQuickGame() {
    if (quickRoom) {
      navigate(`/room/${quickRoom.id}`);
      return;
    }
    navigate("/play/create");
  }

  function handleConnectById() {
    const roomId = joinRoomId.trim();
    if (!roomId) {
      setJoinRoomError(t("play.quick.enterId"));
      return;
    }
    setJoinRoomError(null);
    navigate(`/room/${roomId}`);
  }

  const hasDeckFilter = filters.deck !== "all";
  const hasPlayersFilter = filters.maxPlayers !== "all";
  const hasStakeFilter = filters.maxStake !== INITIAL_FILTERS.maxStake;
  const isModePodkidnoy = filters.mode === "Подкидной";
  const isModeTransfer = filters.mode === "Переводной";
  const isFairFairPlay = filters.fairness === "Честная игра";
  const isFairCheater = filters.fairness === "Шулер";

  return (
    <>
      <section className="screen play-screen">
        <div className="play-screen__head">
          <h1 className="screen__title">{t("play.title")}</h1>
          <div className="play-screen__head-balance">
            <div className="play-screen__head-balance-main">
              <span className="play-screen__head-balance-symbol">$</span>
              <span className="play-screen__head-balance-amount">
                {balance === null ? "вЂ”" : balance.toFixed(0)}
              </span>
            </div>
            <AppButton
              type="button"
              className="play-screen__head-balance-add"
              onClick={() => navigate("/profile/deposit")}
            >
              +
            </AppButton>
          </div>
        </div>

      {isFilterOpen ? (
        <>
          <p className="screen__subtitle">{t("play.subtitle.filters")}</p>

          <AppCard className="card--compact">
            <div className="card__row">
              <span>{t("play.maxStake")}</span>
              <strong>${filters.maxStake}</strong>
            </div>
            <input
              type="range"
              min={1}
              max={500}
              value={filters.maxStake}
              onChange={(event) =>
                setFilters((prev) => ({ ...prev, maxStake: Number(event.target.value) }))
              }
            />
          </AppCard>

          <div className="filter-group">
            <span className="filter-group__label">{t("play.deck")}</span>
            <div className="pill-group">
              {[24, 36, 52].map((deck) => (
                <button
                  key={deck}
                  className={`pill ${filters.deck === deck ? "pill--active" : ""}`}
                  type="button"
                  onClick={() => setFilters((prev) => ({ ...prev, deck: deck as 24 | 36 | 52 }))}
                >
                  {deck}
                </button>
              ))}
            </div>
          </div>

          <div className="filter-group">
            <span className="filter-group__label">{t("play.players")}</span>
            <div className="pill-group">
              {[2, 3, 4].map((players) => (
                <button
                  key={players}
                  className={`pill ${filters.maxPlayers === players ? "pill--active" : ""}`}
                  type="button"
                  onClick={() =>
                    setFilters((prev) => ({ ...prev, maxPlayers: players as 2 | 3 | 4 }))
                  }
                >
                  {players}
                </button>
              ))}
            </div>
          </div>

          <div className="filter-group">
            <span className="filter-group__label">{t("play.gameType")}</span>
            <div className="filter-grid">
              {[
                {
                  title: t("play.types.podkidnoy"),
                  group: "mode" as const,
                  modeValue: "Подкидной" as const,
                  icon: PodkidnoyIcon,
                },
                {
                  title: t("play.types.fair"),
                  group: "fairness" as const,
                  fairnessValue: "Честная игра" as const,
                  icon: FairPlayIcon,
                },
                {
                  title: t("play.types.perevodnoy"),
                  group: "mode" as const,
                  modeValue: "Переводной" as const,
                  icon: TransferIcon,
                },
                {
                  title: t("play.types.shuler"),
                  group: "fairness" as const,
                  fairnessValue: "Шулер" as const,
                  icon: CheaterIcon,
                },
              ].map((item, index) => {
                const isActive =
                  item.group === "mode"
                    ? filters.mode === item.modeValue
                    : filters.fairness === item.fairnessValue;

                return (
                  <button
                    key={`${item.title}-${index}`}
                    className={`filter-card ${isActive ? "filter-card--active" : ""}`}
                    type="button"
                    onClick={() =>
                      setFilters((prev) => {
                        if (item.group === "mode") {
                          const nextMode =
                            prev.mode === item.modeValue ? "all" : item.modeValue;
                          return { ...prev, mode: nextMode };
                        }

                        if (item.group === "fairness") {
                          const nextFairness =
                            prev.fairness === item.fairnessValue ? "all" : item.fairnessValue;
                          return { ...prev, fairness: nextFairness };
                        }
                        return prev;
                      })
                    }
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

          <div className="filters-screen__actions">
            <button
              className="button"
              type="button"
              onClick={() =>
                setFilters((prev) => ({
                  ...prev,
                  deck: "all",
                  maxPlayers: "all",
                  mode: "all",
                  fairness: "all",
                }))
              }
            >
              {t("common.reset")}
            </button>
            <button
              className="button button--primary"
              type="button"
              onClick={() => setIsFilterOpen(false)}
            >
              {t("common.save")}
            </button>
          </div>
        </>
      ) : (
        <>
          <p className="screen__subtitle">{t("play.subtitle.main")}</p>

          <AppCard className="play-main-actions">
            <button type="button" className="button button--primary" onClick={handleQuickGame}>
              {t("play.quick.quickGame")}
            </button>
            <div className="play-main-actions__join-row">
              <input
                value={joinRoomId}
                onChange={(event) => {
                  setJoinRoomId(event.target.value);
                  if (joinRoomError) {
                    setJoinRoomError(null);
                  }
                }}
                placeholder={t("play.quick.joinPlaceholder")}
              />
              <button type="button" className="button" onClick={handleConnectById}>
                {t("play.quick.connect")}
              </button>
            </div>
            {joinRoomError && <div className="card__hint card__hint--error">{joinRoomError}</div>}
          </AppCard>

          <div className="play-quick-filters">
            <button
              type="button"
              className="play-quick-filters__group play-quick-filters__group--types"
              onClick={() => setIsFilterOpen(true)}
            >
              <div className="play-quick-filters__types-grid">
                <span
                  className={`play-quick-filters__icon ${
                    isModePodkidnoy ? "play-quick-filters__icon--active" : ""
                  }`}
                >
                  <PodkidnoyIcon size={18} />
                </span>
                <span
                  className={`play-quick-filters__icon ${
                    isFairFairPlay ? "play-quick-filters__icon--active" : ""
                  }`}
                >
                  <FairPlayIcon size={18} />
                </span>
                <span
                  className={`play-quick-filters__icon ${
                    isModeTransfer ? "play-quick-filters__icon--active" : ""
                  }`}
                >
                  <TransferIcon size={18} />
                </span>
                <span
                  className={`play-quick-filters__icon ${
                    isFairCheater ? "play-quick-filters__icon--active" : ""
                  }`}
                >
                  <CheaterIcon size={18} />
                </span>
              </div>
            </button>

            <button
              type="button"
              className="play-quick-filters__group"
              onClick={() => setIsFilterOpen(true)}
            >
              <div className="play-quick-filters__label">
                <DeckIcon size={16} />
                <span>{t("play.deck")}</span>
              </div>
              {hasDeckFilter ? (
                <div className="play-quick-filters__value">
                  <span>{t("play.cardCountSuffix", { count: filters.deck })}</span>
                </div>
              ) : (
                <div className="play-quick-filters__value play-quick-filters__value--column">
                  <span>24</span>
                  <span>36</span>
                  <span>52</span>
                </div>
              )}
            </button>

            <div className="play-quick-filters__stack">
              <button
                type="button"
                className="play-quick-filters__group"
                onClick={() => setIsFilterOpen(true)}
              >
                <div className="play-quick-filters__label">
                  <UserIcon size={16} />
                  <span>{t("play.players")}</span>
                </div>
                {hasPlayersFilter ? (
                  <div className="play-quick-filters__value">
                    <span>{t("play.playersSuffix", { count: filters.maxPlayers })}</span>
                  </div>
                ) : (
                  <div className="play-quick-filters__value">
                    <span>{t("play.playersRange")}</span>
                  </div>
                )}
              </button>

              <button
                type="button"
                className="play-quick-filters__group"
                onClick={() => setIsFilterOpen(true)}
              >
                <div className="play-quick-filters__label">
                  <span>$</span>
                </div>
                <div className="play-quick-filters__value">
                  <span>
                    {t("play.stakeRange", { max: filters.maxStake, plus: hasStakeFilter ? "+" : "" })}
                  </span>
                </div>
              </button>
            </div>

            <button
              type="button"
              className="play-quick-filters__arrow"
              onClick={() => setIsFilterOpen(true)}
            >
              <ArrowRightThinIcon size={18} />
            </button>
          </div>

          {isLoading && (
            <div className="list">
              <CardSkeleton rows={3} />
              <CardSkeleton rows={3} />
            </div>
          )}
          {error && (
            <ErrorStateBlock
              title={t("play.errorTitle")}
              message={error}
              actionLabel={t("common.retry")}
              onAction={() => void loadRooms()}
            />
          )}

          {!isLoading && !error && filteredRooms.length === 0 && (
            <EmptyStateBlock
              title={t("play.empty.title")}
              message={t("play.empty.message")}
            />
          )}

          <div className="list">
            {filteredRooms.map((room) => (
              <AppCard className="room-card" key={room.id}>
                <div className="room-card__top">
                  <div className="room-card__stake-wrap">
                    <span className="room-card__stake-currency">$</span>
                    <strong className="room-card__stake">{room.stakeUsd}</strong>
                  </div>
                  <span
                    className={`room-card__badge ${
                      room.players >= room.maxPlayers ? "room-card__badge--busy" : ""
                    }`}
                  >
                    {room.players}/{room.maxPlayers}
                  </span>
                </div>
                <div className="room-card__title">{room.title}</div>
                <div className="room-card__bottom">
                  <div className="room-card__meta">
                    <span>{formatModeLabel(room.mode, t)}</span>
                    <span>{t("play.cardCountSuffix", { count: room.deck })}</span>
                  </div>
                  <Link className="room-card__enter" to={`/room/${room.id}`}>
                    <ArrowRightThinIcon size={18} />
                  </Link>
                </div>
              </AppCard>
            ))}
          </div>
        </>
      )}
      </section>

      
    </>
  );
}


