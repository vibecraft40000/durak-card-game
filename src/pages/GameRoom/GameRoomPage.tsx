import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import type { Room } from "@/entities/match/types";
import { HttpError } from "@/shared/api/http";
import { getRoom, joinRoom, leaveRoom, normalizeRoom, readyRoom, startRoom } from "@/shared/api/rooms";
import { getProfile } from "@/shared/api/user";
import { onWsEvent } from "@/shared/api/ws/events";
import { wsClient } from "@/shared/api/ws/socket";
import { hapticImpact, hapticNotification } from "@/shared/lib/telegram";
import { BackIcon } from "@/shared/ui/Icons";
import { CardSkeleton, ConfirmModal, EmptyStateBlock, ErrorStateBlock } from "@/shared/ui/StateBlocks";
import { AppCard } from "@/shared/ui/Card";
import { AppButton } from "@/shared/ui/Button";

function formatApiError(e: unknown, fallback = "Попробуйте снова."): string {
  if (e instanceof HttpError) {
    if (e.status === 401)
      return "Ошибка авторизации. Откройте приложение заново из Telegram.";
    const body = String(e.responseBody ?? e.message ?? "").toLowerCase();
    if (body.includes("insufficient balance"))
      return "Недостаточно средств у одного из игроков. Пополните баланс.";
    if (body.includes("match start already in progress"))
      return "Старт уже выполняется. Подождите несколько секунд и попробуйте снова.";
    return body ? String(e.responseBody ?? e.message) : fallback;
  }
  return fallback;
}

export function GameRoomPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [rooms, setRooms] = useState<Room[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [infoMessage, setInfoMessage] = useState<string | null>(null);
  const [isReadyLoading, setIsReadyLoading] = useState(false);
  const [isStartLoading, setIsStartLoading] = useState(false);
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
  const isCreator = room && currentUserId && room.playerIds?.[0] === currentUserId;
  const allReady = room && (room.readyPlayers ?? 0) >= (room.players ?? room.maxPlayers ?? 2);
  const canConfirm = Boolean(
    room &&
      room.status === "waiting" &&
      playersCount >= 2 &&
      !isCurrentUserConfirmed,
  );
  const canStart = Boolean(
    room && room.status === "waiting" && isCreator && allReady && playersCount >= 2,
  );

  async function confirmReady() {
    if (!id) return;
    if (isReadyLoading) return;
    if (!room || room.players < 2) {
      setInfoMessage("Нельзя начать: в комнате пока нет соперника.");
      return;
    }
    setIsReadyLoading(true);
    setError(null);
    setInfoMessage(null);

    try {
      wsClient.send({ type: "confirm_join", payload: { roomId: id } });
      const updatedRoom = await readyRoom(id);
      setRooms([updatedRoom]);
      setInfoMessage("Подтверждение отправлено...");
      setIsWaitingStart(true);
      if (updatedRoom.matchId) {
        navigate(`/game/${id}`);
      }
    } catch (e) {
      const msg = formatApiError(e, "Не удалось подтвердить готовность. Проверьте подключение и попробуйте снова.");
      setError(msg);
    } finally {
      setIsReadyLoading(false);
    }
  }

  async function handleStart() {
    if (!id) return;
    if (isStartLoading) return;
    if (!room || room.players < 2 || !allReady) {
      setInfoMessage("Сначала все должны подтвердить участие.");
      return;
    }
    setIsStartLoading(true);
    setError(null);
    setInfoMessage(null);

    try {
      wsClient.send({ type: "start_game", payload: { roomId: id } });
      const updatedRoom = await startRoom(id);
      setRooms([updatedRoom]);
      if (updatedRoom.matchId) {
        navigate(`/game/${id}`);
      }
    } catch (e) {
      const msg = formatApiError(e, "Не удалось начать игру. Попробуйте снова.");
      setError(msg);
    } finally {
      setIsStartLoading(false);
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
      setRooms([normalized]);
      if (normalized.matchId) {
        navigate(`/game/${id}`);
        return;
      }
      if (normalized.readyPlayers && normalized.readyPlayers >= normalized.maxPlayers && !normalized.matchId) {
        const creatorId = normalized.playerIds?.[0];
        const amCreator = creatorId === currentUserId;
        setInfoMessage(amCreator ? "Все подтвердили. Нажмите «Начать»." : "Все подтвердили. Ждём запуска от создателя стола.");
      }
    });

    return () => {
      offRoomUpdate();
      wsClient.disconnect();
    };
  }, [id, navigate, currentUserId]);

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
            <div className="card__hint">Сначала оба подтверждают, затем создатель стола нажимает «Начать».</div>
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
                const botUsername = import.meta.env.VITE_TELEGRAM_BOT_USERNAME ?? "durakton777_bot";
                const url = `https://t.me/${botUsername}/app?startapp=room_${id ?? ""}`;
                const text = `Присоединяйся к игре в дурака: ${room.title}`;
                try {
                  await navigator.share({ title: "Дурак Онлайн", text, url });
                  hapticNotification("success");
                  setInfoMessage("Приглашение отправлено");
                } catch {
                  await navigator.clipboard.writeText(url);
                  setInfoMessage("Ссылка скопирована");
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
                void confirmReady();
              }}
              disabled={isReadyLoading || !canConfirm}
            >
              {isReadyLoading ? "Подключаем..." : "Подтвердить"}
            </AppButton>
            {canStart && (
              <AppButton
                variant="primary"
                type="button"
                onClick={() => {
                  hapticImpact("medium");
                  void handleStart();
                }}
                disabled={isStartLoading}
              >
                {isStartLoading ? "Запускаем..." : "Начать"}
              </AppButton>
            )}
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
