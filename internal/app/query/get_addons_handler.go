package query

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/app"
)

type GetAddonsHandler struct {
	stripePort app.StripePort
}

func NewGetAddonsHandler(stripePort app.StripePort) *GetAddonsHandler {
	return &GetAddonsHandler{
		stripePort: stripePort,
	}
}

func (h *GetAddonsHandler) Handle(ctx context.Context) ([]app.PlanDTO, error) {
	return h.stripePort.ListActiveAddons(ctx)
}
