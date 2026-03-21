package command

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
	"github.com/stripe/stripe-go/v76"
)

type CreateSetupIntentHandler struct {
	stripe   app.StripePort
	subRepo  domain.SubscriptionRepository
}

func NewCreateSetupIntentHandler(stripe app.StripePort, subRepo domain.SubscriptionRepository) *CreateSetupIntentHandler {
	return &CreateSetupIntentHandler{
		stripe:  stripe,
		subRepo: subRepo,
	}
}

func (h *CreateSetupIntentHandler) Handle(ctx context.Context, userID string) (*stripe.SetupIntent, error) {
	sub, err := h.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if sub.StripeCustomerID == "" {
		return nil, domain.ErrSubscriptionNotFound // Or specific ErrCustomerNotFound
	}

	return h.stripe.CreateSetupIntent(ctx, sub.StripeCustomerID)
}
