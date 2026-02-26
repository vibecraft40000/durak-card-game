import { emitWsEvent } from "@/shared/api/ws/events";
import type { ClientIntent, ClientWsEvent, IntentType, ServerWsEvent } from "@/shared/api/ws/types";
import { getAccessToken } from "@/shared/api/auth";
import { getGameState, setInteractionLocked } from "@/store/game.store";
import { mapServerErrorToMessage } from "@/shared/utils/mapServerErrorToMessage";

const RAW_WS_URL = import.meta.env.VITE_WS_URL ?? "/ws";

// Prefer same-origin WS on production domain to avoid localhost leaks in builds.
const WS_URL =
  typeof window !== "undefined" &&
  (window.location.origin === "https://durakonline.duckdns.org" ||
    window.location.origin === "https://www.durakonline.duckdns.org")
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

  connect(roomId: string) {
    if (this.socket && this.connectedRoomId === roomId) {
      return;
    }

    this.disconnect();
    this.connectedRoomId = roomId;
    const token = getAccessToken();
    const url = resolveWsUrl(WS_URL);
    if (token) {
      url.searchParams.set("token", token);
    }
    url.searchParams.set("roomId", roomId);
    this.socket = new WebSocket(url.toString());

    this.socket.addEventListener("open", () => {
      const isReconnect = this.reconnectAttempts > 0;
      this.reconnectAttempts = 0;
      connectionHandler?.("connect");
      this.send({
        type: isReconnect ? "reconnect" : "join_room",
        payload: { roomId },
      });
      if (isReconnect) {
        setTimeout(() => this.send({ type: "sync_request", payload: { roomId } }), 100);
      }
    });

    this.socket.addEventListener("message", (event) => {
      const parsed = tryParse(event.data);
      if (parsed) {
        emitWsEvent(parsed);

        // Попытка вытащить ошибку интента из серверного события и ретранслировать в UI
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
    });

    this.socket.addEventListener("close", () => {
      const roomToReconnect = this.connectedRoomId;
      this.socket = null;
      if (roomToReconnect) {
        connectionHandler?.("disconnect");
      }
      if (!roomToReconnect) {
        return;
      }
      const delay = Math.min(1000 * 2 ** this.reconnectAttempts, 10000);
      this.reconnectAttempts += 1;
      window.setTimeout(() => {
        if (this.connectedRoomId !== roomToReconnect) {
          return;
        }
        this.connect(roomToReconnect);
      }, delay);
    });
  }

  send(event: ClientWsEvent) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return;
    }
    this.socket.send(JSON.stringify(event));
  }

  sendIntent(type: IntentType, payload?: ClientIntent["payload"]) {
    const gameState = getGameState();
    const version = gameState.matchState?.version ?? 0;
    const payloadObj = (payload ?? {}) as Record<string, unknown>;
    const roomId =
      (typeof payloadObj.roomId === "string" && payloadObj.roomId) || this.connectedRoomId;
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
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
    this.connectedRoomId = null;
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

function mapIntentToWireAction(type: IntentType):
  | "attack_card"
  | "defend_card"
  | "take_cards"
  | "pass_turn"
  | null {
  switch (type) {
    case "playAttack":
    case "throwIn":
    case "translate":
      return "attack_card";
    case "playDefend":
      return "defend_card";
    case "take":
      return "take_cards";
    case "pass":
    case "endTurn":
    case "confirmStake":
    case "shulerReport":
      return "pass_turn";
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
