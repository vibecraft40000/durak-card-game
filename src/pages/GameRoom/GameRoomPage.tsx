import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import type { Room } from "@/entities/match/types";
import { HttpError } from "@/shared/api/http";
import {
  confirmStake as confirmStakeRoom,
  getRoom,
  joinRoom,
  leaveRoom,
  normalizeRoom,
  readyRoom,
  startRoom,
} from "@/shared/api/rooms";
import { getProfile } from "@/shared/api/user";
import { onWsEvent } from "@/shared/api/ws/events";
import { wsClient } from "@/shared/api/ws/socket";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { hapticImpact, hapticNotification } from "@/shared/lib/telegram";
import { BackIcon } from "@/shared/ui/Icons";
import { CardSkeleton, ConfirmModal, EmptyStateBlock, ErrorStateBlock } from "@/shared/ui/StateBlocks";
import { AppCard } from "@/shared/ui/Card";
import { AppButton } from "@/shared/ui/Button";

function formatApiError(
  e: unknown,
  t: (key: string, params?: Record<string, string | number>) => string,
  fallbackKey: string,
): string {
  if (e instanceof HttpError) {
    if (e.status === 401) {
      return t("room.error.auth");
    }
    const body = String(e.responseBody ?? e.message ?? "").toLowerCase();
    if (body.includes("insufficient balance")) {
      return t("room.error.insufficientBalance");
    }
    if (body.includes("match start already in progress")) {
      return t("room.error.startInProgress");
    }
    if (body.includes("stake confirmation is required")) {
      return t("room.error.stakeRequired");
    }
    if (body.includes("stake confirmation expired")) {
      return t("room.error.stakeExpired");
    }
    if (body.includes("only room participants can confirm stake")) {
      return t("room.error.notParticipant");
    }
    return body ? String(e.responseBody ?? e.message) : t(fallbackKey);
  }
  return t(fallbackKey);
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

export function GameRoomPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t, language } = useLanguage();

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
    void joinRoom(id)
      .then((roomData) => setRooms([roomData]))
      .catch(() => setError(t("room.error.join")))
      .finally(() => setIsLoading(false));
  }, [id, t]);

  const room = useMemo(() => rooms.find((item) => item.id === id), [id, rooms]);
  const isCurrentUserConfirmed = room && currentUserId ? room.readyUserIds.includes(currentUserId) : false;
  const isCurrentUserStakeConfirmed =
    room && currentUserId ? room.stakeConfirmedUserIds.includes(currentUserId) : false;
  const playersCount = room ? (room.playerIds?.length ?? room.players ?? 0) : 0;
  const realPlayersCount = room
    ? room.playerIds.filter((playerId) => !playerId.startsWith("bot:")).length || playersCount
    : 0;
  const isCreator = room && currentUserId && room.playerIds?.[0] === currentUserId;
  const allReady = room && (room.readyPlayers ?? 0) >= (room.players ?? room.maxPlayers ?? 2);
  const allStakeConfirmed = room && (room.stakeConfirmedPlayers ?? 0) >= realPlayersCount;
  const locale = language === "uk" ? "uk-UA" : "ru-RU";
  const stakeConfirmDeadlineLabel = room?.stakeConfirmDeadline
    ? new Date(room.stakeConfirmDeadline).toLocaleTimeString(locale, {
        hour: "2-digit",
        minute: "2-digit",
      })
    : null;
  const canConfirm = Boolean(room && room.status === "waiting" && playersCount >= 2 && !isCurrentUserConfirmed);
  const canConfirmStake = Boolean(
    room &&
      room.status === "awaiting_stake_confirm" &&
      playersCount >= 2 &&
      !isCurrentUserStakeConfirmed,
  );
  const canStart = Boolean(room && room.status === "waiting" && isCreator && allReady && playersCount >= 2);

  async function confirmReady() {
    if (!id) return;
    if (isReadyLoading) return;
    if (!room || room.players < 2) {
      setInfoMessage(t("room.noOpponent"));
      return;
    }
    setIsReadyLoading(true);
    setError(null);
    setInfoMessage(null);

    try {
      wsClient.send({ type: "confirm_join", payload: { roomId: id } });
      const updatedRoom = await readyRoom(id);
      setRooms([updatedRoom]);
      setInfoMessage(t("room.readySent"));
      setIsWaitingStart(true);
      if (updatedRoom.matchId) {
        navigate(`/game/${id}`);
      }
    } catch (e) {
      const msg = formatApiError(e, t, "room.error.ready");
      setError(msg);
    } finally {
      setIsReadyLoading(false);
    }
  }

  async function confirmStake() {
    if (!id || !room || room.status !== "awaiting_stake_confirm") return;
    if (isReadyLoading) return;
    setIsReadyLoading(true);
    setError(null);
    setInfoMessage(null);

    try {
      wsClient.send({ type: "confirm_stake", payload: { roomId: id } });
      const updatedRoom = await confirmStakeRoom(id);
      setRooms([updatedRoom]);
      setIsWaitingStart(true);
      if (updatedRoom.matchId) {
        navigate(`/game/${id}`);
        return;
      }
      setInfoMessage(t("room.stakeConfirmed"));
    } catch (e) {
      const msg = formatApiError(e, t, "room.error.stake");
      setError(msg);
    } finally {
      setIsReadyLoading(false);
    }
  }

  async function handleStart() {
    if (!id) return;
    if (isStartLoading) return;
    if (room?.status === "awaiting_stake_confirm") {
      setInfoMessage(t("room.needStakeBeforeStart"));
      return;
    }
    if (!room || room.players < 2 || !allReady) {
      setInfoMessage(t("room.needAllReady"));
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
      const msg = formatApiError(e, t, "room.error.start");
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
      if (normalized.status === "awaiting_stake_confirm") {
        const realPlayers =
          normalized.playerIds.filter((playerId) => !playerId.startsWith("bot:")).length ||
          normalized.players;
        const confirmed = normalized.stakeConfirmedPlayers ?? 0;
        if (realPlayers > 0) {
          if (normalized.stakeConfirmDeadline) {
            const leftSec = Math.max(0, Math.ceil((normalized.stakeConfirmDeadline - Date.now()) / 1000));
            setInfoMessage(
              t("room.stakeProgressLeft", {
                confirmed,
                total: realPlayers,
                seconds: leftSec,
              }),
            );
          } else {
            setInfoMessage(
              t("room.stakeProgress", {
                confirmed,
                total: realPlayers,
              }),
            );
          }
        }
        return;
      }
      if (normalized.readyPlayers && normalized.readyPlayers >= normalized.maxPlayers && !normalized.matchId) {
        const creatorId = normalized.playerIds?.[0];
        const amCreator = creatorId === currentUserId;
        setInfoMessage(amCreator ? t("room.readyAllCreator") : t("room.readyAllWaitCreator"));
      }
    });

    return () => {
      offRoomUpdate();
      wsClient.disconnect();
    };
  }, [currentUserId, id, navigate, t]);

  useEffect(() => {
    if (!id || !isWaitingStart) {
      return;
    }
    const interval = window.setInterval(() => {
      void getRoom(id)
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
        <h1 className="page-header__title">{t("room.title")}</h1>
        <div className="page-header__spacer" />
      </div>
      <p className="screen__subtitle">{t("room.subtitle")}</p>

      {isLoading && <CardSkeleton rows={4} />}

      {error && (
        <ErrorStateBlock
          title={t("room.unavailableTitle")}
          message={error}
          actionLabel={t("room.unavailableAction")}
          onAction={() => navigate("/play")}
        />
      )}

      {!isLoading && !error && !room && (
        <EmptyStateBlock
          title={t("room.notFoundTitle")}
          message={t("room.notFoundMessage")}
          actionLabel={t("room.notFoundAction")}
          onAction={() => navigate("/play")}
        />
      )}

      {room && room.status === "cancelled" && (
        <EmptyStateBlock
          title={t("room.cancelledTitle")}
          message={t("room.cancelledMessage")}
          actionLabel={t("room.cancelledAction")}
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
              <span>{formatModeLabel(room.mode, t)}</span>
              <span>
                {room.deck} {t("common.cards")}
              </span>
            </div>
          </AppCard>

          <AppCard>
            <div className="card__label">{t("room.conditions")}</div>
            <div className="card__hint">{t("room.testModeHint")}</div>
            <div className="card__hint">{t("room.flowHint")}</div>
            <div className="card__hint">
              {t("room.readyLabel", {
                ready: room.readyPlayers ?? 0,
                players: room.players,
              })}
            </div>
            {(room.status === "awaiting_stake_confirm" || (room.stakeConfirmedPlayers ?? 0) > 0) && (
              <div className="card__hint">
                {t("room.stakeLabel", {
                  confirmed: room.stakeConfirmedPlayers ?? 0,
                  players: realPlayersCount,
                })}
              </div>
            )}
            {room.status === "awaiting_stake_confirm" && stakeConfirmDeadlineLabel && (
              <div className="card__hint">{t("room.stakeDeadline", { time: stakeConfirmDeadlineLabel })}</div>
            )}
          </AppCard>

          <AppCard>
            <div className="card__label">{t("room.invite")}</div>
            <AppButton
              type="button"
              variant="ghost"
              onClick={async () => {
                const botUsername = import.meta.env.VITE_TELEGRAM_BOT_USERNAME ?? "durakton777_bot";
                const url = `https://t.me/${botUsername}/app?startapp=room_${id ?? ""}`;
                const text = t("room.inviteText", { title: room.title });
                try {
                  await navigator.share({ title: "Дурак Онлайн", text, url });
                  hapticNotification("success");
                  setInfoMessage(t("room.inviteSent"));
                } catch {
                  await navigator.clipboard.writeText(url);
                  setInfoMessage(t("room.linkCopied"));
                  hapticNotification("success");
                  setInfoMessage(t("room.linkCopied"));
                }
              }}
            >
              {"share" in navigator ? t("common.share") : t("common.copyLink")}
            </AppButton>
          </AppCard>

          <div className="action-list">
            {room.status === "awaiting_stake_confirm" ? (
              <AppButton
                variant="primary"
                type="button"
                onClick={() => {
                  hapticImpact("medium");
                  void confirmStake();
                }}
                disabled={isReadyLoading || !canConfirmStake}
              >
                {isReadyLoading
                  ? t("room.button.stakeLoading")
                  : isCurrentUserStakeConfirmed
                    ? t("room.button.stakeDone")
                    : t("room.button.stakeConfirm")}
              </AppButton>
            ) : (
              <AppButton
                variant="primary"
                type="button"
                onClick={() => {
                  hapticImpact("medium");
                  void confirmReady();
                }}
                disabled={isReadyLoading || !canConfirm}
              >
                {isReadyLoading
                  ? t("room.button.readyLoading")
                  : isCurrentUserConfirmed
                    ? t("room.button.readyDone")
                    : t("room.button.readyConfirm")}
              </AppButton>
            )}
            {room.status === "awaiting_stake_confirm" && !allStakeConfirmed && (
              <div className="card__hint">{t("room.waitStakeAll")}</div>
            )}
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
                {isStartLoading ? t("room.button.startLoading") : t("room.button.start")}
              </AppButton>
            )}
            <AppButton type="button" onClick={() => setIsLeaveModalOpen(true)}>
              {t("room.button.leave")}
            </AppButton>
            <Link className="button" to="/play">
              {t("room.button.backToList")}
            </Link>
          </div>
        </>
      )}

      <ConfirmModal
        isOpen={isLeaveModalOpen}
        title={t("room.leaveModal.title")}
        message={t("room.leaveModal.message")}
        confirmLabel={t("room.leaveModal.confirm")}
        onConfirm={() => {
          if (!id) return;
          setIsLeaveModalOpen(false);
          wsClient.disconnect();
          void leaveRoom(id)
            .then(() => navigate("/play"))
            .catch(() => navigate("/play"));
        }}
        onCancel={() => setIsLeaveModalOpen(false)}
      />
    </section>
  );
}
