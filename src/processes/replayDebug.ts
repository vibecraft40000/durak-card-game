type ReplaySyncMode = "replay" | "snapshot" | "noop";

type ReplayDebugSnapshot = {
  roomId: string;
  bufferedVersioned: number;
  bufferedUnversioned: number;
  flushed: number;
  droppedDuplicate: number;
  droppedStale: number;
  pendingVersioned: number;
  pendingUnversioned: number;
  peakPendingVersioned: number;
  peakPendingUnversioned: number;
  syncRequests: number;
  syncReplay: number;
  syncSnapshot: number;
  syncNoop: number;
  lastStateVersion: number | null;
  warningCount: number;
  lastWarningAt: number | null;
  updatedAt: number;
};

const WINDOW_KEY = "__durakReplayDebug";
const MODE = String(import.meta.env.VITE_WS_REPLAY_DEBUG ?? "").trim().toLowerCase();
const ENABLED = MODE === "1" || MODE === "true" || MODE === "verbose";
const VERBOSE = MODE === "verbose";
const WARN_PENDING_VERSIONED = toNonNegativeInt(
  import.meta.env.VITE_WS_REPLAY_WARN_PENDING_VERSIONED,
  12,
);
const WARN_PENDING_UNVERSIONED = toNonNegativeInt(
  import.meta.env.VITE_WS_REPLAY_WARN_PENDING_UNVERSIONED,
  6,
);
const WARN_DROPPED_DUPLICATE = toNonNegativeInt(
  import.meta.env.VITE_WS_REPLAY_WARN_DROPPED_DUPLICATE,
  16,
);
const WARN_DROPPED_STALE = toNonNegativeInt(import.meta.env.VITE_WS_REPLAY_WARN_DROPPED_STALE, 10);
const WARN_INTERVAL_MS = toNonNegativeInt(import.meta.env.VITE_WS_REPLAY_WARN_INTERVAL_MS, 15_000);

function getStore(): Record<string, ReplayDebugSnapshot> {
  if (typeof window === "undefined") {
    return {};
  }
  const root = window as unknown as {
    [WINDOW_KEY]?: Record<string, ReplayDebugSnapshot>;
    __durakReplayDebugGet?: (roomId?: string) => Record<string, ReplayDebugSnapshot> | ReplayDebugSnapshot | null;
    __durakReplayDebugClear?: () => void;
  };
  root[WINDOW_KEY] = root[WINDOW_KEY] ?? {};
  if (typeof root.__durakReplayDebugGet !== "function") {
    root.__durakReplayDebugGet = (roomId?: string) => {
      const store = root[WINDOW_KEY] ?? {};
      if (!roomId) {
        return { ...store };
      }
      return store[roomId] ? { ...store[roomId] } : null;
    };
  }
  if (typeof root.__durakReplayDebugClear !== "function") {
    root.__durakReplayDebugClear = () => {
      root[WINDOW_KEY] = {};
    };
  }
  return root[WINDOW_KEY]!;
}

export class ReplayDebugTracker {
  private snapshot: ReplayDebugSnapshot;

  constructor(roomId: string) {
    this.snapshot = {
      roomId,
      bufferedVersioned: 0,
      bufferedUnversioned: 0,
      flushed: 0,
      droppedDuplicate: 0,
      droppedStale: 0,
      pendingVersioned: 0,
      pendingUnversioned: 0,
      peakPendingVersioned: 0,
      peakPendingUnversioned: 0,
      syncRequests: 0,
      syncReplay: 0,
      syncSnapshot: 0,
      syncNoop: 0,
      lastStateVersion: null,
      warningCount: 0,
      lastWarningAt: null,
      updatedAt: Date.now(),
    };
    this.publish("init");
  }

  setStateVersion(version: number): void {
    this.snapshot.lastStateVersion = version;
    this.publish("state");
  }

  incBufferedVersioned(): void {
    this.snapshot.bufferedVersioned += 1;
    this.publish("buffer:v");
  }

  incBufferedUnversioned(): void {
    this.snapshot.bufferedUnversioned += 1;
    this.publish("buffer:nov");
  }

  incFlushed(): void {
    this.snapshot.flushed += 1;
    this.publish("flush");
  }

  incDroppedDuplicate(): void {
    this.snapshot.droppedDuplicate += 1;
    this.publish("drop:dup");
  }

  incDroppedStale(): void {
    this.snapshot.droppedStale += 1;
    this.publish("drop:stale");
  }

  incSyncRequest(): void {
    this.snapshot.syncRequests += 1;
    this.publish("sync:req");
  }

  markSync(mode: ReplaySyncMode): void {
    if (mode === "replay") {
      this.snapshot.syncReplay += 1;
    } else if (mode === "snapshot") {
      this.snapshot.syncSnapshot += 1;
    } else {
      this.snapshot.syncNoop += 1;
    }
    this.publish("sync:" + mode);
  }

  updatePending(versioned: number, unversioned: number): void {
    this.snapshot.pendingVersioned = versioned;
    this.snapshot.pendingUnversioned = unversioned;
    if (versioned > this.snapshot.peakPendingVersioned) {
      this.snapshot.peakPendingVersioned = versioned;
    }
    if (unversioned > this.snapshot.peakPendingUnversioned) {
      this.snapshot.peakPendingUnversioned = unversioned;
    }
    this.publish("pending");
  }

  dispose(): void {
    if (!ENABLED || typeof window === "undefined") {
      return;
    }
    const store = getStore();
    store[this.snapshot.roomId] = { ...this.snapshot, updatedAt: Date.now() };
    if (VERBOSE) {
      // eslint-disable-next-line no-console
      console.debug("[ws-replay]", this.snapshot.roomId, "dispose", store[this.snapshot.roomId]);
    }
  }

  private publish(reason: string): void {
    this.snapshot.updatedAt = Date.now();
    this.maybeWarn(reason);
    if (!ENABLED || typeof window === "undefined") {
      return;
    }
    const store = getStore();
    store[this.snapshot.roomId] = { ...this.snapshot };
    if (VERBOSE) {
      // eslint-disable-next-line no-console
      console.debug("[ws-replay]", this.snapshot.roomId, reason, store[this.snapshot.roomId]);
    }
  }

  private maybeWarn(reason: string): void {
    if (!ENABLED || typeof window === "undefined") {
      return;
    }
    if (!this.hasAnyWarningSignal()) {
      return;
    }
    const now = Date.now();
    if (
      this.snapshot.lastWarningAt !== null &&
      now - this.snapshot.lastWarningAt < WARN_INTERVAL_MS
    ) {
      return;
    }
    this.snapshot.lastWarningAt = now;
    this.snapshot.warningCount += 1;
    // eslint-disable-next-line no-console
    console.warn("[ws-replay] disorder signal", {
      roomId: this.snapshot.roomId,
      reason,
      pendingVersioned: this.snapshot.pendingVersioned,
      pendingUnversioned: this.snapshot.pendingUnversioned,
      droppedDuplicate: this.snapshot.droppedDuplicate,
      droppedStale: this.snapshot.droppedStale,
      bufferedVersioned: this.snapshot.bufferedVersioned,
      bufferedUnversioned: this.snapshot.bufferedUnversioned,
      flushed: this.snapshot.flushed,
      lastStateVersion: this.snapshot.lastStateVersion,
      warningCount: this.snapshot.warningCount,
    });
  }

  private hasAnyWarningSignal(): boolean {
    return (
      this.snapshot.pendingVersioned >= WARN_PENDING_VERSIONED ||
      this.snapshot.pendingUnversioned >= WARN_PENDING_UNVERSIONED ||
      this.snapshot.droppedDuplicate >= WARN_DROPPED_DUPLICATE ||
      this.snapshot.droppedStale >= WARN_DROPPED_STALE
    );
  }
}

function toNonNegativeInt(value: unknown, fallback: number): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  const normalized = Math.floor(parsed);
  if (normalized < 0) {
    return fallback;
  }
  return normalized;
}
