package games

import (
	"context"

	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/wallet"
)

func SettleIfFinished(ctx context.Context, walletService *wallet.Service, state engine.GameState, stake float64, commissionBps int) error {
	if state.Status != engine.StatusFinished || state.WinnerPlayer == "" {
		return nil
	}
	pot := stake * 2
	return walletService.SettleWin(ctx, state.WinnerPlayer, state.MatchID, pot, commissionBps)
}
