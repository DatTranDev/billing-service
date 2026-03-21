package command

import (
	"context"

	"github.com/stripe/stripe-go/v76"
	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
)

type DirectChargeHandler struct {
	stripePort app.StripePort
	subRepo    domain.SubscriptionRepository
}

func NewDirectChargeHandler(stripePort app.StripePort, subRepo domain.SubscriptionRepository) *DirectChargeHandler {
	return &DirectChargeHandler{stripePort: stripePort, subRepo: subRepo}
}

type DirectChargeCmd struct {
	UserID          string `json:"userId"`
	PriceRef        string `json:"priceRef"`
	Quantity        int    `json:"quantity"`
	PaymentMethodID string `json:"paymentMethodId"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

func (h *DirectChargeHandler) Handle(ctx context.Context, cmd DirectChargeCmd) (*stripe.PaymentIntent, error) {
	// Resolve Stripe Customer ID
	sub, err := h.subRepo.GetByUserID(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}
	if sub == nil || sub.StripeCustomerID == "" {
		return nil, domain.ErrSubscriptionNotFound
	}

	metadata := map[string]string{
		"userId": cmd.UserID,
	}

	return h.stripePort.DirectCharge(ctx, sub.StripeCustomerID, cmd.PriceRef, cmd.Quantity, cmd.PaymentMethodID, metadata, cmd.IdempotencyKey)
}

type DirectSubscriptionHandler struct {
	stripePort app.StripePort
	subRepo    domain.SubscriptionRepository
}

func NewDirectSubscriptionHandler(stripePort app.StripePort, subRepo domain.SubscriptionRepository) *DirectSubscriptionHandler {
	return &DirectSubscriptionHandler{stripePort: stripePort, subRepo: subRepo}
}

type DirectSubscriptionCmd struct {
	UserID          string `json:"userId"`
	PriceRef        string `json:"priceRef"`
	SubType         string `json:"subType"` // "plan" or "storage"
	PaymentMethodID string `json:"paymentMethodId"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

func (h *DirectSubscriptionHandler) Handle(ctx context.Context, cmd DirectSubscriptionCmd) (*stripe.Subscription, error) {
	// Resolve Stripe Customer ID
	sub, err := h.subRepo.GetByUserID(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}
	if sub == nil || sub.StripeCustomerID == "" {
		return nil, domain.ErrSubscriptionNotFound
	}

	metadata := map[string]string{
		"userId":   cmd.UserID,
		"sub_type": cmd.SubType,
	}

	return h.stripePort.DirectSubscription(ctx, sub.StripeCustomerID, cmd.PriceRef, cmd.PaymentMethodID, metadata, cmd.IdempotencyKey)
}
