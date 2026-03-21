package domain

import (
	"context"
	"errors"
	"time"
)

type SubscriptionStatus string

const (
	StatusActive     SubscriptionStatus = "active"
	StatusCanceled   SubscriptionStatus = "canceled"
	StatusPastDue    SubscriptionStatus = "past_due"
	StatusTrialing   SubscriptionStatus = "trialing"
	StatusIncomplete SubscriptionStatus = "incomplete"
)

var (
	ErrNotFound = errors.New("resource not found")
	ErrSubscriptionNotFound = errors.New("subscription not found")
)

type Subscription struct {
	ID                 string             `bson:"id"`
	UserID             string             `bson:"userId"`
	PlanID             string             `bson:"planId"`
	PlanName           string             `bson:"planName"`
	Status             SubscriptionStatus `bson:"status"`
	CurrentPeriodStart time.Time          `bson:"currentPeriodStart"`
	CurrentPeriodEnd   time.Time          `bson:"currentPeriodEnd"`
	StripeCustomerID   string             `bson:"stripeCustomerId"`
	StripeSubscriptionID string           `bson:"stripeSubscriptionId"` // Legacy/Last used
	StripePlanSubscriptionID string       `bson:"stripePlanSubscriptionId"`
	StripeStorageSubscriptionID string     `bson:"stripeStorageSubscriptionId"`
	StripeSubscriptionScheduleID string   `bson:"stripeSubscriptionScheduleId"`
	BaseAILimit        int                `bson:"baseAILimit"`
	BaseStorageQuota   int                `bson:"baseStorageQuota"`
	AddonStorageQuota  int                `bson:"addonStorageQuota"`
}

func NewSubscription(userID, customerID, subID, planID string, storageLimit, aiLimit int) *Subscription {
	return &Subscription{
		ID:                   "sub_" + userID,
		UserID:               userID,
		PlanID:               planID,
		Status:               StatusActive,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subID,
		StripePlanSubscriptionID: subID,
		BaseAILimit:          aiLimit,
		BaseStorageQuota:     storageLimit,
		CurrentPeriodEnd:     time.Now().AddDate(0, 1, 0), // Default to 1 month from now
	}
}

func (s *Subscription) GetTotalStorageLimit() int {
	if s.Status != StatusActive && s.Status != StatusTrialing {
		// Default to Free Tier (0.5GB) if subscription is not active
		return 512 * 1024 * 1024
	}
	
	// Free tier default if not set
	if s.BaseStorageQuota == 0 {
		return 512 * 1024 * 1024
	}

	return s.BaseStorageQuota + s.AddonStorageQuota
}

type SubscriptionRepository interface {
	Save(ctx context.Context, sub *Subscription) error
	GetByUserID(ctx context.Context, userID string) (*Subscription, error)
	GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*Subscription, error)
	GetByStripeCustomerID(ctx context.Context, stripeCustomerID string) (*Subscription, error)
	GetActiveSubscriptions(ctx context.Context) ([]*Subscription, error)
}
