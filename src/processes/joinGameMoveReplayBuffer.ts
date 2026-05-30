import type { ServerWsEvent } from "@/shared/api/ws/types";

type MoveAppliedPayload = Extract<ServerWsEvent, { type: "move_applied" }>["payload"];

type BufferedMove = {
  eventId: string;
  playerId: string;
  action: string;
  cardId?: string;
  receivedAt: number;
};

type ReplayDebugAdapter = {
  updatePending(versionedCount: number, unversionedCount: number): void;
  incDroppedDuplicate(): void;
  incDroppedStale(): void;
  incFlushed(): void;
  incBufferedVersioned(): void;
  incBufferedUnversioned(): void;
};

type CreateJoinGameMoveReplayBufferOptions = {
  getCurrentVersion: () => number | undefined;
  hasMoveEventIdInActivity: (eventId: string) => boolean;
  addBufferedMove: (move: BufferedMove) => void;
  replayDebug: ReplayDebugAdapter;
};

const MAX_SEEN_MOVE_IDS = 256;
const MAX_PENDING_UNVERSIONED = 24;

function parseVersionFromEventId(eventId?: string): number | null {
  if (!eventId) {
    return null;
  }

  const markerIndex = eventId.lastIndexOf(":v");
  if (markerIndex < 0 || markerIndex + 2 >= eventId.length) {
    return null;
  }

  const parsed = Number.parseInt(eventId.slice(markerIndex + 2), 10);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }

  return parsed;
}

export function createJoinGameMoveReplayBuffer(
  options: CreateJoinGameMoveReplayBufferOptions,
) {
  const pendingMovesByVersion = new Map<number, BufferedMove>();
  const pendingUnversionedMoves: BufferedMove[] = [];
  const seenMoveEventIds = new Set<string>();
  const seenMoveEventIdQueue: string[] = [];

  const rememberMoveEventId = (eventId: string) => {
    if (seenMoveEventIds.has(eventId)) {
      return;
    }

    seenMoveEventIds.add(eventId);
    seenMoveEventIdQueue.push(eventId);

    if (seenMoveEventIdQueue.length > MAX_SEEN_MOVE_IDS) {
      const stale = seenMoveEventIdQueue.shift();
      if (stale) {
        seenMoveEventIds.delete(stale);
      }
    }
  };

  const publishPendingReplayWindow = () => {
    options.replayDebug.updatePending(
      pendingMovesByVersion.size,
      pendingUnversionedMoves.length,
    );
  };

  const flush = () => {
    const currentVersion = options.getCurrentVersion();

    if (typeof currentVersion === "number") {
      const readyVersions = Array.from(pendingMovesByVersion.keys())
        .filter((version) => version <= currentVersion)
        .sort((left, right) => left - right);

      for (const version of readyVersions) {
        const buffered = pendingMovesByVersion.get(version);
        if (!buffered) {
          continue;
        }

        pendingMovesByVersion.delete(version);

        if (
          seenMoveEventIds.has(buffered.eventId) ||
          options.hasMoveEventIdInActivity(buffered.eventId)
        ) {
          options.replayDebug.incDroppedDuplicate();
          rememberMoveEventId(buffered.eventId);
          continue;
        }

        options.addBufferedMove(buffered);
        options.replayDebug.incFlushed();
        rememberMoveEventId(buffered.eventId);
      }
    }

    while (pendingUnversionedMoves.length > 0) {
      const buffered = pendingUnversionedMoves.shift();
      if (!buffered) {
        continue;
      }

      if (
        seenMoveEventIds.has(buffered.eventId) ||
        options.hasMoveEventIdInActivity(buffered.eventId)
      ) {
        options.replayDebug.incDroppedDuplicate();
        rememberMoveEventId(buffered.eventId);
        continue;
      }

      options.addBufferedMove(buffered);
      options.replayDebug.incFlushed();
      rememberMoveEventId(buffered.eventId);
    }

    publishPendingReplayWindow();
  };

  const handleMoveApplied = (payload: MoveAppliedPayload) => {
    const eventId = payload.eventId ?? `${payload.matchId}-${Date.now()}`;

    if (
      seenMoveEventIds.has(eventId) ||
      options.hasMoveEventIdInActivity(eventId)
    ) {
      options.replayDebug.incDroppedDuplicate();
      rememberMoveEventId(eventId);
      return;
    }

    const eventVersion = parseVersionFromEventId(payload.eventId);
    const currentVersion = options.getCurrentVersion();
    if (
      typeof eventVersion === "number" &&
      typeof currentVersion === "number" &&
      eventVersion <= currentVersion
    ) {
      options.replayDebug.incDroppedStale();
      rememberMoveEventId(eventId);
      return;
    }

    const buffered: BufferedMove = {
      eventId,
      playerId: payload.playerId,
      action: payload.action,
      cardId: payload.cardId,
      receivedAt: Date.now(),
    };

    if (typeof eventVersion === "number") {
      if (!pendingMovesByVersion.has(eventVersion)) {
        pendingMovesByVersion.set(eventVersion, buffered);
        options.replayDebug.incBufferedVersioned();
      }
    } else {
      pendingUnversionedMoves.push(buffered);
      options.replayDebug.incBufferedUnversioned();

      if (pendingUnversionedMoves.length > MAX_PENDING_UNVERSIONED) {
        pendingUnversionedMoves.shift();
      }
    }

    publishPendingReplayWindow();
    flush();
  };

  return {
    flush,
    handleMoveApplied,
  };
}
