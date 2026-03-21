package command

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
)

type PaymentMethodHandler struct {
	stripe  app.StripePort
	subRepo domain.SubscriptionRepository
}

func NewPaymentMethodHandler(stripe app.StripePort, subRepo domain.SubscriptionRepository) *PaymentMethodHandler {
	return &PaymentMethodHandler{
		stripe:  stripe,
		subRepo: subRepo,
	}
}

func (h *PaymentMethodHandler) SetDefault(ctx context.Context, userID, pmID string) error {
	sub, err := h.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	return h.stripe.SetDefaultPaymentMethod(ctx, sub.StripeCustomerID, pmID)
}

func (h *PaymentMethodHandler) Detach(ctx context.Context, pmID string) error {
	_, err := h.stripe.DetachPaymentMethod(ctx, pmID)
	return err
}
