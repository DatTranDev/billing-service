package app

import (
	"context"
	"time"
	"github.com/stripe/stripe-go/v76"
)

type CheckoutParams struct {
	UserID               string  `json:"userId"`
	Email                string  `json:"email"`
	PriceRef             string  `json:"priceRef"` // Can be a Lookup Key or Price ID
	SuccessURL           string  `json:"successUrl"`
	CancelURL            string  `json:"cancelUrl"`
	StripeCustomerID     string  `json:"stripeCustomerId"`
	StripeSubscriptionID string  `json:"stripeSubscriptionId"`
	FirstName            string  `json:"firstName"`
	LastName             string  `json:"lastName"`
	Address              Address `json:"address"`
	Timezone             string  `json:"timezone"`
	IdempotencyKey       string  `json:"idempotencyKey"`
}

type Address struct {
	Line1      string
	City       string
	State      string
	PostalCode string
	Country    string
}

type AddonParams struct {
	UserID     string `json:"userId"`
	Email      string `json:"email"`
	PriceRef   string `json:"priceRef"` // Can be a Lookup Key or Price ID
	Quantity   int    `json:"quantity"`
	SuccessURL string `json:"successUrl"`
	CancelURL  string `json:"cancelUrl"`
	StripeCustomerID string `json:"stripeCustomerId"`
	IdempotencyKey   string `json:"idempotencyKey"`
}

type LineItem struct {
	PriceID  string
	Quantity int
	Metadata map[string]string // Assuming we can fetch price/product metadata
}

type PriceDTO struct {
	ID         string            `json:"id"`
	LookupKey  string            `json:"lookupKey"`
	UnitAmount int64             `json:"unitAmount"`
	Currency   string            `json:"currency"`
	Interval   string            `json:"interval"` // month, year
	Nickname   string            `json:"nickname"` // e.g., "1TB Storage"
	Metadata   map[string]string `json:"metadata"`
}

type PlanDTO struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Features    []string          `json:"features"`
	Prices      []PriceDTO        `json:"prices"`
	Metadata    map[string]string `json:"metadata"`
}

type CreditPackDTO struct {
	ID              string    `json:"id"`
	TotalAmount     int       `json:"totalAmount"`
	RemainingAmount int       `json:"remainingAmount"`
	Type            string    `json:"type"` // ai, storage
	PurchasedAt     time.Time `json:"purchasedAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

type StripePort interface {
	// Checkout & Sessions
	CreateCheckoutSession(ctx context.Context, params CheckoutParams) (string, error)
	CreateAddonSession(ctx context.Context, params AddonParams) (string, error)
	GetSessionLineItems(ctx context.Context, sessionID string) ([]LineItem, error)
	CreatePortalSession(ctx context.Context, stripeCustomerID, returnURL string) (string, error)

	// Customer CRUD
	CreateCustomer(ctx context.Context, email, name string, metadata map[string]string, idempotencyKey string) (*stripe.Customer, error)
	UpdateCustomer(ctx context.Context, customerID string, params *stripe.CustomerParams, idempotencyKey string) (*stripe.Customer, error)
	GetCustomer(ctx context.Context, customerID string) (*stripe.Customer, error)
	DeleteCustomer(ctx context.Context, customerID string) (*stripe.Customer, error)
	FindCustomerByEmail(ctx context.Context, email string) (*stripe.Customer, error)

	// Subscription CRUD
	GetSubscription(ctx context.Context, id string) (*stripe.Subscription, error)
	CreateSubscription(ctx context.Context, customerID, priceRef string, metadata map[string]string, idempotencyKey string) (*stripe.Subscription, error)
	UpdateSubscription(ctx context.Context, subID string, params *stripe.SubscriptionParams, idempotencyKey string) (*stripe.Subscription, error)
	CancelSubscription(ctx context.Context, subID string) (*stripe.Subscription, error)

	// PaymentMethod CRUD
	GetPaymentMethod(ctx context.Context, id string) (*stripe.PaymentMethod, error)
	ListPaymentMethods(ctx context.Context, customerID string) ([]*stripe.PaymentMethod, error)
	AttachPaymentMethod(ctx context.Context, pmID, customerID string) (*stripe.PaymentMethod, error)
	DetachPaymentMethod(ctx context.Context, pmID string) (*stripe.PaymentMethod, error)
	SetDefaultPaymentMethod(ctx context.Context, customerID, pmID string) error

	// SetupIntent
	CreateSetupIntent(ctx context.Context, customerID string) (*stripe.SetupIntent, error)

	// Direct Payment
	DirectCharge(ctx context.Context, customerID, priceRef string, quantity int, pmID string, metadata map[string]string, idempotencyKey string) (*stripe.PaymentIntent, error)
	DirectSubscription(ctx context.Context, customerID, priceRef string, pmID string, metadata map[string]string, idempotencyKey string) (*stripe.Subscription, error)

	// Invoices
	ListInvoices(ctx context.Context, customerID string, limit int) ([]*stripe.Invoice, error)

	// Plans & Products
	ListActivePlans(ctx context.Context) ([]PlanDTO, error)
	ListActiveAddons(ctx context.Context) ([]PlanDTO, error)
}
