package ratelimit

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Service struct {
	redis *redis.Client
}

var tokenBucketScript = redis.NewScript(`
local now_ms = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local refill_per_ms = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])
local ttl_ms = tonumber(ARGV[5])

local tokens = tonumber(redis.call("GET", KEYS[1]))
if tokens == nil then
  tokens = capacity
end

local last_refill = tonumber(redis.call("GET", KEYS[2]))
if last_refill == nil then
  last_refill = now_ms
end

local elapsed = now_ms - last_refill
if elapsed < 0 then
  elapsed = 0
end

tokens = math.min(capacity, tokens + (elapsed * refill_per_ms))

local allowed = 0
if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
end

redis.call("SET", KEYS[1], tokens, "PX", ttl_ms)
redis.call("SET", KEYS[2], now_ms, "PX", ttl_ms)

return {allowed, tokens}
`)

func NewService(redisClient *redis.Client) *Service {
	return &Service{redis: redisClient}
}

func (s *Service) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 || window <= 0 {
		return false, nil
	}
	nowMs := time.Now().UnixMilli()
	windowMs := window.Milliseconds()
	refillPerMs := float64(limit) / float64(windowMs)
	ttlMs := windowMs * 2
	keys := []string{
		"rl:tb:" + key + ":tokens",
		"rl:tb:" + key + ":ts",
	}

	result, err := tokenBucketScript.Run(
		ctx,
		s.redis,
		keys,
		nowMs,
		limit,
		refillPerMs,
		1,
		ttlMs,
	).Result()
	if err != nil {
		return false, err
	}
	values, ok := result.([]interface{})
	if !ok || len(values) < 1 {
		return false, errors.New("invalid rate limiter response")
	}
	allowed, ok := values[0].(int64)
	if !ok {
		return false, errors.New("invalid rate limiter allow value")
	}
	return allowed == 1, nil
}
