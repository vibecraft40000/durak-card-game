package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"durakonline/backend/internal/users"
	"durakonline/backend/pkg/metrics"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrInvalidRefreshToken = errors.New("invalid refresh token")

type Service struct {
	users      *users.Repository
	redis      *redis.Client
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	replayTTL  time.Duration
}

func NewService(usersRepo *users.Repository, redisClient *redis.Client, jwtSecret string, accessTTL, refreshTTL, replayTTL time.Duration) *Service {
	return &Service{
		users:      usersRepo,
		redis:      redisClient,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		replayTTL:  replayTTL,
	}
}

func (s *Service) ReplayTTL() time.Duration {
	return s.replayTTL
}

func (s *Service) ExchangeTelegram(ctx context.Context, tgUser TelegramUser) (users.User, string, string, error) {
	user, err := s.users.GetOrCreateByTelegram(ctx, tgUser.ID, tgUser.Username, tgUser.FirstName, tgUser.LastName, tgUser.PhotoURL)
	if err != nil {
		return users.User{}, "", "", err
	}

	accessToken, err := s.issueJWT(user.ID, s.accessTTL)
	if err != nil {
		return users.User{}, "", "", err
	}

	refreshToken := uuid.NewString()
	start := time.Now()
	if err := s.redis.Set(ctx, refreshKey(refreshToken), user.ID, s.refreshTTL).Err(); err != nil {
		metrics.ObserveRedisLatency("set_refresh_token", start)
		return users.User{}, "", "", err
	}
	metrics.ObserveRedisLatency("set_refresh_token", start)

	return user, accessToken, refreshToken, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (string, error) {
	start := time.Now()
	userID, err := s.redis.Get(ctx, refreshKey(refreshToken)).Result()
	metrics.ObserveRedisLatency("get_refresh_token", start)
	if err != nil {
		return "", ErrInvalidRefreshToken
	}
	return s.issueJWT(userID, s.accessTTL)
}

func (s *Service) MarkInitDataHashUsed(ctx context.Context, hash string) (bool, error) {
	key := replayKey(hash)
	start := time.Now()
	ok, err := s.redis.SetNX(ctx, key, "1", s.replayTTL).Result()
	metrics.ObserveRedisLatency("setnx_replay_hash", start)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (s *Service) issueJWT(userID string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(ttl).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func replayKey(hash string) string {
	return fmt.Sprintf("auth:replay:%s", hash)
}

func refreshKey(token string) string {
	return fmt.Sprintf("auth:refresh:%s", token)
}

func (s *Service) ParseJWT(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", errors.New("sub claim missing")
	}
	return sub, nil
}
