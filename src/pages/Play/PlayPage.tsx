import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { GameMode, Room } from "@/entities/match/types";
import { getRooms } from "@/shared/api/rooms";
import { getProfile } from "@/shared/api/user";
import { AppCard } from "@/shared/ui/Card";
import { AppButton } from "@/shared/ui/Button";
import {
  ArrowRightThinIcon,
  CheckCircleIcon,
  ClassicIcon,
  CheaterIcon,
  DeckIcon,
  DrawIcon,
  FairPlayIcon,
  PodkidnoyIcon,
  TransferIcon,
  UserIcon,
} from "@/shared/ui/Icons";
import { CardSkeleton, EmptyStateBlock, ErrorStateBlock } from "@/shared/ui/StateBlocks";

type FilterState = {
  mode: GameMode | "all";
  fairness: "Честная игра" | "Шулер" | "all";
  style: "Классика" | "Ничья" | "all";
  maxStake: number;
  deck: 24 | 36 | 52 | "all";
  maxPlayers: 2 | 3 | 4 | "all";
};

const FILTERS_STORAGE_KEY = "play.filters.v1";

const INITIAL_FILTERS: FilterState = {
  mode: "all",
  fairness: "all",
  style: "all",
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

function isValidStyle(value: unknown): value is FilterState["style"] {
  return value === "all" || value === "Классика" || value === "Ничья";
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
  const style = isValidStyle(partial.style) ? partial.style : INITIAL_FILTERS.style;
  const deck = isValidDeck(partial.deck) ? partial.deck : INITIAL_FILTERS.deck;
  const maxPlayers = isValidMaxPlayers(partial.maxPlayers)
    ? partial.maxPlayers
    : INITIAL_FILTERS.maxPlayers;

  return {
    mode,
    fairness,
    style,
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
    params.has("style") ||
    params.has("deck") ||
    params.has("maxPlayers") ||
    params.has("maxStake");

  if (!hasAny) return null;

  const modeParam = params.get("mode");
  const fairnessParam = params.get("fairness");
  const styleParam = params.get("style");
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
  if (styleParam) {
    partial.style = styleParam as FilterState["style"];
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

export function PlayPage() {
  const navigate = useNavigate();
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
    // Автообновление списка комнат каждые 5 секунд
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
      setError("Не удалось загрузить список игр.");
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
    if (filters.style !== "all") {
      params.set("style", filters.style);
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
      const byMode = roomMatchesMode(room.mode, filters.mode);
      const byStake = room.stakeUsd <= filters.maxStake;
      const byDeck = filters.deck === "all" ? true : room.deck === filters.deck;
      const byPlayers = filters.maxPlayers === "all" ? true : room.maxPlayers === filters.maxPlayers;
      return byMode && byStake && byDeck && byPlayers;
    });
  }, [filters.deck, filters.maxPlayers, filters.maxStake, filters.mode, rooms]);

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
      setJoinRoomError("Введите ID игры");
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
  const isStyleClassic = filters.style === "Классика";
  const isStyleDraw = filters.style === "Ничья";

  return (
    <>
      <section className="screen play-screen">
        <div className="play-screen__head">
          <h1 className="screen__title">Игры</h1>
          <div className="play-screen__head-balance">
            <div className="play-screen__head-balance-main">
              <span className="play-screen__head-balance-symbol">$</span>
              <span className="play-screen__head-balance-amount">
                {balance === null ? "—" : balance.toFixed(0)}
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
          <p className="screen__subtitle">
            Подберите стол по колоде, игрокам и типу игры.
          </p>

          <AppCard className="card--compact">
            <div className="card__row">
              <span>Макс. ставка</span>
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
            <span className="filter-group__label">Колода</span>
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
            <span className="filter-group__label">Игроки</span>
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
                    ? filters.mode === item.modeValue
                    : item.group === "fairness"
                      ? filters.fairness === item.fairnessValue
                      : filters.style === item.styleValue;

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

                        const nextStyle =
                          prev.style === item.styleValue ? "all" : item.styleValue;
                        return { ...prev, style: nextStyle };
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
                  style: "all",
                }))
              }
            >
              Сбросить
            </button>
            <button
              className="button button--primary"
              type="button"
              onClick={() => setIsFilterOpen(false)}
            >
              Сохранить
            </button>
          </div>
        </>
      ) : (
        <>
          <p className="screen__subtitle">Выберите комнату или создайте свой стол.</p>

          <AppCard className="play-main-actions">
            <Link className="button button--primary" to="/play/create">
              Создать игру
            </Link>
            <button type="button" className="button" onClick={handleQuickGame}>
              Быстрая игра
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
                placeholder="ID комнаты"
              />
              <button type="button" className="button" onClick={handleConnectById}>
                Подключиться
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
                    isStyleDraw ? "play-quick-filters__icon--active" : ""
                  }`}
                >
                  <DrawIcon size={18} />
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
                <span
                  className={`play-quick-filters__icon ${
                    isStyleClassic ? "play-quick-filters__icon--active" : ""
                  }`}
                >
                  <ClassicIcon size={18} />
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
                <span>Колода</span>
              </div>
              {hasDeckFilter ? (
                <div className="play-quick-filters__value">
                  <span>{filters.deck} карт</span>
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
                  <span>Игроки</span>
                </div>
                {hasPlayersFilter ? (
                  <div className="play-quick-filters__value">
                    <span>{filters.maxPlayers} игрока</span>
                  </div>
                ) : (
                  <div className="play-quick-filters__value">
                    <span>2–3–4</span>
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
                    10 – {filters.maxStake}
                    {hasStakeFilter ? "+" : ""}
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
              title="Ошибка загрузки"
              message={error}
              actionLabel="Повторить"
              onAction={() => void loadRooms()}
            />
          )}

          {!isLoading && !error && filteredRooms.length === 0 && (
            <EmptyStateBlock
              title="Комнаты не найдены"
              message="С такими фильтрами активных столов нет. Попробуйте изменить параметры или создать игру."
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
                    <span>{room.mode}</span>
                    <span>{room.deck} карт</span>
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

