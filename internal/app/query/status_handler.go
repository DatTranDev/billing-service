package query

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
)

type BillingStatusDTO struct {
	Subscription struct {
		PlanID               string    `json:"planId"`
		PlanName             string    `json:"planName"`
		Status               string    `json:"status"`
		CurrentPeriodStart   time.Time `json:"currentPeriodStart"`
		CurrentPeriodEnd     time.Time `json:"currentPeriodEnd"`
		BaseAILimit          int       `json:"baseAiLimit"`
		BaseStorageQuota     int       `json:"baseStorageQuota"`
		AddonStorageQuota    int       `json:"addonStorageQuota"`
		StripeSubscriptionID string    `json:"stripeSubscriptionId,omitempty"`
		StripeCustomerID     string    `json:"stripeCustomerId,omitempty"`
	} `json:"subscription"`
	Usage struct {
		MonthlyAIUsed    int `json:"monthlyAiUsed"`
		MonthlyAILimit   int `json:"monthlyAiLimit"`
		PurchasedAIUsed  int `json:"purchasedAiUsed"`
		PurchasedAITotal int `json:"purchasedAiTotal"`

		MonthlyStorageUsed    int `json:"monthlyStorageUsed"`
		MonthlyStorageLimit   int `json:"monthlyStorageLimit"`
		PurchasedStorageUsed  int `json:"purchasedStorageUsed"`
		PurchasedStorageTotal int `json:"purchasedStorageTotal"`
	} `json:"usage"`
	AIAddons      []app.CreditPackDTO `json:"aiAddons"`
	StorageAddons []app.CreditPackDTO `json:"storageAddons"`
	Wallet        struct {
		LiquidBalance int `json:"liquidBalance"`
		PlanBalance   int `json:"planBalance"`
	} `json:"wallet"`
}

type GetBillingStatusHandler struct {
	subRepo    domain.SubscriptionRepository
	walletRepo domain.WalletRepository
	usageRepo  domain.UsageRepository
}

func NewGetBillingStatusHandler(
	subRepo domain.SubscriptionRepository,
	walletRepo domain.WalletRepository,
	usageRepo domain.UsageRepository,
) *GetBillingStatusHandler {
	return &GetBillingStatusHandler{
		subRepo:    subRepo,
		walletRepo: walletRepo,
		usageRepo:  usageRepo,
	}
}

func (h *GetBillingStatusHandler) Handle(ctx context.Context, userID string) (*BillingStatusDTO, error) {
	slog.Info("fetching billing status", "userId", userID)

	sub, err := h.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		slog.Warn("failed to fetch subscription", "userId", userID, "error", err)
		return nil, domain.ErrSubscriptionNotFound
	}

	wallet, err := h.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		slog.Warn("failed to fetch wallet, using empty wallet", "userId", userID, "error", err)
		wallet = &domain.Wallet{UserID: userID}
	}

	storageUsed, _ := h.usageRepo.GetTotalUsage(ctx, userID, domain.UsageTypeStorageByte, time.Time{})
	aiUsed, _ := h.usageRepo.GetTotalUsage(ctx, userID, domain.UsageTypeAITokens, time.Time{})

	// Prefer the wallet storage snapshot if available (faster, atomic)
	if wallet.StorageUsedBytes > 0 {
		storageUsed = wallet.StorageUsedBytes
	}

	dto := &BillingStatusDTO{}
	dto.AIAddons = []app.CreditPackDTO{}
	dto.StorageAddons = []app.CreditPackDTO{}
	dto.Subscription.PlanID = sub.PlanID
	dto.Subscription.PlanName = sub.PlanName
	dto.Subscription.Status = string(sub.Status)
	dto.Subscription.CurrentPeriodStart = sub.CurrentPeriodStart
	dto.Subscription.CurrentPeriodEnd = sub.CurrentPeriodEnd
	dto.Subscription.BaseAILimit = sub.BaseAILimit
	dto.Subscription.BaseStorageQuota = sub.BaseStorageQuota
	dto.Subscription.AddonStorageQuota = sub.AddonStorageQuota
	if sub.StripePlanSubscriptionID != "" {
		dto.Subscription.StripeSubscriptionID = sub.StripePlanSubscriptionID
	} else {
		dto.Subscription.StripeSubscriptionID = sub.StripeSubscriptionID
	}
	dto.Subscription.StripeCustomerID = sub.StripeCustomerID

	dto.Usage.MonthlyAIUsed = int(math.Min(float64(aiUsed), float64(sub.BaseAILimit)))
	dto.Usage.MonthlyAILimit = sub.BaseAILimit
	dto.Usage.PurchasedAIUsed = int(math.Max(0, float64(aiUsed-sub.BaseAILimit)))

	purchasedAITotal := 0
	for _, pack := range wallet.CreditPacks {
		if pack.Type == "ai" {
			purchasedAITotal += pack.TotalAmount
			dto.AIAddons = append(dto.AIAddons, app.CreditPackDTO{
				ID:              pack.ID,
				TotalAmount:     pack.TotalAmount,
				RemainingAmount: pack.RemainingAmount,
				Type:            pack.Type,
				PurchasedAt:     pack.PurchasedAt,
				ExpiresAt:       pack.ExpiresAt,
			})
		} else if pack.Type == "storage" {
			dto.StorageAddons = append(dto.StorageAddons, app.CreditPackDTO{
				ID:              pack.ID,
				TotalAmount:     pack.TotalAmount,
				RemainingAmount: pack.RemainingAmount,
				Type:            pack.Type,
				PurchasedAt:     pack.PurchasedAt,
				ExpiresAt:       pack.ExpiresAt,
			})
		}
	}
	dto.Usage.PurchasedAITotal = purchasedAITotal

	dto.Usage.MonthlyStorageUsed = storageUsed
	dto.Usage.MonthlyStorageLimit = sub.BaseStorageQuota
	dto.Usage.PurchasedStorageUsed = 0 // For now
	dto.Usage.PurchasedStorageTotal = sub.AddonStorageQuota

	dto.Wallet.LiquidBalance = wallet.LiquidBalance
	dto.Wallet.PlanBalance = wallet.PlanBalance

	slog.Info("billing status constructed successfully", "userId", userID, "planId", dto.Subscription.PlanID)
	return dto, nil
}
