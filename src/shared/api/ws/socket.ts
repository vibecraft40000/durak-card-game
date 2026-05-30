import { emitWsEvent } from "@/shared/api/ws/events";
import type { ClientWsEvent, IntentType, ServerWsEvent } from "@/shared/api/ws/types";
import { issueWsTicket } from "@/shared/api/ws/ticket";
import { getRuntimeLanguage } from "@/shared/i18n/runtime";
import { getGameState, setInteractionLocked } from "@/store/game.store";
import { mapServerErrorToMessage } from "@/shared/utils/mapServerErrorToMessage";

const RAW_WS_URL = import.meta.env.VITE_WS_URL ?? "/ws";
const WS_PACKET_DISORDER_ENABLED =
  import.meta.env.DEV && toBool(import.meta.env.VITE_WS_PACKET_DISORDER);
const WS_PACKET_DISORDER_MAX_DELAY_MS = toNonNegativeInt(
  import.meta.env.VITE_WS_PACKET_DISORDER_MAX_DELAY_MS,
  450,
);
const WS_PACKET_DUPLICATE_RATE = clamp01(
  toNumber(import.meta.env.VITE_WS_PACKET_DUPLICATE_RATE, 0),
);
let disorderConfigLogged = false;

// Prefer same-origin WS on production domain to avoid localhost leaks in builds.
const WS_URL =
  typeof window !== "undefined" &&
  (window.location.origin === "https://your-domain.example" ||
    window.location.origin === "https://www.your-domain.example")
    ? "/ws"
    : RAW_WS_URL;

type ConnectionHandler = (event: "disconnect" | "connect") => void;
let connectionHandler: ConnectionHandler | null = null;

export function setConnectionHandler(handler: ConnectionHandler | null) {
  connectionHandler = handler;
}

class WsClient {
  private socket: WebSocket | null = null;
  private connectedRoomId: string | null = null;
  private reconnectAttempts = 0;
  private connectAttemptId = 0;
  private isConnecting = false;

  connect(roomId: string) {
    if (this.connectedRoomId === roomId && (this.socket || this.isConnecting)) {
      return;
    }

    this.disconnect();
    this.connectedRoomId = roomId;
    const attemptId = ++this.connectAttemptId;
    this.isConnecting = true;
    void this.openSocket(roomId, attemptId);
  }

  send(event: ClientWsEvent) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return;
    }
    this.socket.send(JSON.stringify(event));
  }

  sendIntent(type: IntentType, payload?: Record<string, unknown>) {
    const gameState = getGameState();
    const version = gameState.matchState?.version ?? 0;
    const payloadObj = (payload ?? {}) as Record<string, unknown>;
    const roomId =
      (typeof payloadObj.roomId === "string" && payloadObj.roomId) || this.connectedRoomId;

    if (type === "confirmStake") {
      if (!roomId) {
        return;
      }
      this.send({
        type: "confirm_stake",
        payload: { roomId },
      });
      return;
    }

    const action = mapIntentToWireAction(type);
    if (!roomId || !action) {
      return;
    }
    const cardId = typeof payloadObj.cardId === "string" ? payloadObj.cardId : undefined;
    const actionId = crypto.randomUUID();

    setInteractionLocked(true);
    this.send({
      type: "make_move",
      payload: {
        roomId,
        action,
        ...(cardId ? { cardId } : {}),
        expectedVersion: version,
        actionId,
      },
    });
  }

  disconnect() {
    this.connectAttemptId += 1;
    this.isConnecting = false;
    const socket = this.socket;
    this.socket = null;
    this.connectedRoomId = null;
    if (socket) {
      socket.close();
    }
  }

  private async openSocket(roomId: string, attemptId: number): Promise<void> {
    try {
      const ticket = await issueWsTicket(roomId);
      if (!this.isAttemptCurrent(roomId, attemptId)) {
        return;
      }

      const url = resolveWsUrl(WS_URL);
      url.searchParams.set("ticket", ticket);
      url.searchParams.set("roomId", roomId);
      url.searchParams.set("locale", getRuntimeLanguage());

      const socket = new WebSocket(url.toString());
      if (!this.isAttemptCurrent(roomId, attemptId)) {
        socket.close();
        return;
      }

      this.socket = socket;
      this.attachSocket(socket, roomId, attemptId);
    } catch {
      if (!this.isAttemptCurrent(roomId, attemptId)) {
        return;
      }
      this.isConnecting = false;
      connectionHandler?.("disconnect");
      this.scheduleReconnect(roomId, attemptId);
    }
  }

  private attachSocket(socket: WebSocket, roomId: string, attemptId: number): void {
    socket.addEventListener("open", () => {
      if (!this.isCurrentSocket(socket, roomId, attemptId)) {
        socket.close();
        return;
      }

      logDisorderConfigOnce();
      this.isConnecting = false;
      const isReconnect = this.reconnectAttempts > 0;
      this.reconnectAttempts = 0;
      connectionHandler?.("connect");
      const knownVersion = getGameState().matchState?.version;
      const knownMatchId = getGameState().matchState?.matchId;
      this.send({
        type: isReconnect ? "reconnect" : "join_room",
        payload: {
          roomId,
          ...(typeof knownVersion === "number" ? { lastKnownVersion: knownVersion } : {}),
          ...(typeof knownMatchId === "string" && knownMatchId
            ? { lastKnownMatchId: knownMatchId }
            : {}),
          supportsStateDiff: true,
        },
      });
    });

    socket.addEventListener("message", (event) => {
      if (!this.isCurrentSocket(socket, roomId, attemptId)) {
        return;
      }
      const parsed = tryParse(event.data);
      if (parsed) {
        scheduleIncomingEvent(parsed);
      }
    });

    socket.addEventListener("close", () => {
      if (!this.isAttemptCurrent(roomId, attemptId)) {
        return;
      }
      if (this.socket === socket) {
        this.socket = null;
      }
      this.isConnecting = false;
      connectionHandler?.("disconnect");
      this.scheduleReconnect(roomId, attemptId);
    });
  }

  private scheduleReconnect(roomId: string, attemptId: number): void {
    if (!this.isAttemptCurrent(roomId, attemptId)) {
      return;
    }
    const delay = Math.min(1000 * 2 ** this.reconnectAttempts, 10000);
    this.reconnectAttempts += 1;
    window.setTimeout(() => {
      if (!this.isAttemptCurrent(roomId, attemptId)) {
        return;
      }
      this.connect(roomId);
    }, delay);
  }

  private isAttemptCurrent(roomId: string, attemptId: number): boolean {
    return this.connectedRoomId === roomId && this.connectAttemptId === attemptId;
  }

  private isCurrentSocket(socket: WebSocket, roomId: string, attemptId: number): boolean {
    return this.socket === socket && this.isAttemptCurrent(roomId, attemptId);
  }
}

function tryParse(data: string): ServerWsEvent | null {
  try {
    return JSON.parse(data) as ServerWsEvent;
  } catch {
    return null;
  }
}

export const wsClient = new WsClient();

function scheduleIncomingEvent(event: ServerWsEvent): void {
  if (!WS_PACKET_DISORDER_ENABLED || typeof window === "undefined") {
    dispatchIncomingEvent(event);
    return;
  }
  const delayMs = randomDelayMs();
  window.setTimeout(() => {
    dispatchIncomingEvent(event);
  }, delayMs);
  if (WS_PACKET_DUPLICATE_RATE > 0 && Math.random() < WS_PACKET_DUPLICATE_RATE) {
    const duplicateDelayMs = randomDelayMs();
    window.setTimeout(() => {
      dispatchIncomingEvent(event);
    }, duplicateDelayMs);
  }
}

function dispatchIncomingEvent(parsed: ServerWsEvent): void {
  emitWsEvent(parsed);

  try {
    const anyParsed = parsed as any;
    const errPayload =
      anyParsed?.error ||
      anyParsed?.intentError ||
      (anyParsed?.type === "intent_response" && anyParsed?.error) ||
      anyParsed?.intentResponse?.error;

    if (errPayload) {
      const mapped = mapServerErrorToMessage(errPayload);
      // eslint-disable-next-line no-console
      console.warn("[ws] intent error:", { raw: errPayload, mapped });

      window.dispatchEvent(
        new CustomEvent("tma:intentError", {
          detail: {
            raw: errPayload,
            text: mapped.text,
            code: mapped.code,
            original: anyParsed,
          },
        }),
      );
    }
  } catch (err) {
    // eslint-disable-next-line no-console
    console.error("[ws] error while handling intent error payload", err);
  }
}

function randomDelayMs(): number {
  if (WS_PACKET_DISORDER_MAX_DELAY_MS <= 0) {
    return 0;
  }
  return Math.floor(Math.random() * (WS_PACKET_DISORDER_MAX_DELAY_MS + 1));
}

function toBool(value: unknown): boolean {
  const normalized = String(value ?? "").trim().toLowerCase();
  return normalized === "1" || normalized === "true" || normalized === "on";
}

function toNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function toNonNegativeInt(value: unknown, fallback: number): number {
  const parsed = Math.floor(toNumber(value, fallback));
  return parsed >= 0 ? parsed : fallback;
}

function clamp01(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  if (value < 0) {
    return 0;
  }
  if (value > 1) {
    return 1;
  }
  return value;
}

function logDisorderConfigOnce(): void {
  if (!WS_PACKET_DISORDER_ENABLED || disorderConfigLogged) {
    return;
  }
  disorderConfigLogged = true;
  // eslint-disable-next-line no-console
  console.warn("[ws] packet disorder mode enabled", {
    maxDelayMs: WS_PACKET_DISORDER_MAX_DELAY_MS,
    duplicateRate: WS_PACKET_DUPLICATE_RATE,
  });
}

function mapIntentToWireAction(type: IntentType):
  | "attack_card"
  | "defend_card"
  | "throw_card"
  | "translate"
  | "take_cards"
  | "pass_turn"
  | "shuler_play"
  | "shuler_report"
  | null {
  switch (type) {
    case "playAttack":
      return "attack_card";
    case "throwIn":
      return "throw_card";
    case "translate":
      return "translate";
    case "playDefend":
      return "defend_card";
    case "take":
      return "take_cards";
    case "pass":
    case "endTurn":
      return "pass_turn";
    case "shulerPlay":
      return "shuler_play";
    case "shulerReport":
      return "shuler_report";
    default:
      return null;
  }
}

function resolveWsUrl(raw: string): URL {
  if (raw.startsWith("ws://") || raw.startsWith("wss://")) {
    return new URL(raw);
  }
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return new URL(raw, `${protocol}//${window.location.host}`);
}
