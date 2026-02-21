package games

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"durakonline/backend/internal/games/engine"
	"durakonline/backend/pkg/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var ErrMatchNotFound = errors.New("match not found")

type HistoryRecord struct {
	MatchID   string          `json:"match_id"`
	StateJSON json.RawMessage `json:"state"`
	CreatedAt time.Time       `json:"created_at"`
}

type Service struct {
	db       *pgxpool.Pool
	redis    *redis.Client
	history  []HistoryRecord
	turnTTL  time.Duration
	stateTTL time.Duration
	locks    sync.Map
}

func NewService(db *pgxpool.Pool, redisClient *redis.Client, turnTTL, stateTTL time.Duration) *Service {
	return &Service{
		db:       db,
		redis:    redisClient,
		history:  make([]HistoryRecord, 0, 128),
		turnTTL:  turnTTL,
		stateTTL: stateTTL,
	}
}

func (s *Service) StartMatch(ctx context.Context, matchID string, stake float64, mode string, players []string) (engine.GameState, error) {
	if mode == "" {
		mode = "classic"
	}
	mu := s.matchMutex(matchID)
	mu.Lock()
	defer mu.Unlock()

	state := engine.NewGameState(matchID, players, s.turnTTL)
	if err := s.saveState(ctx, state); err != nil {
		return engine.GameState{}, err
	}
	if s.db != nil {
		start := time.Now()
		_, _ = s.db.Exec(ctx, `INSERT INTO matches (id, status, stake, mode, created_at) VALUES ($1::uuid, 'active', $2, $3, NOW())`, matchID, stake, mode)
		metrics.ObserveDBQuery("insert_match", start)
	}
	start := time.Now()
	_ = s.redis.SAdd(ctx, "matches:active", matchID).Err()
	metrics.ObserveRedisLatency("sadd_matches_active", start)
	s.updateActiveMatchesGauge(ctx)
	s.snapshotLocked(matchID, state)
	return state, nil
}

func (s *Service) GetState(ctx context.Context, matchID string) (engine.GameState, error) {
	start := time.Now()
	raw, err := s.redis.Get(ctx, stateKey(matchID)).Result()
	metrics.ObserveRedisLatency("get_state", start)
	if err != nil {
		return engine.GameState{}, ErrMatchNotFound
	}
	var state engine.GameState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return engine.GameState{}, err
	}
	return state, nil
}

func (s *Service) Apply(ctx context.Context, matchID, playerID string, action engine.Action, cardID string) (engine.GameState, error) {
	mu := s.matchMutex(matchID)
	mu.Lock()
	defer mu.Unlock()
	release, err := s.acquireRedisLock(ctx, matchID)
	if err != nil {
		return engine.GameState{}, err
	}
	defer release()

	state, err := s.GetState(ctx, matchID)
	if err != nil {
		return engine.GameState{}, ErrMatchNotFound
	}
	if err := engine.ApplyAction(&state, playerID, action, cardID, s.turnTTL); err != nil {
		return engine.GameState{}, err
	}
	if err := s.saveState(ctx, state); err != nil {
		return engine.GameState{}, err
	}
	if state.Status == engine.StatusFinished {
		dur := 0
		if !state.StartedAt.IsZero() {
			dur = int(time.Since(state.StartedAt).Seconds())
			metrics.ObserveGameDuration(float64(dur))
		}
		s.updateMatchFinishedWithDetails(ctx, matchID, state.WinnerPlayer, "normal", dur)
		start := time.Now()
		_ = s.redis.SRem(ctx, "matches:active", matchID).Err()
		metrics.ObserveRedisLatency("srem_matches_active", start)
		s.updateActiveMatchesGauge(ctx)
	}
	s.snapshotLocked(matchID, state)
	return state, nil
}

// updateMatchFinishedWithDetails saves match outcome: winner, reason (normal|abandon|disconnect_timeout), duration_seconds.
func (s *Service) updateMatchFinishedWithDetails(ctx context.Context, matchID, winnerID, reason string, durationSec int) {
	if s.db == nil {
		return
	}
	metrics.IncGameFinishReason(reason)
	start := time.Now()
	_, _ = s.db.Exec(ctx,
		`UPDATE matches SET status='finished', finished_at=NOW(), winner=$2, finish_reason=$3::match_finish_reason, duration_seconds=$4 WHERE id=$1::uuid`,
		matchID, winnerID, reason, durationSec)
	metrics.ObserveDBQuery("update_match_finished", start)
}

func (s *Service) updateMatchFinished(ctx context.Context, matchID string) {
	s.updateMatchFinishedWithDetails(ctx, matchID, "", "normal", 0)
}

func (s *Service) HandleTimeouts(ctx context.Context) []string {
	finished := make([]string, 0)
	now := time.Now()
	start := time.Now()
	matchIDs, err := s.redis.SMembers(ctx, "matches:active").Result()
	metrics.ObserveRedisLatency("smembers_matches_active", start)
	if err != nil {
		return finished
	}
	for _, matchID := range matchIDs {
		state, err := s.GetState(ctx, matchID)
		if err != nil || !engine.Expired(state, now) {
			continue
		}
		_ = engine.ApplyAction(&state, state.TurnPlayerID, engine.ActionPass, "", s.turnTTL)
		_ = s.saveState(ctx, state)
		if state.Status == engine.StatusFinished {
			dur := 0
			if !state.StartedAt.IsZero() {
				dur = int(time.Since(state.StartedAt).Seconds())
				metrics.ObserveGameDuration(float64(dur))
			}
			s.updateMatchFinishedWithDetails(ctx, matchID, state.WinnerPlayer, "normal", dur)
			finished = append(finished, matchID)
			start := time.Now()
			_ = s.redis.SRem(ctx, "matches:active", matchID).Err()
			metrics.ObserveRedisLatency("srem_matches_active", start)
		}
		s.snapshotLocked(matchID, state)
	}
	s.updateActiveMatchesGauge(ctx)
	return finished
}

func (s *Service) snapshotLocked(matchID string, state engine.GameState) {
	raw, _ := json.Marshal(state)
	s.history = append(s.history, HistoryRecord{
		MatchID:   matchID,
		StateJSON: raw,
		CreatedAt: time.Now().UTC(),
	})
	if s.db != nil {
		start := time.Now()
		_, _ = s.db.Exec(context.Background(), `INSERT INTO game_history (match_id, state, created_at) VALUES ($1, $2, NOW())`, matchID, raw)
		metrics.ObserveDBQuery("insert_game_history", start)
	}
}

func (s *Service) saveState(ctx context.Context, state engine.GameState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	start := time.Now()
	err = s.redis.Set(ctx, stateKey(state.MatchID), raw, s.stateTTL).Err()
	metrics.ObserveRedisLatency("set_state", start)
	return err
}

func stateKey(matchID string) string {
	return "match:state:" + matchID
}

func disconnectedKey(matchID, playerID string) string {
	return "match:disconnected:" + matchID + ":" + playerID
}

const disconnectGracePeriod = 60 * time.Second
const disconnectedKeyTTL = 120 * time.Second

// MarkDisconnected records that a player disconnected (in-game grace period).
func (s *Service) MarkDisconnected(ctx context.Context, matchID, playerID string) error {
	key := disconnectedKey(matchID, playerID)
	val := fmt.Sprintf("%d", time.Now().Unix())
	start := time.Now()
	err := s.redis.Set(ctx, key, val, disconnectedKeyTTL).Err()
	metrics.ObserveRedisLatency("set_disconnected", start)
	return err
}

// ClearDisconnected removes the disconnected marker when player reconnects.
func (s *Service) ClearDisconnected(ctx context.Context, matchID, playerID string) error {
	key := disconnectedKey(matchID, playerID)
	start := time.Now()
	err := s.redis.Del(ctx, key).Err()
	metrics.ObserveRedisLatency("del_disconnected", start)
	return err
}

// AbandonResult is returned when a match is force-finished due to disconnect timeout.
type AbandonResult struct {
	MatchID string
	State   engine.GameState
}

// HandleDisconnectTimeouts finds players disconnected > 60s and force-finishes those matches.
func (s *Service) HandleDisconnectTimeouts(ctx context.Context) []AbandonResult {
	var results []AbandonResult
	start := time.Now()
	matchIDs, err := s.redis.SMembers(ctx, "matches:active").Result()
	metrics.ObserveRedisLatency("smembers_matches_active", start)
	if err != nil {
		return results
	}
	now := time.Now()
	for _, matchID := range matchIDs {
		state, err := s.GetState(ctx, matchID)
		if err != nil {
			continue
		}
		if state.Status != engine.StatusPlaying {
			continue
		}
		for _, playerID := range state.PlayerOrder {
			key := disconnectedKey(matchID, playerID)
			val, err := s.redis.Get(ctx, key).Result()
			if err != nil {
				continue
			}
			var ts int64
			if _, err := fmt.Sscanf(val, "%d", &ts); err != nil {
				continue
			}
			disconnectedAt := time.Unix(ts, 0)
			if now.Sub(disconnectedAt) < disconnectGracePeriod {
				continue
			}
			// Disconnected > 60s -> force abandon
			abandoned, err := s.ForceAbandon(ctx, matchID, playerID)
			if err == nil {
				results = append(results, AbandonResult{MatchID: matchID, State: abandoned})
			}
			break // only one abandon per match per tick
		}
	}
	s.updateActiveMatchesGauge(ctx)
	return results
}

// ForceAbandon ends the match with the non-disconnected player as winner.
func (s *Service) ForceAbandon(ctx context.Context, matchID, disconnectedPlayerID string) (engine.GameState, error) {
	mu := s.matchMutex(matchID)
	mu.Lock()
	defer mu.Unlock()
	release, err := s.acquireRedisLock(ctx, matchID)
	if err != nil {
		return engine.GameState{}, err
	}
	defer release()

	state, err := s.GetState(ctx, matchID)
	if err != nil {
		return engine.GameState{}, err
	}
	if state.Status != engine.StatusPlaying {
		return state, nil
	}
	var winnerID string
	for _, pid := range state.PlayerOrder {
		if pid != disconnectedPlayerID {
			winnerID = pid
			break
		}
	}
	if winnerID == "" {
		return engine.GameState{}, errors.New("no other player to win")
	}
	engine.ForceFinishWithWinner(&state, winnerID)
	if err := s.saveState(ctx, state); err != nil {
		return engine.GameState{}, err
	}
	dur := 0
	if !state.StartedAt.IsZero() {
		dur = int(time.Since(state.StartedAt).Seconds())
		metrics.ObserveGameDuration(float64(dur))
	}
	metrics.IncGameAbandon()
	s.updateMatchFinishedWithDetails(ctx, matchID, winnerID, "disconnect_timeout", dur)
	key := disconnectedKey(matchID, disconnectedPlayerID)
	_ = s.redis.Del(ctx, key).Err()
	start := time.Now()
	_ = s.redis.SRem(ctx, "matches:active", matchID).Err()
	metrics.ObserveRedisLatency("srem_matches_active", start)
	s.snapshotLocked(matchID, state)
	return state, nil
}

func (s *Service) matchMutex(matchID string) *sync.Mutex {
	value, _ := s.locks.LoadOrStore(matchID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func (s *Service) acquireRedisLock(ctx context.Context, matchID string) (func(), error) {
	key := "lock:match:" + matchID
	token := fmt.Sprintf("%d", rand.Int63())
	start := time.Now()
	ok, err := s.redis.SetNX(ctx, key, token, 3*time.Second).Result()
	metrics.ObserveRedisLatency("setnx_match_lock", start)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("match is locked")
	}
	return func() {
		start := time.Now()
		s.redis.Eval(ctx, `
if redis.call("GET", KEYS[1]) == ARGV[1] then
 return redis.call("DEL", KEYS[1])
end
return 0`, []string{key}, token)
		metrics.ObserveRedisLatency("eval_unlock_match", start)
	}, nil
}

func (s *Service) ActiveMatches(ctx context.Context) int {
	start := time.Now()
	count, err := s.redis.SCard(ctx, "matches:active").Result()
	metrics.ObserveRedisLatency("scard_matches_active", start)
	if err != nil {
		return 0
	}
	return int(count)
}

func (s *Service) updateActiveMatchesGauge(ctx context.Context) {
	metrics.SetActiveMatches(s.ActiveMatches(ctx))
}

// ReconcileActiveMatches removes orphaned match IDs from matches:active when match:state has expired.
// Call periodically to prevent memory growth if matches finish without proper SRem (e.g. crash).
func (s *Service) ReconcileActiveMatches(ctx context.Context) int {
	start := time.Now()
	matchIDs, err := s.redis.SMembers(ctx, "matches:active").Result()
	metrics.ObserveRedisLatency("smembers_matches_active_reconcile", start)
	if err != nil {
		return 0
	}
	removed := 0
	for _, matchID := range matchIDs {
		_, err := s.redis.Get(ctx, stateKey(matchID)).Result()
		if err != nil {
			s.redis.SRem(ctx, "matches:active", matchID)
			removed++
		}
	}
	if removed > 0 {
		s.updateActiveMatchesGauge(ctx)
	}
	return removed
}
