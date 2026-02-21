import { emitWsEvent } from "@/shared/api/ws/events";
import type { ClientWsEvent, ServerWsEvent } from "@/shared/api/ws/types";
import { getAccessToken } from "@/shared/api/auth";

const WS_URL = import.meta.env.VITE_WS_URL ?? "/ws";

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
      this.send({
        type: isReconnect ? "reconnect" : "join_room",
        payload: { roomId },
      });
    });

    this.socket.addEventListener("message", (event) => {
      const parsed = tryParse(event.data);
      if (parsed) {
        emitWsEvent(parsed);
      }
    });

    this.socket.addEventListener("close", () => {
      const roomToReconnect = this.connectedRoomId;
      this.socket = null;
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

function resolveWsUrl(raw: string): URL {
  if (raw.startsWith("ws://") || raw.startsWith("wss://")) {
    return new URL(raw);
  }
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return new URL(raw, `${protocol}//${window.location.host}`);
}
