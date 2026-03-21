package query

import (
	"context"
	"errors"
	"time"

	"github.com/teachingassistant/billing-service/internal/domain"
)

var ErrUsageExceeded = errors.New("USAGE_EXCEEDED")

type CheckUsageHandler struct {
	walletRepo domain.WalletRepository
	subRepo    domain.SubscriptionRepository
	usageRepo  domain.UsageRepository
}

func NewCheckUsageHandler(
	walletRepo domain.WalletRepository,
	subRepo domain.SubscriptionRepository,
	usageRepo domain.UsageRepository,
) *CheckUsageHandler {
	return &CheckUsageHandler{
		walletRepo: walletRepo,
		subRepo:    subRepo,
		usageRepo:  usageRepo,
	}
}

func (h *CheckUsageHandler) GetSubscription(ctx context.Context, userID string) (*domain.Subscription, error) {
	return h.subRepo.GetByUserID(ctx, userID)
}

func (h *CheckUsageHandler) Handle(ctx context.Context, userID string, usageType domain.UsageType, amount int) error {
	if usageType == domain.UsageTypeAITokens {
		wallet, err := h.walletRepo.GetByUserID(ctx, userID)
		if err != nil {
			return err
		}
		if wallet.Balance() < amount {
			return ErrUsageExceeded
		}
		return nil
	}

	if usageType == domain.UsageTypeStorageByte {
		sub, err := h.subRepo.GetByUserID(ctx, userID)
		if err != nil {
			// If no sub found, assume free tier limits
			sub = &domain.Subscription{Status: domain.StatusCanceled}
		}

		limit := sub.GetTotalStorageLimit()

		totalUsage, err := h.usageRepo.GetTotalUsage(ctx, userID, domain.UsageTypeStorageByte, time.Time{})
		if err != nil {
			return err
		}

		if totalUsage+amount > limit {
			return ErrUsageExceeded
		}
		return nil
	}

	return errors.New("unknown usage type")
}
