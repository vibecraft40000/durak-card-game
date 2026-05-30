package games

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"durakonline/backend/internal/gameresults"
	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/pkg/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	ErrMatchNotFound   = errors.New("match not found")
	ErrVersionMismatch = errors.New("version mismatch: state changed")
)

type DisconnectPolicy string

const (
	DisconnectPolicyAbandon     DisconnectPolicy = "abandon"
	DisconnectPolicyBotTakeover DisconnectPolicy = "bot_takeover"
)

func NormalizeDisconnectPolicy(raw string) DisconnectPolicy {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "bot_takeover", "bot", "skip", "auto":
		return DisconnectPolicyBotTakeover
	default:
		return DisconnectPolicyAbandon
	}
}

type HistoryRecord struct {
	MatchID   string          `json:"match_id"`
	StateJSON json.RawMessage `json:"state"`
	CreatedAt time.Time       `json:"created_at"`
}

type Service struct {
	db               *pgxpool.Pool
	redis            *redis.Client
	gameResultsRepo  *gameresults.Repository
	history          []HistoryRecord
	turnTTL          time.Duration
	stateTTL         time.Duration
	disconnectPolicy DisconnectPolicy
	locks            sync.Map
}

func NewService(db *pgxpool.Pool, redisClient *redis.Client, turnTTL, stateTTL time.Duration) *Service {
	var gameResultsRepo *gameresults.Repository
	if db != nil {
		gameResultsRepo = gameresults.NewRepository(db)
	}
	return &Service{
		db:               db,
		redis:            redisClient,
		gameResultsRepo:  gameResultsRepo,
		history:          make([]HistoryRecord, 0, 128),
		turnTTL:          turnTTL,
		stateTTL:         stateTTL,
		disconnectPolicy: DisconnectPolicyAbandon,
	}
}

func (s *Service) SetDisconnectPolicy(policy DisconnectPolicy) {
	if policy == "" {
		policy = DisconnectPolicyAbandon
	}
	s.disconnectPolicy = policy
}

func (s *Service) DisconnectPolicy() DisconnectPolicy {
	if s.disconnectPolicy == "" {
		return DisconnectPolicyAbandon
	}
	return s.disconnectPolicy
}

func (s *Service) StartMatch(ctx context.Context, matchID string, stake float64, mode string, players []string) (engine.GameState, error) {
	return s.StartMatchWithConfig(ctx, matchID, stake, engine.GameConfig{
		DeckSize: 36,
		Mode:     mode,
	}, players)
}

func (s *Service) StartMatchWithConfig(ctx context.Context, matchID string, stake float64, cfg engine.GameConfig, players []string) (engine.GameState, error) {
	if cfg.Mode == "" {
		cfg.Mode = "podkidnoy"
	}
	if cfg.DeckSize == 0 {
		cfg.DeckSize = 36
	}
	mu := s.matchMutex(matchID)
	mu.Lock()
	defer mu.Unlock()

	state := engine.NewGameStateWithConfig(matchID, players, s.turnTTL, cfg)
	if err := s.saveState(ctx, state); err != nil {
		return engine.GameState{}, err
	}
	if s.db != nil {
		start := time.Now()
		_, _ = s.db.Exec(ctx, `INSERT INTO matches (id, status, stake, mode, created_at) VALUES ($1::uuid, 'active', $2, $3, NOW())`, matchID, stake, state.Mode)
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

// MutateMatch is the single mutation pipeline: lock → load → validate → apply → save → release.
// All mutation callers (Apply, HandleTimeouts, bot) use this path.
// If actionID is non-empty and was already processed, returns (state, false, nil) — applied=false means no broadcast.
func (s *Service) MutateMatch(ctx context.Context, matchID string, expectedVersion *int64, actorID string, action engine.Action, cardID string, actionID string) (engine.GameState, bool, error) {
	mu := s.matchMutex(matchID)
	mu.Lock()
	defer mu.Unlock()
	release, err := s.acquireRedisLock(ctx, matchID)
	if err != nil {
		return engine.GameState{}, false, err
	}
	defer release()

	state, err := s.GetState(ctx, matchID)
	if err != nil {
		return engine.GameState{}, false, ErrMatchNotFound
	}
	if actionID != "" {
		key := processedActionKey(matchID, actionID)
		if _, err := s.redis.Get(ctx, key).Result(); err == nil {
			return state, false, nil
		}
	}
	if expectedVersion != nil && state.Version != *expectedVersion {
		return engine.GameState{}, false, ErrVersionMismatch
	}
	if err := s.applyCore(ctx, matchID, &state, actorID, action, cardID); err != nil {
		return engine.GameState{}, false, err
	}
	if actionID != "" {
		key := processedActionKey(matchID, actionID)
		s.redis.Set(ctx, key, "1", processedActionTTL)
	}
	return state, true, nil
}

// Apply delegates to MutateMatch. Returns (state, applied, err). When applied=false (duplicate actionID), callers should not broadcast.
func (s *Service) Apply(ctx context.Context, matchID, playerID string, action engine.Action, cardID string, expectedVersion *int64, actionID string) (engine.GameState, bool, error) {
	return s.MutateMatch(ctx, matchID, expectedVersion, playerID, action, cardID, actionID)
}

// applyCore mutates state, saves, updates match if finished. Caller must hold lock.
func (s *Service) applyCore(ctx context.Context, matchID string, state *engine.GameState, actorID string, action engine.Action, cardID string) error {
	if err := engine.ApplyAction(state, actorID, action, cardID, s.turnTTL); err != nil {
		return err
	}
	if err := s.saveState(ctx, *state); err != nil {
		return err
	}
	if state.Status == engine.StatusFinished {
		s.redis.ZRem(ctx, turnDeadlinesKey(), matchID)
		s.clearDisconnectMarkers(ctx, matchID, state.PlayerOrder)
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
	s.snapshotLocked(matchID, *state)
	return nil
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

// HandleTimeouts applies phase-aware auto-actions for expired turns.
// Uses ZSET turn_deadlines for O(1) lookup of expired matches (scales to 10k+ concurrent games).
// Returns TimeoutResult for each successfully applied timeout so callers can broadcast.
func (s *Service) HandleTimeouts(ctx context.Context) []TimeoutResult {
	var results []TimeoutResult
	nowMs := time.Now().UnixMilli()
	start := time.Now()
	matchIDs, err := s.redis.ZRangeByScore(ctx, turnDeadlinesKey(), &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", nowMs),
	}).Result()
	metrics.ObserveRedisLatency("zrangebyscore_turn_deadlines", start)
	if err != nil {
		return results
	}
	for _, matchID := range matchIDs {
		r := s.applyTimeoutForMatch(ctx, matchID, time.Now())
		if r != nil {
			results = append(results, *r)
		}
	}
	s.updateActiveMatchesGauge(ctx)
	return results
}

// applyTimeoutForMatch acquires lock, applies phase-aware auto-action if expired via applyCore, returns result or nil.
func (s *Service) applyTimeoutForMatch(ctx context.Context, matchID string, now time.Time) *TimeoutResult {
	release, err := s.acquireRedisLock(ctx, matchID)
	if err != nil {
		return nil
	}
	defer release()

	state, err := s.GetState(ctx, matchID)
	if err != nil || !engine.Expired(state, now) {
		return nil
	}

	playerID := state.TurnPlayerID
	action, cardID := s.timeoutActionForState(&state)
	if err := s.applyCore(ctx, matchID, &state, playerID, action, cardID); err != nil {
		return nil
	}
	return &TimeoutResult{
		MatchID:  matchID,
		State:    state,
		Action:   action,
		CardID:   cardID,
		PlayerID: playerID,
	}
}

func (s *Service) timeoutActionForState(state *engine.GameState) (engine.Action, string) {
	if state.TurnState == engine.TurnDefend {
		return engine.ActionTake, ""
	}
	if len(state.TableCards) > 0 && len(state.TableCards)%2 == 0 {
		return engine.ActionPass, ""
	}
	hand := state.Hands[state.TurnPlayerID]
	if len(hand) == 0 {
		return engine.ActionPass, ""
	}
	return engine.ActionAttack, hand[0].ID
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
	if err != nil {
		return err
	}
	if state.Status == engine.StatusPlaying && !state.TurnEndsAt.IsZero() {
		zkey := turnDeadlinesKey()
		score := float64(state.TurnEndsAt.UnixMilli())
		s.redis.ZAdd(ctx, zkey, redis.Z{Score: score, Member: state.MatchID})
	}
	return nil
}

func stateKey(matchID string) string {
	return "match:state:" + matchID
}

func processedActionKey(matchID, actionID string) string {
	return "processed:" + matchID + ":" + actionID
}

const processedActionTTL = 1 * time.Hour

func turnDeadlinesKey() string {
	return "turn_deadlines"
}

func disconnectedKey(matchID, playerID string) string {
	return "match:disconnected:" + matchID + ":" + playerID
}

func botTakeoverKey(matchID, playerID string) string {
	return "match:bot_takeover:" + matchID + ":" + playerID
}

const disconnectGracePeriod = 60 * time.Second
const disconnectedKeyTTL = 120 * time.Second
const botTakeoverKeyTTL = 6 * time.Hour

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
	botKey := botTakeoverKey(matchID, playerID)
	start := time.Now()
	err := s.redis.Del(ctx, key, botKey).Err()
	metrics.ObserveRedisLatency("del_disconnected", start)
	return err
}

type DisconnectResolutionKind string

const (
	DisconnectResolutionAbandon     DisconnectResolutionKind = "abandon"
	DisconnectResolutionBotTakeover DisconnectResolutionKind = "bot_takeover"
)

// DisconnectResolution is returned when disconnect timeout policy was applied.
type DisconnectResolution struct {
	MatchID  string
	PlayerID string
	State    engine.GameState
	Kind     DisconnectResolutionKind
}

// TimeoutResult is returned when HandleTimeouts successfully applies an auto-action.
type TimeoutResult struct {
	MatchID  string
	State    engine.GameState
	Action   engine.Action
	CardID   string
	PlayerID string
}

// HandleDisconnectTimeouts applies disconnect policy for players offline > grace period.
func (s *Service) HandleDisconnectTimeouts(ctx context.Context) []DisconnectResolution {
	var results []DisconnectResolution
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
			switch s.DisconnectPolicy() {
			case DisconnectPolicyBotTakeover:
				updatedState, err := s.EnableBotTakeover(ctx, matchID, playerID)
				if err == nil {
					results = append(results, DisconnectResolution{
						MatchID:  matchID,
						PlayerID: playerID,
						State:    updatedState,
						Kind:     DisconnectResolutionBotTakeover,
					})
				}
			default:
				abandoned, err := s.ForceAbandon(ctx, matchID, playerID)
				if err == nil {
					results = append(results, DisconnectResolution{
						MatchID:  matchID,
						PlayerID: playerID,
						State:    abandoned,
						Kind:     DisconnectResolutionAbandon,
					})
				}
			}
			break // only one resolution per match per tick
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
	s.redis.ZRem(ctx, turnDeadlinesKey(), matchID)
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
	s.clearDisconnectMarkers(ctx, matchID, state.PlayerOrder)
	start := time.Now()
	_ = s.redis.SRem(ctx, "matches:active", matchID).Err()
	metrics.ObserveRedisLatency("srem_matches_active", start)
	s.snapshotLocked(matchID, state)
	return state, nil
}

// EnableBotTakeover switches disconnect handling from abandon to bot/auto mode for the player.
func (s *Service) EnableBotTakeover(ctx context.Context, matchID, playerID string) (engine.GameState, error) {
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
	inMatch := false
	for _, pid := range state.PlayerOrder {
		if pid == playerID {
			inMatch = true
			break
		}
	}
	if !inMatch {
		return engine.GameState{}, errors.New("player not in match")
	}

	start := time.Now()
	activated, err := s.redis.SetNX(ctx, botTakeoverKey(matchID, playerID), "1", botTakeoverKeyTTL).Result()
	metrics.ObserveRedisLatency("setnx_bot_takeover", start)
	if err != nil {
		return engine.GameState{}, err
	}

	start = time.Now()
	_ = s.redis.Del(ctx, disconnectedKey(matchID, playerID)).Err()
	metrics.ObserveRedisLatency("del_disconnected_after_takeover", start)
	if activated {
		metrics.IncGameBotTakeover()
	}
	return state, nil
}

func (s *Service) clearDisconnectMarkers(ctx context.Context, matchID string, playerIDs []string) {
	if len(playerIDs) == 0 {
		return
	}
	keys := make([]string, 0, len(playerIDs)*2)
	for _, playerID := range playerIDs {
		if playerID == "" {
			continue
		}
		keys = append(keys, disconnectedKey(matchID, playerID))
		keys = append(keys, botTakeoverKey(matchID, playerID))
	}
	if len(keys) == 0 {
		return
	}
	start := time.Now()
	_ = s.redis.Del(ctx, keys...).Err()
	metrics.ObserveRedisLatency("del_disconnect_markers", start)
}

func (s *Service) matchMutex(matchID string) *sync.Mutex {
	value, _ := s.locks.LoadOrStore(matchID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func (s *Service) acquireRedisLock(ctx context.Context, matchID string) (func(), error) {
	key := "lock:match:" + matchID
	token := fmt.Sprintf("%d", rand.Int63())
	start := time.Now()
	ok, err := s.redis.SetNX(ctx, key, token, 10*time.Second).Result()
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

// SettleMatchIfFinished runs settlement and returns PayoutInfo for match_finished broadcast.
func (s *Service) SettleMatchIfFinished(ctx context.Context, walletService *wallet.Service, state engine.GameState, stake float64, commissionBps int) (*PayoutInfo, error) {
	return SettleIfFinished(ctx, s.db, s.redis, walletService, s.gameResultsRepo, state, stake, commissionBps)
}

// ReconcileTurnDeadlines backfills turn_deadlines ZSET for matches in matches:active that are not yet in ZSET.
// Handles matches created before ZSET was introduced. Runs periodically (e.g. every 5 min).
func (s *Service) ReconcileTurnDeadlines(ctx context.Context) int {
	start := time.Now()
	active, err := s.redis.SMembers(ctx, "matches:active").Result()
	metrics.ObserveRedisLatency("smembers_reconcile_deadlines", start)
	if err != nil {
		return 0
	}
	added := 0
	zkey := turnDeadlinesKey()
	for _, matchID := range active {
		state, err := s.GetState(ctx, matchID)
		if err != nil || state.Status != engine.StatusPlaying || state.TurnEndsAt.IsZero() {
			continue
		}
		score := float64(state.TurnEndsAt.UnixMilli())
		if n, _ := s.redis.ZAdd(ctx, zkey, redis.Z{Score: score, Member: matchID}).Result(); n > 0 {
			added++
		}
	}
	return added
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
