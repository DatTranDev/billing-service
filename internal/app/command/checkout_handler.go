package command

import (
	"context"

	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
)

type CreateCheckoutSessionHandler struct {
	stripePort app.StripePort
	subRepo    domain.SubscriptionRepository
}
func (h *CreateCheckoutSessionHandler) Stripe() app.StripePort {
	return h.stripePort
}

func NewCreateCheckoutSessionHandler(stripePort app.StripePort, subRepo domain.SubscriptionRepository) *CreateCheckoutSessionHandler {
	return &CreateCheckoutSessionHandler{stripePort: stripePort, subRepo: subRepo}
}

func (h *CreateCheckoutSessionHandler) Handle(ctx context.Context, cmd app.CheckoutParams) (string, error) {
	// Check if the user already has an active subscription
	sub, err := h.subRepo.GetByUserID(ctx, cmd.UserID)
	if err == nil && sub != nil {
		if sub.Status == domain.StatusActive || sub.Status == domain.StatusTrialing {
			cmd.StripeSubscriptionID = sub.StripeSubscriptionID
		}
		cmd.StripeCustomerID = sub.StripeCustomerID
	}

	return h.stripePort.CreateCheckoutSession(ctx, cmd)
}

type CreateAddonSessionHandler struct {
	stripePort app.StripePort
	subRepo    domain.SubscriptionRepository
}

func NewCreateAddonSessionHandler(stripePort app.StripePort, subRepo domain.SubscriptionRepository) *CreateAddonSessionHandler {
	return &CreateAddonSessionHandler{stripePort: stripePort, subRepo: subRepo}
}

func (h *CreateAddonSessionHandler) Handle(ctx context.Context, cmd app.AddonParams) (string, error) {
	sub, err := h.subRepo.GetByUserID(ctx, cmd.UserID)
	if err == nil && sub != nil {
		cmd.StripeCustomerID = sub.StripeCustomerID
	}
	return h.stripePort.CreateAddonSession(ctx, cmd)
}
