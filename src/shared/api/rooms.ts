import type { CreateRoomInput, Room } from "@/entities/match/types";
import { httpRequest } from "@/shared/api/http";

type ListRoomsResponse = {
  rooms: Array<Record<string, unknown>>;
};

type CreateRoomResponse = {
  room: Record<string, unknown>;
};

type GetRoomResponse = {
  room: Record<string, unknown>;
};

type RoomMutationResponse = {
  ok: boolean;
  room: Record<string, unknown>;
};

export async function getRooms(signal?: AbortSignal): Promise<Room[]> {
  const response = await httpRequest<ListRoomsResponse>("/api/rooms", { signal });
  return response.rooms.map(normalizeRoom);
}

export async function createRoom(input: CreateRoomInput): Promise<Room> {
  const response = await httpRequest<CreateRoomResponse>("/api/rooms", {
    method: "POST",
    body: {
      title: input.title,
      stake: input.stakeUsd,
      maxPlayers: input.maxPlayers,
      deck: input.deck,
      mode: input.mode,
    },
  });
  return normalizeRoom(response.room);
}

export async function getRoom(roomId: string): Promise<Room> {
  const response = await httpRequest<GetRoomResponse>(`/api/rooms/${roomId}`);
  return normalizeRoom(response.room);
}

export async function joinRoom(roomId: string): Promise<Room> {
  const response = await httpRequest<RoomMutationResponse>(`/api/rooms/${roomId}/join`, {
    method: "POST",
  });
  return normalizeRoom(response.room);
}

export async function readyRoom(roomId: string): Promise<Room> {
  const response = await httpRequest<RoomMutationResponse>(`/api/rooms/${roomId}/ready`, {
    method: "POST",
  });
  return normalizeRoom(response.room);
}

export async function leaveRoom(roomId: string): Promise<Room> {
  const response = await httpRequest<RoomMutationResponse>(`/api/rooms/${roomId}/leave`, {
    method: "POST",
  });
  return normalizeRoom(response.room);
}

export function normalizeRoom(raw: Record<string, unknown>): Room {
  const playersRaw = raw.players;
  const playerIds = Array.isArray(playersRaw) ? playersRaw.map((item) => String(item)) : [];
  const playersCount =
    playerIds.length > 0 ? playerIds.length : (Number.isFinite(Number(raw.players)) ? Number(raw.players) : 0);
  const readyUsersRaw = raw.readyUsers ?? raw.ready_users;
  const readyUserIds = Array.isArray(readyUsersRaw) ? readyUsersRaw.map((item) => String(item)) : [];
  const readyCount =
    readyUserIds.length > 0
      ? readyUserIds.length
      : (Number.isFinite(Number(readyUsersRaw)) ? Number(readyUsersRaw) : 0);
  return {
    id: String(raw.id ?? ""),
    title: String(raw.title ?? "Стол"),
    stakeUsd: Number(raw.stakeUsd ?? raw.stake ?? 0),
    players: Number.isFinite(playersCount) ? playersCount : 0,
    playerIds,
    maxPlayers: Number(raw.maxPlayers ?? raw.max_players ?? 2),
    deck: Number(raw.deck ?? 36) as Room["deck"],
    mode: String(raw.mode ?? "Подкидной") as Room["mode"],
    status: String(raw.status ?? "waiting") as Room["status"],
    matchId: String(raw.matchId ?? raw.match_id ?? ""),
    readyPlayers: Number.isFinite(readyCount) ? readyCount : 0,
    readyUserIds,
  };
}
