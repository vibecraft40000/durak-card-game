import { useEffect, useState } from "react";
import type { Room } from "@/entities/match/types";
import { joinGameRoom } from "@/processes/joinGame.process";
import { getRoom } from "@/shared/api/rooms";
import { getProfile } from "@/shared/api/user";
import { getGameState, subscribeGameStore } from "@/store/game.store";

type GameTableState = ReturnType<typeof getGameState>;

export function useGameTableSession(roomId?: string) {
  const [room, setRoom] = useState<Room | null>(null);
  const [isRoomLoading, setIsRoomLoading] = useState(true);
  const [gameState, setGameState] = useState<GameTableState>(() => getGameState());
  const [currentUserId, setCurrentUserId] = useState<string | null>(null);
  const [profileBalance, setProfileBalance] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;

    void getProfile()
      .then((response) => {
        if (cancelled) {
          return;
        }

        setCurrentUserId(response.user.id);
        setProfileBalance(response.balance);
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!gameState.matchResult) {
      return;
    }

    let cancelled = false;

    void getProfile()
      .then((response) => {
        if (!cancelled) {
          setProfileBalance(response.balance);
        }
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, [gameState.matchResult]);

  useEffect(() => {
    return subscribeGameStore(setGameState);
  }, []);

  useEffect(() => {
    if (!roomId) {
      setRoom(null);
      setIsRoomLoading(false);
      return;
    }

    let cancelled = false;
    setIsRoomLoading(true);

    void getRoom(roomId)
      .then((data) => {
        if (!cancelled) {
          setRoom(data);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setRoom(null);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setIsRoomLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [roomId]);

  useEffect(() => {
    if (!roomId) {
      return;
    }

    let cleanup: (() => void) | undefined;
    let disposed = false;

    void joinGameRoom(roomId).then((dispose) => {
      if (disposed) {
        dispose();
        return;
      }

      cleanup = dispose;
    });

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, [roomId]);

  return {
    room,
    isRoomLoading,
    gameState,
    currentUserId,
    profileBalance,
  };
}
