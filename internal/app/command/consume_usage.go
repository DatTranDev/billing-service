package command

import (
	"context"
	"errors"
	"time"

	"github.com/teachingassistant/billing-service/internal/domain"
)

var ErrUsageExceeded = errors.New("USAGE_EXCEEDED")

type ConsumeUsageHandler struct {
	walletRepo domain.WalletRepository
	subRepo    domain.SubscriptionRepository
	usageRepo  domain.UsageRepository
}

func NewConsumeUsageHandler(
	walletRepo domain.WalletRepository,
	subRepo domain.SubscriptionRepository,
	usageRepo domain.UsageRepository,
) *ConsumeUsageHandler {
	return &ConsumeUsageHandler{
		walletRepo: walletRepo,
		subRepo:    subRepo,
		usageRepo:  usageRepo,
	}
}

func (h *ConsumeUsageHandler) Handle(ctx context.Context, userID string, usageType domain.UsageType, amount int, description string) error {
	if amount <= 0 {
		return errors.New("invalid amount")
	}

	wallet, err := h.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	now := time.Now()
	remaining := amount

	switch usageType {
	case domain.UsageTypeAITokens:
		for i := range wallet.CreditPacks {
			pack := &wallet.CreditPacks[i]
			if pack.Type != "ai" || pack.RemainingAmount <= 0 {
				continue
			}
			if !pack.ExpiresAt.IsZero() && pack.ExpiresAt.Before(now) {
				continue
			}

			take := remaining
			if pack.RemainingAmount < take {
				take = pack.RemainingAmount
			}
			pack.RemainingAmount -= take
			remaining -= take
			if remaining == 0 {
				break
			}
		}

		if remaining > 0 {
			if wallet.Balance() < remaining {
				return ErrUsageExceeded
			}
			if err := wallet.Deduct(remaining); err != nil {
				if errors.Is(err, domain.ErrInsufficientBalance) {
					return ErrUsageExceeded
				}
				return err
			}
		}

	case domain.UsageTypeStorageByte:
		sub, err := h.subRepo.GetByUserID(ctx, userID)
		if err != nil || sub == nil {
			sub = &domain.Subscription{Status: domain.StatusCanceled}
		}

		storageUsed := wallet.StorageUsedBytes
		if storageUsed <= 0 {
			usedFromLog, usageErr := h.usageRepo.GetTotalUsage(ctx, userID, domain.UsageTypeStorageByte, time.Time{})
			if usageErr == nil {
				storageUsed = usedFromLog
			}
		}

		baseLimit := sub.GetTotalStorageLimit()
		baseRemaining := baseLimit - storageUsed
		if baseRemaining < 0 {
			baseRemaining = 0
		}

		packRemaining := 0
		for i := range wallet.CreditPacks {
			pack := &wallet.CreditPacks[i]
			if pack.Type != "storage" || pack.RemainingAmount <= 0 {
				continue
			}
			if !pack.ExpiresAt.IsZero() && pack.ExpiresAt.Before(now) {
				continue
			}
			packRemaining += pack.RemainingAmount
		}

		if amount > baseRemaining+packRemaining {
			return ErrUsageExceeded
		}

		for i := range wallet.CreditPacks {
			pack := &wallet.CreditPacks[i]
			if remaining == 0 {
				break
			}
			if pack.Type != "storage" || pack.RemainingAmount <= 0 {
				continue
			}
			if !pack.ExpiresAt.IsZero() && pack.ExpiresAt.Before(now) {
				continue
			}

			take := remaining
			if pack.RemainingAmount < take {
				take = pack.RemainingAmount
			}
			pack.RemainingAmount -= take
			remaining -= take
		}

		if remaining > 0 {
			wallet.StorageUsedBytes = storageUsed + remaining
		}

	default:
		return errors.New("unknown usage type")
	}

	if err := h.walletRepo.Update(ctx, wallet); err != nil {
		return err
	}

	if description == "" {
		description = "usage consumed"
	}

	return h.usageRepo.Log(ctx, domain.UsageEvent{
		UserID:      userID,
		Type:        usageType,
		Amount:      amount,
		Description: description,
		Timestamp:   now,
	})
}