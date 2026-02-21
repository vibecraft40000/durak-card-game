import type { ServerWsEvent } from "@/shared/api/ws/types";

type EventHandler<T extends ServerWsEvent["type"]> = (
  event: Extract<ServerWsEvent, { type: T }>,
) => void;

type RawEventHandler = (event: ServerWsEvent) => void;

const eventMap: Partial<Record<ServerWsEvent["type"], Set<RawEventHandler>>> = {};

export function onWsEvent<T extends ServerWsEvent["type"]>(
  type: T,
  handler: EventHandler<T>,
) {
  if (!eventMap[type]) {
    eventMap[type] = new Set();
  }
  const wrapped: RawEventHandler = (event) => {
    if (event.type === type) {
      handler(event as Extract<ServerWsEvent, { type: T }>);
    }
  };
  eventMap[type]?.add(wrapped);

  return () => {
    eventMap[type]?.delete(wrapped);
  };
}

export function emitWsEvent(event: ServerWsEvent) {
  eventMap[event.type]?.forEach((handler) => {
    handler(event as never);
  });
}
