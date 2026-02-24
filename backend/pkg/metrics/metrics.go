package metrics

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	wsActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ws_active_connections",
		Help: "Current number of active websocket connections.",
	})
	activeMatches = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_matches",
		Help: "Current number of active matches.",
	})
	matchMoveDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "match_move_duration_seconds",
		Help:    "Duration of make_move processing.",
		Buckets: prometheus.DefBuckets,
	})
	dbQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "Duration of DB queries.",
		Buckets: prometheus.DefBuckets,
	}, []string{"query"})
	redisLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "redis_latency_seconds",
		Help:    "Redis operation latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"op"})
	settlementTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "settlement_total",
		Help: "Number of settlement attempts by result.",
	}, []string{"result"})
	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "API request duration.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
	gameDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "game_duration_seconds",
		Help:    "Duration of completed games.",
		Buckets: []float64{10, 30, 60, 120, 300, 600, 1200},
	})
	gameAbandonTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "game_abandon_total",
		Help: "Games ended by disconnect timeout (abandon).",
	})
	gameReconnectTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "game_reconnect_total",
		Help: "Successful reconnects during active game.",
	})
	wsDisconnectTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_disconnect_total",
		Help: "WebSocket disconnections by reason.",
	}, []string{"reason"})
	gameFinishReasonTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "game_finish_reason_total",
		Help: "Games finished by reason: normal, abandon, disconnect_timeout.",
	}, []string{"reason"})
	roomCancelledTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "room_cancelled_total",
		Help: "Rooms cancelled (stale timeout).",
	})
	versionMismatchTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "version_mismatch_total",
		Help: "make_move rejected due to version mismatch.",
	})
	timeoutAppliedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "timeout_applied_total",
		Help: "Turn timeouts auto-applied.",
	})
)

func Handler() http.Handler {
	return promhttp.Handler()
}

func SetWSActiveConnections(value int) {
	wsActiveConnections.Set(float64(value))
}

func SetActiveMatches(value int) {
	activeMatches.Set(float64(value))
}

func ObserveMatchMoveDuration(start time.Time) {
	matchMoveDuration.Observe(time.Since(start).Seconds())
}

func ObserveDBQuery(query string, start time.Time) {
	dbQueryDuration.WithLabelValues(query).Observe(time.Since(start).Seconds())
}

func ObserveRedisLatency(op string, start time.Time) {
	redisLatency.WithLabelValues(op).Observe(time.Since(start).Seconds())
}

func IncSettlement(result string) {
	settlementTotal.WithLabelValues(result).Inc()
}

func ObserveGameDuration(seconds float64) {
	gameDurationSeconds.Observe(seconds)
}

func IncGameAbandon() {
	gameAbandonTotal.Inc()
}

func IncGameReconnect() {
	gameReconnectTotal.Inc()
}

func IncWSDisconnect(reason string) {
	wsDisconnectTotal.WithLabelValues(reason).Inc()
}

func IncGameFinishReason(reason string) {
	gameFinishReasonTotal.WithLabelValues(reason).Inc()
}

func IncRoomCancelled() {
	roomCancelledTotal.Inc()
}

func IncVersionMismatch() {
	versionMismatchTotal.Inc()
}

func IncTimeoutApplied() {
	timeoutAppliedTotal.Inc()
}

func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rec.status)).
			Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying writer does not support hijacking")
	}
	return h.Hijack()
}

func (s *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := s.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}
