package query

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
	"github.com/stripe/stripe-go/v76"
)

type GetPaymentMethodsHandler struct {
	stripe  app.StripePort
	subRepo domain.SubscriptionRepository
}

func NewGetPaymentMethodsHandler(stripe app.StripePort, subRepo domain.SubscriptionRepository) *GetPaymentMethodsHandler {
	return &GetPaymentMethodsHandler{
		stripe:  stripe,
		subRepo: subRepo,
	}
}

func (h *GetPaymentMethodsHandler) Handle(ctx context.Context, userID string) ([]*stripe.PaymentMethod, error) {
	sub, err := h.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return h.stripe.ListPaymentMethods(ctx, sub.StripeCustomerID)
}
