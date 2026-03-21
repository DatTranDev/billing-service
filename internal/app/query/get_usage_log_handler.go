package query

import (
	"context"
	"time"

	"github.com/teachingassistant/billing-service/internal/domain"
)

// UsageEventDTO is the API representation of a usage event.
type UsageEventDTO struct {
	Type        string    `json:"type"`
	Amount      int       `json:"amount"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

// GetUsageLogHandler lists recent usage events for a user.
type GetUsageLogHandler struct {
	usageRepo domain.UsageRepository
}

func NewGetUsageLogHandler(usageRepo domain.UsageRepository) *GetUsageLogHandler {
	return &GetUsageLogHandler{usageRepo: usageRepo}
}

const defaultUsageLogLimit = 50

func (h *GetUsageLogHandler) Handle(ctx context.Context, userID string, usageType domain.UsageType, limit int) ([]UsageEventDTO, error) {
	if limit <= 0 {
		limit = defaultUsageLogLimit
	}

	events, err := h.usageRepo.ListUsage(ctx, userID, usageType, limit)
	if err != nil {
		return nil, err
	}

	dtos := make([]UsageEventDTO, 0, len(events))
	for _, e := range events {
		dtos = append(dtos, UsageEventDTO{
			Type:        string(e.Type),
			Amount:      e.Amount,
			Description: e.Description,
			Timestamp:   e.Timestamp,
		})
	}
	return dtos, nil
}
