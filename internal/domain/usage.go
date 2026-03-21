package domain

import (
	"context"
	"time"
)

type UsageType string

const (
	UsageTypeAITokens    UsageType = "AI_TOKEN"
	UsageTypeStorageByte UsageType = "STORAGE_BYTE"
)

type UsageEvent struct {
	UserID      string    `bson:"userId"`
	Type        UsageType `bson:"type"`
	Amount      int       `bson:"amount"`
	Description string    `bson:"description"`
	Timestamp   time.Time `bson:"timestamp"`
}

type UsageRepository interface {
	Log(ctx context.Context, event UsageEvent) error
	GetTotalUsage(ctx context.Context, userID string, usageType UsageType, since time.Time) (int, error)
	ListUsage(ctx context.Context, userID string, usageType UsageType, limit int) ([]UsageEvent, error)
}
