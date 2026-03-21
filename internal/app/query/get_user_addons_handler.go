package query

import (
	"context"
	"time"

	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
)

// GetUserAddonsHandler returns the credit packs (AI & storage) owned by a user.
type GetUserAddonsHandler struct {
	walletRepo domain.WalletRepository
}

func NewGetUserAddonsHandler(walletRepo domain.WalletRepository) *GetUserAddonsHandler {
	return &GetUserAddonsHandler{walletRepo: walletRepo}
}

type UserAddonsDTO struct {
	AIPacks      []app.CreditPackDTO `json:"aiPacks"`
	StoragePacks []app.CreditPackDTO `json:"storagePacks"`
}

func (h *GetUserAddonsHandler) Handle(ctx context.Context, userID string) (*UserAddonsDTO, error) {
	wallet, err := h.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		wallet = &domain.Wallet{UserID: userID}
	}

	dto := &UserAddonsDTO{
		AIPacks:      make([]app.CreditPackDTO, 0),
		StoragePacks: make([]app.CreditPackDTO, 0),
	}

	now := time.Now()
	for _, pack := range wallet.CreditPacks {
		// Skip expired packs
		if !pack.ExpiresAt.IsZero() && pack.ExpiresAt.Before(now) {
			continue
		}
		cdto := app.CreditPackDTO{
			ID:              pack.ID,
			TotalAmount:     pack.TotalAmount,
			RemainingAmount: pack.RemainingAmount,
			Type:            pack.Type,
			PurchasedAt:     pack.PurchasedAt,
			ExpiresAt:       pack.ExpiresAt,
		}
		if pack.Type == "ai" {
			dto.AIPacks = append(dto.AIPacks, cdto)
		} else if pack.Type == "storage" {
			dto.StoragePacks = append(dto.StoragePacks, cdto)
		}
	}
	return dto, nil
}
