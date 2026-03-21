package worker

import (
	"context"
	"log"

	"github.com/robfig/cron/v3"
	"github.com/teachingassistant/billing-service/internal/domain"
)

type ResetCreditsWorker struct {
	subRepo    domain.SubscriptionRepository
	walletRepo domain.WalletRepository
}

func NewResetCreditsWorker(subRepo domain.SubscriptionRepository, walletRepo domain.WalletRepository) *ResetCreditsWorker {
	return &ResetCreditsWorker{
		subRepo:    subRepo,
		walletRepo: walletRepo,
	}
}

func (w *ResetCreditsWorker) Start() *cron.Cron {
	c := cron.New()
	
	// Run everyday at midnight
	_, err := c.AddFunc("0 0 * * *", func() {
		w.run(context.Background())
	})
	
	if err != nil {
		log.Printf("Failed to start ResetCreditsWorker: %v", err)
		return nil
	}
	
	c.Start()
	log.Println("ResetCreditsWorker started")
	return c
}

func (w *ResetCreditsWorker) run(ctx context.Context) {
	subs, err := w.subRepo.GetActiveSubscriptions(ctx)
	if err != nil {
		log.Printf("ResetCreditsWorker failed to get active subscriptions: %v", err)
		return
	}

	for _, sub := range subs {
		wallet, err := w.walletRepo.GetByUserID(ctx, sub.UserID)
		if err != nil {
			log.Printf("ResetCreditsWorker failed to get wallet for user %s: %v", sub.UserID, err)
			continue
		}

		// Refill plan credits back to the base AI limit
		wallet.SetPlanBalance(sub.BaseAILimit)
		err = w.walletRepo.Update(ctx, wallet)
		if err != nil {
			log.Printf("ResetCreditsWorker failed to update wallet for user %s: %v", sub.UserID, err)
		}
	}
}
