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
	subRepo    domain.SubscriptionRepository
}

func NewGetUserAddonsHandler(walletRepo domain.WalletRepository, subRepo domain.SubscriptionRepository) *GetUserAddonsHandler {
	return &GetUserAddonsHandler{walletRepo: walletRepo, subRepo: subRepo}
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

	sub, err := h.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		sub = nil
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

	if sub != nil && sub.AddonStorageQuota > 0 {
		hasStoragePack := len(dto.StoragePacks) > 0
		if !hasStoragePack {
			dto.StoragePacks = append(dto.StoragePacks, app.CreditPackDTO{
				ID:              "sub_storage_quota",
				TotalAmount:     sub.AddonStorageQuota,
				RemainingAmount: sub.AddonStorageQuota,
				Type:            "storage",
				PurchasedAt:     sub.CurrentPeriodStart,
				ExpiresAt:       sub.CurrentPeriodEnd,
			})
		}
	}

	return dto, nil
}
