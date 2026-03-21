package command

import (
	"context"
	"fmt"

	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
	"github.com/stripe/stripe-go/v76"
)

type SetupFreeTierHandler struct {
	stripe     app.StripePort
	subRepo    domain.SubscriptionRepository
	walletRepo domain.WalletRepository
}

func NewSetupFreeTierHandler(stripe app.StripePort, subRepo domain.SubscriptionRepository, walletRepo domain.WalletRepository) *SetupFreeTierHandler {
	return &SetupFreeTierHandler{
		stripe:     stripe,
		subRepo:    subRepo,
		walletRepo: walletRepo,
	}
}

type SetupFreeTierParams struct {
	UserID    string      `json:"userId"`
	Email     string      `json:"email"`
	FirstName string      `json:"firstName"`
	LastName  string      `json:"lastName"`
	Address   app.Address `json:"address"`
	Timezone  string      `json:"timezone"`
	Country   string      `json:"country"`
}

func (h *SetupFreeTierHandler) Handle(ctx context.Context, params SetupFreeTierParams) error {
	// 1. Get or Create Stripe Customer
	name := fmt.Sprintf("%s %s", params.FirstName, params.LastName)
	metadata := map[string]string{
		"userId":   params.UserID,
		"timezone": params.Timezone,
		"country":  params.Country,
	}

	cus, err := h.stripe.FindCustomerByEmail(ctx, params.Email)
	if err != nil {
		return fmt.Errorf("failed to check for existing stripe customer: %w", err)
	}

	if cus == nil {
		cus, err = h.stripe.CreateCustomer(ctx, params.Email, name, metadata, fmt.Sprintf("setup-free-cus-%s", params.UserID))
		if err != nil {
			return fmt.Errorf("failed to create stripe customer: %w", err)
		}
	}

	// 2. Update customer with address
	cusParams := &stripe.CustomerParams{
		Address: &stripe.AddressParams{
			Line1:      stripe.String(params.Address.Line1),
			City:       stripe.String(params.Address.City),
			State:      stripe.String(params.Address.State),
			PostalCode: stripe.String(params.Address.PostalCode),
			Country:    stripe.String(params.Address.Country),
		},
	}
	_, err = h.stripe.UpdateCustomer(ctx, cus.ID, cusParams, fmt.Sprintf("setup-free-cus-update-%s", params.UserID))
	if err != nil {
		return fmt.Errorf("failed to update stripe customer with address: %w", err)
	}

	// 3. Subscribe to Free Tier
	freePriceRef := "plan_free"

	sub, err := h.stripe.CreateSubscription(ctx, cus.ID, freePriceRef, metadata, fmt.Sprintf("setup-free-sub-%s", params.UserID))
	if err != nil {
		return fmt.Errorf("failed to create stripe subscription: %w", err)
	}

	// 4. Initialize Local Data
	newSub := domain.NewSubscription(params.UserID, cus.ID, sub.ID, "Free", 512*1024*1024, 100)
	if err := h.subRepo.Save(ctx, newSub); err != nil {
		return fmt.Errorf("failed to save local subscription: %w", err)
	}

	wallet := domain.NewWallet(params.UserID)
	wallet.SetPlanBalance(100)
	if err := h.walletRepo.Update(ctx, wallet); err != nil {
		return fmt.Errorf("failed to update local wallet: %w", err)
	}

	return nil
}
