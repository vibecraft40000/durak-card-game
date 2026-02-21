import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import type { Room } from "@/entities/match/types";
import { getRoom, joinRoom, leaveRoom, normalizeRoom } from "@/shared/api/rooms";
import { getProfile } from "@/shared/api/user";
import { onWsEvent } from "@/shared/api/ws/events";
import { wsClient } from "@/shared/api/ws/socket";
import { hapticImpact, hapticNotification } from "@/shared/lib/telegram";
import { BackIcon } from "@/shared/ui/Icons";
import { CardSkeleton, ConfirmModal, EmptyStateBlock, ErrorStateBlock } from "@/shared/ui/StateBlocks";
import { AppCard } from "@/shared/ui/Card";
import { AppButton } from "@/shared/ui/Button";

export function GameRoomPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [rooms, setRooms] = useState<Room[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [infoMessage, setInfoMessage] = useState<string | null>(null);
  const [isReadyLoading, setIsReadyLoading] = useState(false);
  const [isWaitingStart, setIsWaitingStart] = useState(false);
  const [currentUserId, setCurrentUserId] = useState<string>("");
  const [isLeaveModalOpen, setIsLeaveModalOpen] = useState(false);

  useEffect(() => {
    void getProfile()
      .then((response) => setCurrentUserId(response.user.id))
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    if (!id) {
      setIsLoading(false);
      return;
    }
    // Join room first (idempotent for creator), then fetch latest state
    void joinRoom(id)
      .then((roomData) => setRooms([roomData]))
      .catch(() => setError("Не удалось войти в комнату."))
      .finally(() => setIsLoading(false));
  }, [id]);

  const room = useMemo(() => rooms.find((item) => item.id === id), [id, rooms]);
  const isCurrentUserConfirmed = room && currentUserId ? room.readyUserIds.includes(currentUserId) : false;
  const playersCount = room ? (room.playerIds?.length ?? room.players ?? 0) : 0;
  const canConfirm = Boolean(
    room &&
      room.status === "waiting" &&
      playersCount >= 2 &&
      !isCurrentUserConfirmed,
  );

  async function confirmAndStart() {
    if (!id) {
      return;
    }
    if (!room || room.players < 2) {
      setInfoMessage("Нельзя начать: в комнате пока нет соперника.");
      return;
    }
    setIsReadyLoading(true);
    setError(null);
    setInfoMessage(null);

    try {
      console.log("[confirm_flow] send confirm_join", {
        roomId: id,
        currentUserId,
        readyUserIds: room.readyUserIds,
        playersCount,
      });
      wsClient.send({ type: "confirm_join", payload: { roomId: id } });
      setInfoMessage("Подтверждение отправлено...");
      setIsWaitingStart(true);
    } catch {
      setError("Не удалось подтвердить готовность. Попробуйте снова.");
    } finally {
      setIsReadyLoading(false);
    }
  }

  useEffect(() => {
    if (!id) {
      return;
    }
    wsClient.connect(id);
    const offRoomUpdate = onWsEvent("room_update", ({ payload }) => {
      if (!payload || typeof payload !== "object") {
        return;
      }
      const data = payload as Record<string, unknown>;
      if (!data.id && !data.roomId) {
        return;
      }
      const normalized = normalizeRoom(data);
      if (normalized.id !== id) {
        return;
      }
      console.log("[confirm_flow] room_update", {
        roomId: normalized.id,
        readyPlayers: normalized.readyPlayers,
        readyUserIds: normalized.readyUserIds,
        players: normalized.players,
        playerIds: normalized.playerIds,
        matchId: normalized.matchId,
        status: normalized.status,
        canConfirm: normalized.status === "waiting" && (normalized.playerIds?.length ?? normalized.players) >= 2,
      });
      setRooms([normalized]);
      if (normalized.matchId) {
        navigate(`/game/${id}`);
        return;
      }
      if (normalized.readyPlayers && normalized.readyPlayers >= normalized.maxPlayers) {
        setInfoMessage("Все подтвердили вход. Запускаем игру...");
      }
    });

    return () => {
      offRoomUpdate();
      wsClient.disconnect();
    };
  }, [id, navigate]);

  useEffect(() => {
    if (!id || !isWaitingStart) {
      return;
    }
    const interval = window.setInterval(() => {
      void getRoom(id!)
        .then((nextRoom: Room) => {
          setRooms([nextRoom]);
          if (nextRoom.matchId) {
            navigate(`/game/${id}`);
          }
        })
        .catch(() => undefined);
    }, 1500);
    return () => window.clearInterval(interval);
  }, [id, isWaitingStart, navigate]);

  return (
    <section className="screen">
      <div className="page-header">
        <Link className="icon-button" to="/play">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">Комната</h1>
        <div className="page-header__spacer" />
      </div>
      <p className="screen__subtitle">Перед стартом подтвердите участие в игре.</p>

      {isLoading && <CardSkeleton rows={4} />}

      {error && (
        <ErrorStateBlock
          title="Комната недоступна"
          message={error}
          actionLabel="Вернуться в игры"
          onAction={() => navigate("/play")}
        />
      )}

      {!isLoading && !error && !room && (
        <EmptyStateBlock
          title="Комната не найдена"
          message="Возможно, стол уже завершен или был удален."
          actionLabel="К списку игр"
          onAction={() => navigate("/play")}
        />
      )}

      {room && room.status === "cancelled" && (
        <EmptyStateBlock
          title="Комната отменена"
          message="Игра не состоялась. Создайте новую или выберите другую комнату."
          actionLabel="К списку игр"
          onAction={() => navigate("/play")}
        />
      )}

      {room && room.status !== "cancelled" && (
        <>
          {infoMessage && <div className="card__hint">{infoMessage}</div>}
          <AppCard className="room-card room-card--detail">
            <div className="room-card__top">
              <strong className="room-card__stake">${room.stakeUsd}</strong>
              <span className="room-card__badge">
                {room.players}/{room.maxPlayers}
              </span>
            </div>
            <div className="room-card__title">{room.title}</div>
            <div className="room-card__meta">
              <span>{room.mode}</span>
              <span>{room.deck} карт</span>
            </div>
          </AppCard>

          <AppCard>
            <div className="card__label">Условия</div>
            <div className="card__hint">Тестовый режим: игра без списаний и выплат.</div>
            <div className="card__hint">Если не подтвердить вход — игра не стартует.</div>
            <div className="card__hint">
              Готовы: {room.readyPlayers ?? 0}/{room.players}
            </div>
          </AppCard>

          <AppCard>
            <div className="card__label">Пригласить</div>
            <AppButton
              type="button"
              variant="ghost"
              onClick={async () => {
                const url = new URL(`/room/${id}`, window.location.origin).href;
                const text = `Присоединяйся к игре в дурака: ${room.title}`;
                try {
                  await navigator.share({ title: "Дурак Онлайн", text, url });
                  hapticNotification("success");
                  setInfoMessage("Приглашение отправлено");
                } catch {
                  await navigator.clipboard.writeText(url);
                  hapticNotification("success");
                  setInfoMessage("Ссылка скопирована");
                }
              }}
            >
              {"share" in navigator ? "Поделиться" : "Скопировать ссылку"}
            </AppButton>
          </AppCard>

          <div className="action-list">
            <AppButton
              variant="primary"
              type="button"
              onClick={() => {
                hapticImpact("medium");
                void confirmAndStart();
              }}
              disabled={isReadyLoading || !canConfirm}
            >
              {isReadyLoading ? "Подключаем..." : "Подтвердить и начать"}
            </AppButton>
            <AppButton type="button" onClick={() => setIsLeaveModalOpen(true)}>
              Покинуть комнату
            </AppButton>
            <Link className="button" to="/play">
              Вернуться в список
            </Link>
          </div>
        </>
      )}

      <ConfirmModal
        isOpen={isLeaveModalOpen}
        title="Покинуть комнату?"
        message="Другие игроки не смогут начать игру без вас."
        confirmLabel="Покинуть"
        onConfirm={() => {
          if (!id) return;
          setIsLeaveModalOpen(false);
          wsClient.disconnect();
          void leaveRoom(id).then(() => navigate("/play")).catch(() => navigate("/play"));
        }}
        onCancel={() => setIsLeaveModalOpen(false)}
      />
    </section>
  );
}
