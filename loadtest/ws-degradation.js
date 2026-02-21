import http from "k6/http";
import ws from "k6/ws";
import { Counter, Trend } from "k6/metrics";

const moveLatency = new Trend("move_latency_ms");
const moveLatencyP99Track = new Trend("move_latency_p99_track_ms");
const holdDurationSec = new Trend("hold_duration_sec");
const wsErrorsTotal = new Counter("ws_errors_total");
const wsUnexpectedCloses = new Counter("ws_unexpected_closes");
const wsConnectFailuresTotal = new Counter("ws_connect_failures_total");
const wsTransportErrorsTotal = new Counter("ws_transport_errors_total");
const reconnectAttemptsTotal = new Counter("reconnect_attempts_total");
const reconnectSuccessTotal = new Counter("reconnect_success_total");
const moveSentTotal = new Counter("move_sent_total");
const moveAckTotal = new Counter("move_ack_total");
const moveTimeoutsTotal = new Counter("move_timeouts_total");
const redisOpsTotal = new Counter("redis_ops_total");

export const options = {
  summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)"],
  scenarios: {
    ws_degradation: {
      executor: "per-vu-iterations",
      vus: Number(__ENV.VUS || "1200"),
      iterations: 1,
      maxDuration: __ENV.MAX_DURATION || "15m",
    },
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const WS_URL = __ENV.WS_URL || "ws://localhost:8080/ws";
const TOKEN = __ENV.WS_TOKEN || "";
const ROOM_ID = __ENV.ROOM_ID || "load-room";
const HOLD_MIN_SEC = Number(__ENV.HOLD_MIN_SEC || "10");
const HOLD_MAX_SEC = Number(__ENV.HOLD_MAX_SEC || "30");
const MOVE_TIMEOUT_MS = Number(__ENV.MOVE_TIMEOUT_MS || "2000");
const MOVE_INTERVAL_MS = Number(__ENV.MOVE_INTERVAL_MS || "20");

function randInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

function readRedisOpsCount() {
  const res = http.get(`${BASE_URL}/metrics`);
  if (res.status !== 200) return 0;
  const lines = res.body.match(/^redis_latency_seconds_count\{.*\}\s+([0-9.eE+-]+)/gm) || [];
  return lines.reduce((acc, line) => {
    const value = Number(line.trim().split(" ").at(-1) || "0");
    return acc + value;
  }, 0);
}

function runSocketSession(url, roomId, durationMs) {
  ws.connect(url, {}, function (socket) {
    let opened = false;
    let connectFailureCounted = false;
    let hadError = false;
    const pending = [];
    let intervalRef;
    let timeoutRef;

    function markConnectFailureOnce() {
      if (!opened && !connectFailureCounted) {
        wsConnectFailuresTotal.add(1);
        connectFailureCounted = true;
      }
    }

    function sendMoveBurst() {
      const now = Date.now();
      if (pending.length > 0 && now - pending[0] >= MOVE_TIMEOUT_MS) {
        moveTimeoutsTotal.add(1);
        pending.shift();
      }
      pending.push(now);
      moveSentTotal.add(1);
      socket.send(
        JSON.stringify({
          type: "make_move",
          payload: { roomId, action: "pass" },
        }),
      );
    }

    socket.on("open", function () {
      opened = true;
      socket.send(JSON.stringify({ type: "join_room", payload: { roomId } }));
      sendMoveBurst();
      intervalRef = socket.setInterval(function () {
        sendMoveBurst();
      }, MOVE_INTERVAL_MS);
      timeoutRef = socket.setTimeout(function () {
        if (intervalRef) {
          socket.clearInterval(intervalRef);
        }
        socket.close();
      }, durationMs);
    });

    socket.on("message", function () {
      if (pending.length > 0) {
        const started = pending.shift();
        const d = Date.now() - started;
        moveLatency.add(d);
        moveLatencyP99Track.add(d);
        moveAckTotal.add(1);
      }
    });

    socket.on("error", function () {
      hadError = true;
      wsErrorsTotal.add(1);
      wsTransportErrorsTotal.add(1);
      markConnectFailureOnce();
    });

    socket.on("close", function (code) {
      if (intervalRef) {
        socket.clearInterval(intervalRef);
      }
      if (timeoutRef) {
        socket.clearTimeout(timeoutRef);
      }
      if (hadError || (code !== 1000 && code !== 1005)) {
        wsUnexpectedCloses.add(1);
      }
      markConnectFailureOnce();
    });
  });
}

export function setup() {
  const initialOps = readRedisOpsCount();
  return { initialOps, startedAt: Date.now() };
}

export default function () {
  const holdSec = randInt(HOLD_MIN_SEC, HOLD_MAX_SEC);
  holdDurationSec.add(holdSec);
  const totalDurationMs = holdSec * 1000;
  const split = Math.floor(totalDurationMs / 2);
  const firstDuration = Math.max(split, 1000);
  const secondDuration = Math.max(totalDurationMs - split, 1000);

  const url = `${WS_URL}?token=${encodeURIComponent(TOKEN)}&roomId=${encodeURIComponent(ROOM_ID)}`;
  runSocketSession(url, ROOM_ID, firstDuration);

  reconnectAttemptsTotal.add(1);
  runSocketSession(url, ROOM_ID, secondDuration);
  reconnectSuccessTotal.add(1);
}

export function teardown(data) {
  const finalOps = readRedisOpsCount();
  const opsTotal = Math.max(0, finalOps - data.initialOps);
  if (opsTotal > 0) {
    redisOpsTotal.add(opsTotal);
  }
  const elapsedSec = Math.max((Date.now() - data.startedAt) / 1000, 1);
  const opsPerSec = opsTotal / elapsedSec;
  console.log(`redis ops/sec (estimated): ${opsPerSec.toFixed(2)}`);
}
