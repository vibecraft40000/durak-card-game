package friends

import (
	"context"
	"errors"
)

var ErrSelfFriend = errors.New("cannot add yourself as friend")

type Service struct {
	repo     *Repository
	userRepo UserResolver
}

type UserResolver interface {
	ResolveID(ctx context.Context, idOrUsername string) (string, bool)
}

func NewService(repo *Repository, userRepo UserResolver) *Service {
	return &Service{repo: repo, userRepo: userRepo}
}

func (s *Service) SendRequest(ctx context.Context, userID string, friendIDOrUsername string) error {
	friendID, ok := s.userRepo.ResolveID(ctx, friendIDOrUsername)
	if !ok {
		return errors.New("user not found")
	}
	if friendID == userID {
		return ErrSelfFriend
	}
	return s.repo.CreateRequest(ctx, userID, friendID)
}

func (s *Service) AcceptRequest(ctx context.Context, userID, requestUserID string) error {
	return s.repo.Accept(ctx, userID, requestUserID)
}

func (s *Service) ListFriends(ctx context.Context, userID string) ([]Friend, error) {
	return s.repo.ListAccepted(ctx, userID)
}

func (s *Service) ListIncomingRequests(ctx context.Context, userID string) ([]Friend, error) {
	return s.repo.ListIncomingRequests(ctx, userID)
}
