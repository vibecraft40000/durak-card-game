import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import type { GameMode, Room } from "@/entities/match/types";
import { getRooms } from "@/shared/api/rooms";
import { AppCard } from "@/shared/ui/Card";
import { AppButton } from "@/shared/ui/Button";
import { FilterIcon } from "@/shared/ui/Icons";
import { CardSkeleton, EmptyStateBlock, ErrorStateBlock } from "@/shared/ui/StateBlocks";

type FilterState = {
  mode: GameMode | "all";
  maxStake: number;
  deck: 24 | 36 | 52 | "all";
  maxPlayers: 2 | 3 | 4 | "all";
};

const INITIAL_FILTERS: FilterState = {
  mode: "all",
  maxStake: 500,
  deck: "all",
  maxPlayers: "all",
};

export function PlayPage() {
  const [rooms, setRooms] = useState<Room[]>([]);
  const [filters, setFilters] = useState<FilterState>(INITIAL_FILTERS);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isFilterOpen, setIsFilterOpen] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    void loadRooms(controller.signal);
    return () => controller.abort();
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

  const filteredRooms = useMemo(() => {
    return rooms.filter((room) => {
      const byMode = filters.mode === "all" ? true : room.mode === filters.mode;
      const byStake = room.stakeUsd <= filters.maxStake;
      const byDeck = filters.deck === "all" ? true : room.deck === filters.deck;
      const byPlayers = filters.maxPlayers === "all" ? true : room.maxPlayers === filters.maxPlayers;
      return byMode && byStake && byDeck && byPlayers;
    });
  }, [filters.deck, filters.maxPlayers, filters.maxStake, filters.mode, rooms]);

  return (
    <section className="screen play-screen">
      <div className="play-screen__head">
        <h1 className="screen__title">Игры</h1>
        <div className="play-screen__actions">
          <AppButton
            variant="ghost"
            type="button"
            onClick={() => void loadRooms()}
            disabled={isLoading}
          >
            {isLoading ? "…" : "Обновить"}
          </AppButton>
          <AppButton variant="ghost" type="button" onClick={() => setIsFilterOpen(true)}>
            <FilterIcon size={18} />
            Фильтр
          </AppButton>
        </div>
      </div>
      <p className="screen__subtitle">Выберите комнату или создайте свой стол.</p>

      <div className="quick-filter-row">
        {([
          { label: "Все", value: "all" as const },
          { label: "Подкидной", value: "Подкидной" as const },
          { label: "Переводной", value: "Переводной" as const },
        ] satisfies Array<{ label: string; value: FilterState["mode"] }>).map((item) => (
          <button
            key={item.value}
            className={`quick-filter-row__chip ${filters.mode === item.value ? "quick-filter-row__chip--active" : ""}`}
            type="button"
            onClick={() => setFilters((prev) => ({ ...prev, mode: item.value }))}
          >
            {item.label}
          </button>
        ))}
      </div>

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
              <strong className="room-card__stake">${room.stakeUsd}</strong>
              <span className={`room-card__badge ${room.players >= room.maxPlayers ? "room-card__badge--busy" : ""}`}>
                {room.players}/{room.maxPlayers}
              </span>
            </div>
            <div className="room-card__title">{room.title}</div>
            <div className="room-card__meta">
              <span>{room.mode}</span>
              <span>{room.deck} карт</span>
            </div>
            <Link className="button button--primary" to={`/room/${room.id}`}>
              Войти в комнату
            </Link>
          </AppCard>
        ))}
      </div>

      {isFilterOpen && (
        <div className="modal-backdrop" role="presentation" onClick={() => setIsFilterOpen(false)}>
          <div className="modal modal--filters" role="dialog" onClick={(event) => event.stopPropagation()}>
            <div className="modal__title">Фильтры</div>
            <div className="modal__message">Подберите стол по колоде, игрокам и типу игры.</div>

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
                    onClick={() => setFilters((prev) => ({ ...prev, maxPlayers: players as 2 | 3 | 4 }))}
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
                  { title: "Подкидной", value: "Подкидной" as const },
                  { title: "Переводной", value: "Переводной" as const },
                  { title: "Классика", value: "all" as const },
                  { title: "Честная", value: "all" as const },
                  { title: "Шулер", value: "all" as const },
                  { title: "Ничья", value: "all" as const },
                ].map((item, index) => (
                  <button
                    key={`${item.title}-${index}`}
                    className={`filter-card ${
                      (item.value !== "all" ? filters.mode === item.value : filters.mode === "all")
                        ? "filter-card--active"
                        : ""
                    }`}
                    type="button"
                    onClick={() => setFilters((prev) => ({ ...prev, mode: item.value }))}
                  >
                    {item.title}
                  </button>
                ))}
              </div>
            </div>

            <div className="modal__actions">
              <button
                className="button"
                type="button"
                onClick={() =>
                  setFilters((prev) => ({
                    ...prev,
                    deck: "all",
                    maxPlayers: "all",
                    mode: "all",
                  }))
                }
              >
                Сбросить
              </button>
              <button className="button button--primary" type="button" onClick={() => setIsFilterOpen(false)}>
                Сохранить
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
