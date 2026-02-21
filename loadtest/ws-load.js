import http from "k6/http";
import ws from "k6/ws";
import { sleep } from "k6";
import { Counter, Trend } from "k6/metrics";

const moveLatency = new Trend("move_latency_ms");
const wsErrorsTotal = new Counter("ws_errors_total");
const wsUnexpectedCloses = new Counter("ws_unexpected_closes");
const redisOpsTotal = new Counter("redis_ops_total");
const wsMoveTimeouts = new Counter("ws_move_timeouts_total");

export const options = {
  scenarios: {
    ws_load: {
      executor: "per-vu-iterations",
      vus: Number(__ENV.VUS || "200"),
      iterations: 1,
      maxDuration: "5m",
    },
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const WS_URL = __ENV.WS_URL || "ws://localhost:8080/ws";
const TOKEN = __ENV.WS_TOKEN || "";
const ROOM_ID = __ENV.ROOM_ID || "load-room";

function readRedisOpsCount() {
  const res = http.get(`${BASE_URL}/metrics`);
  if (res.status !== 200) return 0;
  const lines = res.body.match(/^redis_latency_seconds_count\{.*\}\s+([0-9.eE+-]+)/gm) || [];
  return lines.reduce((acc, line) => {
    const value = Number(line.trim().split(" ").at(-1) || "0");
    return acc + value;
  }, 0);
}

function readSettlementCount() {
  const res = http.get(`${BASE_URL}/metrics`);
  if (res.status !== 200) return 0;
  const lines = res.body.match(/^settlement_total\{.*\}\s+([0-9]+)/gm) || [];
  return lines.reduce((acc, line) => {
    const value = Number(line.trim().split(" ").at(-1) || "0");
    return acc + value;
  }, 0);
}

export function setup() {
  const initialOps = readRedisOpsCount();
  const initialSettlements = readSettlementCount();
  return { initialOps, initialSettlements, startedAt: Date.now() };
}

export default function () {
  const url = `${WS_URL}?token=${encodeURIComponent(TOKEN)}&roomId=${encodeURIComponent(ROOM_ID)}`;
  ws.connect(url, {}, function (socket) {
    let hadError = false;
    const pendingMoveStarts = [];

    socket.on("open", function () {
      socket.send(JSON.stringify({ type: "join_room", payload: { roomId: ROOM_ID } }));
      for (let i = 0; i < 10; i += 1) {
        const started = Date.now();
        pendingMoveStarts.push(started);
        socket.send(
          JSON.stringify({
            type: "make_move",
            payload: { roomId: ROOM_ID, action: "pass" },
          }),
        );
      }
      // Give server time to respond to moves.
      socket.setTimeout(function () {
        if (pendingMoveStarts.length > 0) {
          wsMoveTimeouts.add(pendingMoveStarts.length);
        }
        socket.close();
      }, 1500);
    });

    socket.on("message", function (raw) {
      let parsed;
      try {
        parsed = JSON.parse(raw);
      } catch {
        return;
      }
      if (!parsed || typeof parsed !== "object") return;
      const type = parsed.type;
      if (type === "error") {
        wsErrorsTotal.add(1);
      }
      if ((type === "game_state" || type === "timer_update" || type === "match_finished") && pendingMoveStarts.length > 0) {
        const started = pendingMoveStarts.shift();
        moveLatency.add(Date.now() - started);
      } else if (pendingMoveStarts.length > 0) {
        // Count any server response as signal for one pending move.
        const started = pendingMoveStarts.shift();
        moveLatency.add(Date.now() - started);
      }
    });

    socket.on("error", function () {
      hadError = true;
      wsErrorsTotal.add(1);
    });

    socket.on("close", function (code) {
      if (hadError || (code !== 1000 && code !== 1005)) {
        wsUnexpectedCloses.add(1);
      }
    });
  });

  // reconnect once
  ws.connect(url, {}, function (socket) {
    let hadError = false;
    socket.on("open", function () {
      socket.send(JSON.stringify({ type: "reconnect", payload: { roomId: ROOM_ID } }));
      socket.close();
    });
    socket.on("error", function () {
      hadError = true;
      wsErrorsTotal.add(1);
    });
    socket.on("close", function (code) {
      if (hadError || (code !== 1000 && code !== 1005)) {
        wsUnexpectedCloses.add(1);
      }
    });
  });

  sleep(0.1);
}

export function teardown(data) {
  const finalOps = readRedisOpsCount();
  const finalSettlements = readSettlementCount();
  const opsTotal = Math.max(0, finalOps - data.initialOps);
  const elapsedSec = Math.max((Date.now() - data.startedAt) / 1000, 1);
  if (opsTotal > 0) {
    redisOpsTotal.add(opsTotal);
  }
  const opsPerSec = opsTotal / elapsedSec;
  console.log(`redis ops/sec (estimated): ${opsPerSec.toFixed(2)}`);
  console.log(`settlement delta: ${finalSettlements - data.initialSettlements}`);
}
