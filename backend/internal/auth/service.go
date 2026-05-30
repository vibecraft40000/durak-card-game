package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"durakonline/backend/internal/users"
	"durakonline/backend/pkg/metrics"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrInvalidRefreshToken = errors.New("invalid refresh token")
var ErrInvalidWSTicket = errors.New("invalid ws ticket")

const wsTicketTTL = 30 * time.Second

type wsTicketClaims struct {
	UserID string `json:"userId"`
	RoomID string `json:"roomId"`
}

type Service struct {
	users      *users.Repository
	redis      *redis.Client
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	replayTTL  time.Duration
	botToken   string
}

func NewService(usersRepo *users.Repository, redisClient *redis.Client, jwtSecret string, accessTTL, refreshTTL, replayTTL time.Duration, botToken string) *Service {
	return &Service{
		users:      usersRepo,
		redis:      redisClient,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		replayTTL:  replayTTL,
		botToken:   botToken,
	}
}

func (s *Service) ReplayTTL() time.Duration {
	return s.replayTTL
}

func (s *Service) WSTicketTTL() time.Duration {
	return wsTicketTTL
}

func (s *Service) ExchangeTelegram(ctx context.Context, tgUser TelegramUser, referralCode string) (users.User, string, string, error) {
	photoURL := tgUser.PhotoURL
	if photoURL == "" && s.botToken != "" {
		photoURL = FetchUserPhotoURL(ctx, s.botToken, tgUser.ID)
	}
	user, err := s.users.GetOrCreateByTelegram(ctx, tgUser.ID, tgUser.Username, tgUser.FirstName, tgUser.LastName, photoURL)
	if err != nil {
		return users.User{}, "", "", err
	}
	if referralCode != "" {
		_ = s.users.BindInviterByReferralCode(ctx, user.ID, referralCode)
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

func wsTicketKey(ticket string) string {
	return fmt.Sprintf("auth:ws_ticket:%s", ticket)
}

func (s *Service) IssueWSTicket(ctx context.Context, userID, roomID string) (string, error) {
	if s.redis == nil {
		return "", errors.New("redis client is required for ws tickets")
	}
	userID = strings.TrimSpace(userID)
	roomID = strings.TrimSpace(roomID)
	if userID == "" || roomID == "" {
		return "", errors.New("userID and roomID are required")
	}

	ticket := uuid.NewString()
	payload, err := json.Marshal(wsTicketClaims{
		UserID: userID,
		RoomID: roomID,
	})
	if err != nil {
		return "", err
	}
	if err := s.redis.Set(ctx, wsTicketKey(ticket), payload, wsTicketTTL).Err(); err != nil {
		return "", err
	}
	return ticket, nil
}

func (s *Service) ConsumeWSTicket(ctx context.Context, ticket, roomID string) (string, error) {
	if s.redis == nil {
		return "", errors.New("redis client is required for ws tickets")
	}
	ticket = strings.TrimSpace(ticket)
	roomID = strings.TrimSpace(roomID)
	if ticket == "" || roomID == "" {
		return "", ErrInvalidWSTicket
	}

	payload, err := s.redis.GetDel(ctx, wsTicketKey(ticket)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrInvalidWSTicket
		}
		return "", err
	}

	var claims wsTicketClaims
	if err := json.Unmarshal([]byte(payload), &claims); err != nil {
		return "", ErrInvalidWSTicket
	}
	if claims.UserID == "" || claims.RoomID == "" || claims.RoomID != roomID {
		return "", ErrInvalidWSTicket
	}

	return claims.UserID, nil
}

func (s *Service) ParseJWT(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token == nil || token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
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
