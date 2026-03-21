package query

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
	"github.com/stripe/stripe-go/v76"
)

type ListInvoicesHandler struct {
	stripe  app.StripePort
	subRepo domain.SubscriptionRepository
}

func NewListInvoicesHandler(stripe app.StripePort, subRepo domain.SubscriptionRepository) *ListInvoicesHandler {
	return &ListInvoicesHandler{
		stripe:  stripe,
		subRepo: subRepo,
	}
}

func (h *ListInvoicesHandler) Handle(ctx context.Context, userID string, limit int) ([]*stripe.Invoice, error) {
	sub, err := h.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return h.stripe.ListInvoices(ctx, sub.StripeCustomerID, limit)
}
