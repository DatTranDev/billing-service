package domain

import (
	"context"
	"time"
)

type LedgerEntryType string

const (
	LedgerTypePayment LedgerEntryType = "payment"
	LedgerTypeRefund  LedgerEntryType = "refund"
)

type LedgerEntry struct {
	ID        string          `bson:"id"`
	UserID    string          `bson:"userId"`
	Amount    int64           `bson:"amount"` // in minor units
	Currency  string          `bson:"currency"`
	Type      LedgerEntryType `bson:"type"`
	StripeID  string          `bson:"stripeId"` // ID of PaymentIntent or Charge
	Metadata  map[string]string `bson:"metadata"`
	CreatedAt time.Time       `bson:"createdAt"`
}

type LedgerRepository interface {
	Save(ctx context.Context, entry *LedgerEntry) error
	GetByUserID(ctx context.Context, userID string) ([]*LedgerEntry, error)
}
