package rooms

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"

	"durakonline/backend/internal/games"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/pkg/metrics"

	"github.com/google/uuid"
)

var (
	ErrRoomNotFound = errors.New("room not found")
	ErrRoomFull     = errors.New("room full")
	ErrNeedOpponent = errors.New("waiting for opponent")
)

type Service struct {
	repo          *Repository
	games         *games.Service
	wallet        *wallet.Service
	commissionBps int
	disableMoney  bool
}

func NewService(repo *Repository, gamesService *games.Service, walletService *wallet.Service, commissionBps int, disableMoney bool) *Service {
	return &Service{
		repo:          repo,
		games:         gamesService,
		wallet:        walletService,
		commissionBps: commissionBps,
		disableMoney:  disableMoney,
	}
}

func (s *Service) List(ctx context.Context) ([]Room, error) {
	return s.repo.List(ctx)
}

func (s *Service) Create(ctx context.Context, title string, stake float64, maxPlayers int, deck int, mode string, ownerID string) (Room, error) {
	room := Room{
		Title:      title,
		Stake:      stake,
		MaxPlayers: maxPlayers,
		Deck:       deck,
		Mode:       mode,
		Players:    []string{ownerID},
	}
	return s.repo.Create(ctx, room)
}

func (s *Service) Join(ctx context.Context, roomID string, userID string) (Room, error) {
	return s.withRoomLock(ctx, roomID, func() (Room, error) {
		room, ok := s.repo.Get(ctx, roomID)
		if !ok {
			return Room{}, ErrRoomNotFound
		}
		if !slices.Contains(room.Players, userID) {
			if len(room.Players) >= room.MaxPlayers {
				return Room{}, ErrRoomFull
			}
			room.Players = append(room.Players, userID)
		}
		_ = s.repo.Save(ctx, room)
		return room, nil
	})
}

func (s *Service) Ready(ctx context.Context, roomID string, userID string) (Room, error) {
	return s.withRoomLock(ctx, roomID, func() (Room, error) {
		room, ok := s.repo.Get(ctx, roomID)
		if !ok {
			return Room{}, ErrRoomNotFound
		}
		if shouldAutofillWithBot(room) && len(room.Players) == 1 {
			botID := BotPlayerID(room.ID)
			if !slices.Contains(room.Players, botID) {
				room.Players = append(room.Players, botID)
			}
			if !slices.Contains(room.ReadyUsers, botID) {
				room.ReadyUsers = append(room.ReadyUsers, botID)
			}
		}
		if !slices.Contains(room.ReadyUsers, userID) {
			room.ReadyUsers = append(room.ReadyUsers, userID)
		}
		if !shouldAutofillWithBot(room) && len(room.Players) < 2 {
			_ = s.repo.Save(ctx, room)
			return room, ErrNeedOpponent
		}
		if len(room.Players) == 2 && len(room.ReadyUsers) == 2 && room.Status == StatusWaiting {
			room.Status = StatusConfirmed
			matchID := uuid.NewString()
			if !s.disableMoney {
				for _, playerID := range room.Players {
					if IsBotPlayer(playerID) {
						continue
					}
					if err := s.wallet.HoldBet(ctx, playerID, matchID, room.Stake); err != nil {
						return Room{}, err
					}
				}
			}

			if _, err := s.games.StartMatch(ctx, matchID, room.Stake, room.Mode, room.Players); err != nil {
				return Room{}, err
			}
			room.MatchID = matchID
			room.Status = StatusInGame
		}
		_ = s.repo.Save(ctx, room)
		return room, nil
	})
}

func (s *Service) Get(ctx context.Context, roomID string) (Room, error) {
	room, ok := s.repo.Get(ctx, roomID)
	if !ok {
		return Room{}, ErrRoomNotFound
	}
	return room, nil
}

// MarkRoomFinished sets room status to finished and saves (uses shorter Redis TTL).
func (s *Service) MarkRoomFinished(ctx context.Context, roomID string) (Room, error) {
	return s.withRoomLock(ctx, roomID, func() (Room, error) {
		room, ok := s.repo.Get(ctx, roomID)
		if !ok {
			return Room{}, ErrRoomNotFound
		}
		if room.Status == StatusFinished || room.Status == StatusCancelled {
			return room, nil
		}
		room.Status = StatusFinished
		_ = s.repo.Save(ctx, room)
		return room, nil
	})
}

func (s *Service) LeaveOnDisconnect(ctx context.Context, roomID, userID string) (Room, error) {
	return s.withRoomLock(ctx, roomID, func() (Room, error) {
		room, ok := s.repo.Get(ctx, roomID)
		if !ok {
			return Room{}, ErrRoomNotFound
		}
		if !slices.Contains(room.Players, userID) {
			return room, nil
		}
		room.Players = slices.DeleteFunc(room.Players, func(id string) bool { return id == userID })
		room.ReadyUsers = slices.DeleteFunc(room.ReadyUsers, func(id string) bool { return id == userID })

		// Lifecycle rule: if a real player leaves, room cannot continue.
		if room.Status == StatusInGame || room.Status == StatusConfirmed || room.Status == StatusWaiting {
			if !shouldAutofillWithBot(room) && len(room.Players) < 2 {
				room.Status = StatusCancelled
				room.MatchID = ""
			}
		}

		_ = s.repo.Save(ctx, room)
		return room, nil
	})
}

func (s *Service) CancelStaleRooms(ctx context.Context, maxWait time.Duration) int {
	roomList, err := s.repo.List(ctx)
	if err != nil {
		return 0
	}
	cancelled := 0
	now := time.Now().UTC()
	for _, room := range roomList {
		if room.Status != StatusWaiting {
			continue
		}
		if len(room.ReadyUsers) > 0 {
			continue
		}
		if now.Sub(room.CreatedAt) < maxWait {
			continue
		}
		_, lockErr := s.withRoomLock(ctx, room.ID, func() (Room, error) {
			current, ok := s.repo.Get(ctx, room.ID)
			if !ok {
				return Room{}, ErrRoomNotFound
			}
			if current.Status != StatusWaiting {
				return current, nil
			}
			if len(current.ReadyUsers) > 0 {
				return current, nil
			}
			if now.Sub(current.CreatedAt) < maxWait {
				return current, nil
			}
			current.Status = StatusCancelled
			_ = s.repo.Save(ctx, current)
			cancelled++
			metrics.IncRoomCancelled()
			return current, nil
		})
		if lockErr != nil {
			continue
		}
	}
	return cancelled
}

const botPlayerPrefix = "bot:"

func BotPlayerID(roomID string) string {
	return botPlayerPrefix + roomID
}

func IsBotPlayer(userID string) bool {
	return strings.HasPrefix(userID, botPlayerPrefix)
}

// ContainsBotPlayerIn returns true if any of the player IDs is a bot.
func ContainsBotPlayerIn(playerIDs []string) bool {
	for _, id := range playerIDs {
		if IsBotPlayer(id) {
			return true
		}
	}
	return false
}

func shouldAutofillWithBot(room Room) bool {
	return room.MaxPlayers == 1 || strings.EqualFold(room.Mode, "bot")
}

var errRoomLocked = errors.New("room is locked")

func (s *Service) withRoomLock(ctx context.Context, roomID string, fn func() (Room, error)) (Room, error) {
	for attempt := 0; attempt < 8; attempt++ {
		release, err := s.acquireRedisLock(ctx, roomID)
		if err != nil {
			if errors.Is(err, errRoomLocked) {
				time.Sleep(25 * time.Millisecond)
				continue
			}
			return Room{}, err
		}
		defer release()
		return fn()
	}
	return Room{}, errRoomLocked
}

func (s *Service) acquireRedisLock(ctx context.Context, roomID string) (func(), error) {
	key := "lock:room:" + roomID
	token := fmt.Sprintf("%d", rand.Int63())
	ok, err := s.repo.redis.SetNX(ctx, key, token, 10*time.Second).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errRoomLocked
	}
	return func() {
		_ = s.repo.redis.Eval(ctx, `
if redis.call("GET", KEYS[1]) == ARGV[1] then
 return redis.call("DEL", KEYS[1])
end
return 0`, []string{key}, token).Err()
	}, nil
}
