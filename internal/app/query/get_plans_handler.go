package query

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/app"
)

type GetPlansHandler struct {
	stripePort app.StripePort
}

func NewGetPlansHandler(stripePort app.StripePort) *GetPlansHandler {
	return &GetPlansHandler{
		stripePort: stripePort,
	}
}

func (h *GetPlansHandler) Handle(ctx context.Context) ([]app.PlanDTO, error) {
	return h.stripePort.ListActivePlans(ctx)
}
