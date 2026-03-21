package command

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/domain"
)

type DeductCreditsHandler struct {
	repo domain.WalletRepository
}

func NewDeductCreditsHandler(repo domain.WalletRepository) *DeductCreditsHandler {
	return &DeductCreditsHandler{repo: repo}
}

func (h *DeductCreditsHandler) Handle(ctx context.Context, userID string, amount int) error {
	wallet, err := h.repo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if err := wallet.Deduct(amount); err != nil {
		return err
	}
	return h.repo.Update(ctx, wallet)
}
