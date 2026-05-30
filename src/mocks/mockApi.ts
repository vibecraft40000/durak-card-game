import type { Card } from "@/entities/card/types";
import type { CreateRoomInput, Room } from "@/entities/match/types";
import type { Player } from "@/entities/player/types";
import type { MatchStatePayload } from "@/shared/api/ws/types";

const MOCK_DELAY_MS = 180;
const MOCK_USER_ID = "local-user";
const MOCK_OPPONENT_ID = "bot-opponent";

type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

const mockRooms: Room[] = [
  {
    id: "room-101",
    title: "Быстрая игра",
    stakeUsd: 10,
    players: 2,
    playerIds: ["local-user", "bot-opponent"],
    maxPlayers: 2,
    deck: 36,
    mode: "Подкидной",
    readyUserIds: [],
    stakeConfirmedUserIds: [],
  },
  {
    id: "room-202",
    title: "Турнирный стол",
    stakeUsd: 50,
    players: 3,
    playerIds: ["p1", "p2", "p3"],
    maxPlayers: 4,
    deck: 52,
    mode: "Переводной",
    readyUserIds: [],
    stakeConfirmedUserIds: [],
  },
];

const mockMatchStates = new Map<string, MatchStatePayload>();

export function isMockApiEnabled() {
  if (import.meta.env.MODE === "production") {
    return false;
  }
  return String(import.meta.env.VITE_USE_MOCK_API).toLowerCase() === "true";
}

export async function mockHttpRequest(
  path: string,
  method: HttpMethod,
  body: unknown,
  signal?: AbortSignal,
) {
  await wait(MOCK_DELAY_MS, signal);

  if (method === "GET" && path === "/api/rooms") {
    return { rooms: mockRooms };
  }
  const getRoomMatch = path.match(/^\/api\/rooms\/([^/]+)$/);
  if (method === "GET" && getRoomMatch) {
    const room = mockRooms.find((entry) => entry.id === getRoomMatch[1]);
    if (!room) {
      throw new Error("Room not found");
    }
    return { room };
  }

  if (method === "POST" && path === "/auth/telegram") {
    return {
      user: {
        id: "local-user",
        telegram_id: 1,
        username: "local",
        referral_code: "LOCAL001",
      },
      accessToken: "mock-access-token",
      refreshToken: "mock-refresh-token",
    };
  }

  if (method === "POST" && path === "/auth/refresh") {
    return { accessToken: "mock-access-token" };
  }

  if (method === "POST" && path === "/api/ws-ticket") {
    return {
      ticket: `mock-ws-ticket-${Date.now()}`,
      expiresInSec: 30,
    };
  }

  if (method === "GET" && path === "/api/profile") {
    return {
      user: {
        id: "local-user",
        telegram_id: 1,
        username: "local",
        referral_code: "LOCAL001",
      },
      balance: 1000,
    };
  }

  if (method === "POST" && path === "/api/rooms") {
    const input = body as CreateRoomInput;
    const room: Room = {
      id: `room-${Date.now()}`,
      title: input.title || "Новый стол",
      stakeUsd: input.stakeUsd,
      players: 1,
      playerIds: [MOCK_USER_ID],
      maxPlayers: input.maxPlayers,
      deck: input.deck,
      mode: input.mode,
      readyUserIds: [],
      stakeConfirmedUserIds: [],
    };
    mockRooms.unshift(room);
    return { room };
  }

  const joinMatch = path.match(/^\/api\/rooms\/([^/]+)\/join$/);
  if (method === "POST" && joinMatch) {
    const roomId = joinMatch[1];
    const exists = mockRooms.some((room) => room.id === roomId);
    if (!exists) {
      throw new Error("Room not found");
    }
    return { ok: true };
  }

  const readyMatch = path.match(/^\/api\/rooms\/([^/]+)\/ready$/);
  if (method === "POST" && readyMatch) {
    const roomId = readyMatch[1];
    const room = mockRooms.find((entry) => entry.id === roomId);
    if (!room) {
      throw new Error("Room not found");
    }
    return { ok: true, room };
  }

  throw new Error(`Mock handler missing for ${method} ${path}`);
}

export function initializeMockMatch(roomId: string): MatchStatePayload {
  const existing = mockMatchStates.get(roomId);
  if (existing) {
    return existing;
  }

  const initial = {
    roomId,
    status: "playing",
    trumpSuit: "hearts",
    turnPlayerId: MOCK_USER_ID,
    turnEndsAt: Date.now() + 35_000,
    tableCards: [],
    players: [
      createPlayer(MOCK_USER_ID, "Вы", createInitialHand()),
      createPlayer(MOCK_OPPONENT_ID, "Bot", []),
    ],
  } as unknown as MatchStatePayload;

  mockMatchStates.set(roomId, initial);
  return initial;
}

export function applyMockMatchAction(params: {
  roomId: string;
  action: "attack" | "defend" | "take" | "pass";
  cardId?: string;
}) {
  const state = initializeMockMatch(params.roomId);
  const players = [...(state.players ?? [])];
  const me = players.find((player) => player.id === MOCK_USER_ID);
  const opponent = players.find((player) => player.id === MOCK_OPPONENT_ID);
  const tableCards = [...(state.tableCards ?? [])];

  if (!me || !opponent) {
    return state;
  }

  if ((params.action === "attack" || params.action === "defend") && params.cardId && me.hand) {
    const index = me.hand.findIndex((card: Card) => card.id === params.cardId);
    if (index >= 0) {
      const [card] = me.hand.splice(index, 1);
      tableCards.push(card);
    }
  }

  if (params.action === "take") {
    me.hand = [...(me.hand ?? []), ...tableCards];
    tableCards.length = 0;
  }

  if (params.action === "pass") {
    tableCards.length = 0;
  }

  me.handCount = me.hand?.length ?? me.handCount;
  opponent.handCount = Math.max(0, opponent.handCount - (params.action === "pass" ? 1 : 0));

  const nextTurn = state.turnPlayerId === MOCK_USER_ID ? MOCK_OPPONENT_ID : MOCK_USER_ID;
  const nextState: MatchStatePayload = {
    ...state,
    tableCards,
    players,
    turnPlayerId: nextTurn,
    turnEndsAt: Date.now() + 30_000,
    winnerPlayerId: me.handCount === 0 ? MOCK_USER_ID : undefined,
    status: me.handCount === 0 ? "finished" : "playing",
  };

  mockMatchStates.set(params.roomId, nextState);
  return nextState;
}

function createPlayer(id: string, username: string, hand: Card[]): Player {
  return {
    id,
    username,
    hand,
    handCount: hand.length,
    isCurrentTurn: id === MOCK_USER_ID,
  };
}

function createInitialHand(): Card[] {
  return [
    { id: "c1", suit: "hearts", rank: "A" },
    { id: "c2", suit: "spades", rank: "K" },
    { id: "c3", suit: "clubs", rank: "10" },
    { id: "c4", suit: "diamonds", rank: "Q" },
    { id: "c5", suit: "hearts", rank: "9" },
    { id: "c6", suit: "spades", rank: "8" },
  ];
}

function wait(ms: number, signal?: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    const timeout = window.setTimeout(() => resolve(), ms);
    signal?.addEventListener("abort", () => {
      window.clearTimeout(timeout);
      reject(new DOMException("Aborted", "AbortError"));
    });
  });
}
