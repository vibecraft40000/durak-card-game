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
	StatusWaiting   Status = "waiting"
	StatusConfirmed Status = "confirmed"
	StatusInGame    Status = "in_game"
	StatusFinished  Status = "finished"
	StatusCancelled Status = "cancelled"
)

type Room struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Stake      float64   `json:"stake"`
	Players    []string  `json:"players"`
	ReadyUsers []string  `json:"ready_users"`
	MaxPlayers int       `json:"max_players"`
	Deck       int       `json:"deck"`
	Mode       string    `json:"mode"`
	Status     Status    `json:"status"`
	MatchID    string    `json:"match_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
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
		if ok && (room.Status == StatusWaiting || room.Status == StatusConfirmed || room.Status == StatusInGame) {
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
