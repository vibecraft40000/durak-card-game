package rooms

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"slices"
	"strings"
	"time"

	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/games"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/pkg/metrics"

	"github.com/google/uuid"
)

var (
	ErrRoomNotFound    = errors.New("room not found")
	ErrRoomFull        = errors.New("room full")
	ErrNeedOpponent    = errors.New("waiting for opponent")
	ErrNotRoomCreator  = errors.New("only room creator can start the game")
	ErrNotAllReady     = errors.New("wait for all players to confirm")
	ErrStartInProgress = errors.New("match start already in progress")
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
		if err := s.repo.Save(ctx, room); err != nil {
			return Room{}, fmt.Errorf("save room: %w", err)
		}
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
			_ = s.repo.AddReadyUser(ctx, roomID, botID)
		}
		_ = s.repo.AddReadyUser(ctx, roomID, userID)
		room.ReadyUsers, _ = s.repo.GetReadyUsers(ctx, roomID)
		requiredPlayers := room.MaxPlayers
		if shouldAutofillWithBot(room) {
			requiredPlayers = 2
		}
		willStart := len(room.Players) == requiredPlayers && len(room.ReadyUsers) == requiredPlayers && room.Status == StatusWaiting
		log.Printf("[ready] room=%s user=%s players=%d ready=%d status=%s willStart=%v",
			roomID, userID, len(room.Players), len(room.ReadyUsers), room.Status,
			willStart)
		if !shouldAutofillWithBot(room) && len(room.Players) < 2 {
			if err := s.repo.Save(ctx, room); err != nil {
				return Room{}, fmt.Errorf("save room: %w", err)
			}
			return room, ErrNeedOpponent
		}
		// Auto-start when all configured players joined and confirmed.
		if willStart {
			return s.startMatchLocked(ctx, room)
		}
		if err := s.repo.Save(ctx, room); err != nil {
			return Room{}, fmt.Errorf("save room: %w", err)
		}
		return room, nil
	})
}

// StartGame starts the match. Only the room creator can call this, and only when all players have confirmed.
func (s *Service) StartGame(ctx context.Context, roomID string, userID string) (Room, error) {
	return s.withRoomLock(ctx, roomID, func() (Room, error) {
		room, ok := s.repo.Get(ctx, roomID)
		if !ok {
			return Room{}, ErrRoomNotFound
		}
		if len(room.Players) == 0 {
			return Room{}, ErrRoomNotFound
		}
		if room.Players[0] != userID {
			return Room{}, ErrNotRoomCreator
		}
		requiredPlayers := room.MaxPlayers
		if shouldAutofillWithBot(room) {
			requiredPlayers = 2
		}
		if len(room.Players) != requiredPlayers || len(room.ReadyUsers) != requiredPlayers || room.Status != StatusWaiting {
			return Room{}, ErrNotAllReady
		}
		return s.startMatchLocked(ctx, room)
	})
}

func (s *Service) startMatchLocked(ctx context.Context, room Room) (Room, error) {
	log.Printf("[start] entering room=%s", room.ID)
	startKey := "room:" + room.ID + ":starting"
	ok, err := s.repo.redis.SetNX(ctx, startKey, "1", 10*time.Second).Result()
	if err != nil || !ok {
		log.Printf("[start] SetNX failed room=%s err=%v ok=%v", room.ID, err, ok)
		updated, _ := s.repo.Get(ctx, room.ID)
		if updated.MatchID != "" {
			return updated, nil
		}
		return Room{}, ErrStartInProgress
	}
	room.Status = StatusConfirmed
	matchID := uuid.NewString()
	heldPlayers := make([]string, 0, len(room.Players))
	rollbackHolds := func() {
		for _, playerID := range heldPlayers {
			if err := s.wallet.RollbackHoldBet(ctx, playerID, matchID); err != nil {
				log.Printf("[start] rollback hold failed room=%s match=%s player=%s err=%v", room.ID, matchID, playerID, err)
			}
		}
	}
	if !s.disableMoney {
		for _, playerID := range room.Players {
			if IsBotPlayer(playerID) {
				continue
			}
			if err := s.wallet.HoldBet(ctx, playerID, matchID, room.Stake); err != nil {
				log.Printf("[start] HoldBet failed room=%s player=%s err=%v", room.ID, playerID, err)
				rollbackHolds()
				_ = s.repo.ReleaseStartLock(ctx, room.ID)
				_ = s.repo.ClearReadySet(ctx, room.ID)
				return Room{}, err
			}
			heldPlayers = append(heldPlayers, playerID)
		}
	}
	cfg := engine.GameConfig{
		DeckSize: room.Deck,
		Mode:     room.Mode,
	}
	if _, err := s.games.StartMatchWithConfig(ctx, matchID, room.Stake, cfg, room.Players); err != nil {
		log.Printf("[start] StartMatch failed room=%s match=%s err=%v", room.ID, matchID, err)
		rollbackHolds()
		_ = s.repo.ReleaseStartLock(ctx, room.ID)
		_ = s.repo.ClearReadySet(ctx, room.ID)
		return Room{}, err
	}
	room.MatchID = matchID
	room.Status = StatusInGame
	if err := s.repo.Save(ctx, room); err != nil {
		return Room{}, fmt.Errorf("save room: %w", err)
	}
	_ = s.repo.ClearReadySet(ctx, room.ID)
	log.Printf("[start] match created room=%s match=%s", room.ID, matchID)
	return room, nil
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
		if err := s.repo.Save(ctx, room); err != nil {
			return Room{}, fmt.Errorf("save room: %w", err)
		}
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
		_ = s.repo.RemoveReadyUser(ctx, roomID, userID)
		room.ReadyUsers, _ = s.repo.GetReadyUsers(ctx, roomID)

		// Lifecycle rule: if a real player leaves, room cannot continue.
		if room.Status == StatusInGame || room.Status == StatusConfirmed || room.Status == StatusWaiting {
			if !shouldAutofillWithBot(room) && len(room.Players) < 2 {
				room.Status = StatusCancelled
				room.MatchID = ""
			}
		}

		if err := s.repo.Save(ctx, room); err != nil {
			return Room{}, fmt.Errorf("save room: %w", err)
		}
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
			if err := s.repo.Save(ctx, current); err != nil {
				return Room{}, fmt.Errorf("save room: %w", err)
			}
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

