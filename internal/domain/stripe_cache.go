package domain

import (
	"context"
	"time"
)

type StripeObjectType string

const (
	StripeObjectCustomer      StripeObjectType = "customer"
	StripeObjectSubscription  StripeObjectType = "subscription"
	StripeObjectPaymentMethod StripeObjectType = "payment_method"
)

type StripeCacheEntry struct {
	ID         string           `bson:"id"`
	Type       StripeObjectType `bson:"type"`
	Data       interface{}      `bson:"data"`
	LastUpdate time.Time        `bson:"lastUpdate"`
}

type StripeCacheRepository interface {
	Save(ctx context.Context, entry *StripeCacheEntry) error
	GetByID(ctx context.Context, id string) (*StripeCacheEntry, error)
	Delete(ctx context.Context, id string) error
}
