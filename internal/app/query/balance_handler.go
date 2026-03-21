package query

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/domain"
)

type GetWalletBalanceHandler struct {
	repo domain.WalletRepository
}

func NewGetWalletBalanceHandler(repo domain.WalletRepository) *GetWalletBalanceHandler {
	return &GetWalletBalanceHandler{repo: repo}
}

func (h *GetWalletBalanceHandler) Handle(ctx context.Context, userID string) (int, error) {
	wallet, err := h.repo.GetByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return wallet.Balance(), nil
}
