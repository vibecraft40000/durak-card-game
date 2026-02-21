package wallet

import (
	"context"

	"durakonline/backend/internal/transactions"
)

type Repository struct {
	txRepo *transactions.Repository
}

func NewRepository(txRepo *transactions.Repository) *Repository {
	return &Repository{txRepo: txRepo}
}

func (r *Repository) Balance(ctx context.Context, userID string) (float64, error) {
	return r.txRepo.Balance(ctx, userID)
}
