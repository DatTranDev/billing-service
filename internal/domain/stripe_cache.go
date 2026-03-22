package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v76"
	"go.mongodb.org/mongo-driver/bson"
)

type StripeObjectType string

const (
	StripeObjectCustomer      StripeObjectType = "customer"
	StripeObjectSubscription  StripeObjectType = "subscription"
	StripeObjectPaymentMethod StripeObjectType = "payment_method"
	StripeObjectProduct       StripeObjectType = "product"
	StripeObjectPrice         StripeObjectType = "price"
)

type StripeCacheEntry struct {
	ID         string           `bson:"id"`
	Type       StripeObjectType `bson:"type"`
	Data       interface{}      `bson:"data"`
	LastUpdate time.Time        `bson:"lastUpdate"`
}

func (e *StripeCacheEntry) UnmarshalBSON(data []byte) error {
	type Alias StripeCacheEntry
	var aux struct {
		Data bson.Raw `bson:"data"`
		*Alias
	}
	aux.Alias = (*Alias)(e)

	if err := bson.Unmarshal(data, &aux); err != nil {
		return err
	}

	switch e.Type {
	case StripeObjectProduct:
		var p stripe.Product
		if err := bson.Unmarshal(aux.Data, &p); err != nil {
			return err
		}
		e.Data = &p
	case StripeObjectPrice:
		var p stripe.Price
		if err := bson.Unmarshal(aux.Data, &p); err != nil {
			return err
		}
		e.Data = &p
	case StripeObjectCustomer:
		var c stripe.Customer
		if err := bson.Unmarshal(aux.Data, &c); err != nil {
			return err
		}
		e.Data = &c
	case StripeObjectSubscription:
		var s stripe.Subscription
		if err := bson.Unmarshal(aux.Data, &s); err != nil {
			return err
		}
		e.Data = &s
	case StripeObjectPaymentMethod:
		var pm stripe.PaymentMethod
		if err := bson.Unmarshal(aux.Data, &pm); err != nil {
			return err
		}
		e.Data = &pm
	default:
		return fmt.Errorf("unknown stripe object type: %s", e.Type)
	}

	return nil
}

type StripeCacheRepository interface {
	Save(ctx context.Context, entry *StripeCacheEntry) error
	GetByID(ctx context.Context, id string) (*StripeCacheEntry, error)
	GetByType(ctx context.Context, objType StripeObjectType) ([]*StripeCacheEntry, error)
	Delete(ctx context.Context, id string) error
}
