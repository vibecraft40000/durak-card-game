package rooms

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"durakonline/backend/pkg/metrics"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Status string

const (
	StatusWaiting              Status = "waiting"
	StatusConfirmed            Status = "confirmed"
	StatusAwaitingStakeConfirm Status = "awaiting_stake_confirm"
	StatusInGame               Status = "in_game"
	StatusFinished             Status = "finished"
	StatusCancelled            Status = "cancelled"
)

type Room struct {
	ID                   string    `json:"id"`
	Title                string    `json:"title"`
	Stake                float64   `json:"stake"`
	Players              []string  `json:"players"`
	ReadyUsers           []string  `json:"ready_users"`
	StakeConfirmedUsers  []string  `json:"stake_confirmed_users,omitempty"`
	StakeConfirmDeadline int64     `json:"stake_confirm_deadline,omitempty"`
	MaxPlayers           int       `json:"max_players"`
	Deck                 int       `json:"deck"`
	Mode                 string    `json:"mode"`
	Status               Status    `json:"status"`
	MatchID              string    `json:"match_id,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

type Repository struct {
	mu    sync.Mutex
	redis *redis.Client
}

func NewRepository(redisClient *redis.Client) *Repository {
	return &Repository{redis: redisClient}
}

func (r *Repository) List(ctx context.Context) ([]Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := time.Now()
	ids, err := r.redis.SMembers(ctx, "rooms:active").Result()
	metrics.ObserveRedisLatency("smembers_rooms_active", start)
	if err != nil {
		return nil, err
	}
	out := make([]Room, 0, len(ids))
	for _, id := range ids {
		room, ok := r.Get(ctx, id)
		if ok && (room.Status == StatusWaiting || room.Status == StatusConfirmed || room.Status == StatusAwaitingStakeConfirm || room.Status == StatusInGame) {
			out = append(out, room)
		}
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in Room) (Room, error) {
	_ = ctx
	in.ID = uuid.NewString()
	in.CreatedAt = time.Now().UTC()
	in.Status = StatusWaiting
	if err := r.Save(ctx, in); err != nil {
		return Room{}, err
	}
	start := time.Now()
	if err := r.redis.SAdd(ctx, "rooms:active", in.ID).Err(); err != nil {
		metrics.ObserveRedisLatency("sadd_rooms_active", start)
		return Room{}, err
	}
	metrics.ObserveRedisLatency("sadd_rooms_active", start)
	return in, nil
}

func (r *Repository) Get(ctx context.Context, roomID string) (Room, bool) {
	start := time.Now()
	raw, err := r.redis.Get(ctx, roomKey(roomID)).Result()
	metrics.ObserveRedisLatency("get_room", start)
	if err != nil {
		return Room{}, false
	}
	var room Room
	if err := json.Unmarshal([]byte(raw), &room); err != nil {
		return Room{}, false
	}
	if n, _ := r.redis.Exists(ctx, readyKey(roomID)).Result(); n > 0 {
		ready, err := r.GetReadyUsers(ctx, roomID)
		if err == nil {
			room.ReadyUsers = ready
		}
	}
	if n, _ := r.redis.Exists(ctx, stakeConfirmKey(roomID)).Result(); n > 0 {
		confirmed, err := r.GetStakeConfirmedUsers(ctx, roomID)
		if err == nil {
			room.StakeConfirmedUsers = confirmed
		}
	}
	return room, true
}

const (
	roomTTLActive   = 4 * time.Hour
	roomTTLFinished = 1 * time.Hour
)

func (r *Repository) Save(ctx context.Context, room Room) error {
	raw, err := json.Marshal(room)
	if err != nil {
		return err
	}
	ttl := roomTTLActive
	if room.Status == StatusFinished || room.Status == StatusCancelled {
		ttl = roomTTLFinished
		startSRem := time.Now()
		_ = r.redis.SRem(ctx, "rooms:active", room.ID).Err()
		metrics.ObserveRedisLatency("srem_rooms_active", startSRem)
		_ = r.ClearReadySet(ctx, room.ID)
		_ = r.ClearStakeConfirmedSet(ctx, room.ID)
	}
	start := time.Now()
	if err := r.redis.Set(ctx, roomKey(room.ID), raw, ttl).Err(); err != nil {
		metrics.ObserveRedisLatency("set_room", start)
		return err
	}
	metrics.ObserveRedisLatency("set_room", start)
	return nil
}

func roomKey(roomID string) string {
	return "room:" + roomID
}

func readyKey(roomID string) string {
	return "room:" + roomID + ":ready"
}

func stakeConfirmKey(roomID string) string {
	return "room:" + roomID + ":stake_confirmed"
}

// AddReadyUser atomically adds user to ready set. Prevents lost update race.
func (r *Repository) AddReadyUser(ctx context.Context, roomID, userID string) error {
	key := readyKey(roomID)
	start := time.Now()
	err := r.redis.SAdd(ctx, key, userID).Err()
	metrics.ObserveRedisLatency("sadd_ready", start)
	if err != nil {
		return err
	}
	_ = r.redis.Expire(ctx, key, roomTTLActive).Err()
	return nil
}

// GetReadyUsers returns the current ready set (source of truth).
func (r *Repository) GetReadyUsers(ctx context.Context, roomID string) ([]string, error) {
	key := readyKey(roomID)
	start := time.Now()
	vals, err := r.redis.SMembers(ctx, key).Result()
	metrics.ObserveRedisLatency("smembers_ready", start)
	return vals, err
}

// RemoveReadyUser removes user from ready set (e.g. on leave).
func (r *Repository) RemoveReadyUser(ctx context.Context, roomID, userID string) error {
	key := readyKey(roomID)
	start := time.Now()
	err := r.redis.SRem(ctx, key, userID).Err()
	metrics.ObserveRedisLatency("srem_ready", start)
	return err
}

// ClearReadySet removes the ready set (call after match start or on start failure).
func (r *Repository) ClearReadySet(ctx context.Context, roomID string) error {
	key := readyKey(roomID)
	return r.redis.Del(ctx, key).Err()
}

func (r *Repository) AddStakeConfirmedUser(ctx context.Context, roomID, userID string) error {
	key := stakeConfirmKey(roomID)
	start := time.Now()
	err := r.redis.SAdd(ctx, key, userID).Err()
	metrics.ObserveRedisLatency("sadd_stake_confirmed", start)
	if err != nil {
		return err
	}
	_ = r.redis.Expire(ctx, key, roomTTLActive).Err()
	return nil
}

func (r *Repository) RemoveStakeConfirmedUser(ctx context.Context, roomID, userID string) error {
	key := stakeConfirmKey(roomID)
	start := time.Now()
	err := r.redis.SRem(ctx, key, userID).Err()
	metrics.ObserveRedisLatency("srem_stake_confirmed", start)
	return err
}

func (r *Repository) GetStakeConfirmedUsers(ctx context.Context, roomID string) ([]string, error) {
	key := stakeConfirmKey(roomID)
	start := time.Now()
	vals, err := r.redis.SMembers(ctx, key).Result()
	metrics.ObserveRedisLatency("smembers_stake_confirmed", start)
	return vals, err
}

func (r *Repository) ClearStakeConfirmedSet(ctx context.Context, roomID string) error {
	key := stakeConfirmKey(roomID)
	return r.redis.Del(ctx, key).Err()
}

// ReleaseStartLock removes the start lock (call on HoldBet/StartMatch failure so room can retry).
func (r *Repository) ReleaseStartLock(ctx context.Context, roomID string) error {
	key := "room:" + roomID + ":starting"
	return r.redis.Del(ctx, key).Err()
}
