package command

import (
	"context"

	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
)

type CreatePortalSessionHandler struct {
	stripePort app.StripePort
	subRepo    domain.SubscriptionRepository
}

func NewCreatePortalSessionHandler(stripePort app.StripePort, subRepo domain.SubscriptionRepository) *CreatePortalSessionHandler {
	return &CreatePortalSessionHandler{stripePort: stripePort, subRepo: subRepo}
}

func (h *CreatePortalSessionHandler) Handle(ctx context.Context, userID, returnURL string) (string, error) {
	// Look up the user's active subscription to get the Stripe customer ID
	sub, err := h.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	if sub == nil || sub.StripeCustomerID == "" {
		return "", domain.ErrNotFound
	}

	return h.stripePort.CreatePortalSession(ctx, sub.StripeCustomerID, returnURL)
}
