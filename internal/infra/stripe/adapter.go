package stripe

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
	"github.com/stripe/stripe-go/v76"
	portalsession "github.com/stripe/stripe-go/v76/billingportal/session"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/paymentmethod"
	"github.com/stripe/stripe-go/v76/subscription"
	"fmt"
	"github.com/stripe/stripe-go/v76/price"
	"github.com/stripe/stripe-go/v76/setupintent"
	"github.com/stripe/stripe-go/v76/invoice"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/product"
)

type StripeAdapter struct {
	apiKey    string
	cacheRepo domain.StripeCacheRepository
}

func NewStripeAdapter(apiKey string, cacheRepo domain.StripeCacheRepository) *StripeAdapter {
	stripe.Key = apiKey
	return &StripeAdapter{
		apiKey:    apiKey,
		cacheRepo: cacheRepo,
	}
}

func (a *StripeAdapter) CreateCheckoutSession(ctx context.Context, params app.CheckoutParams) (string, error) {
	priceID, err := a.resolvePriceID(ctx, params.PriceRef)
	if err != nil {
		return "", err
	}

	checkoutParams := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(params.SuccessURL),
		CancelURL:  stripe.String(params.CancelURL),
		Metadata: map[string]string{
			"userId":   params.UserID,
			"sub_type": "plan",
		},
	}

	// Try to use provided StripeCustomerID first, fallback to search by email
	if params.StripeCustomerID != "" {
		checkoutParams.Customer = stripe.String(params.StripeCustomerID)
	} else {
		cus, _ := a.FindCustomerByEmail(ctx, params.Email)
		if cus != nil {
			checkoutParams.Customer = stripe.String(cus.ID)
		} else {
			checkoutParams.CustomerEmail = stripe.String(params.Email)
		}
	}

	if params.IdempotencyKey != "" {
		checkoutParams.IdempotencyKey = stripe.String(params.IdempotencyKey)
	}

	s, err := session.New(checkoutParams)
	if err != nil {
		return "", err
	}

	return s.URL, nil
}

func (a *StripeAdapter) CreateAddonSession(ctx context.Context, params app.AddonParams) (string, error) {
	priceID, err := a.resolvePriceID(ctx, params.PriceRef)
	if err != nil {
		return "", err
	}

	// Fetch price to check if it's recurring or one-time
	priceParams := &stripe.PriceParams{}
	priceParams.Context = ctx
	p, err := price.Get(priceID, priceParams)
	if err != nil {
		return "", err
	}

	mode := stripe.CheckoutSessionModePayment
	if p.Recurring != nil {
		mode = stripe.CheckoutSessionModeSubscription
	}

	quantity := int64(params.Quantity)
	if quantity < 1 {
		quantity = 1
	}

	checkoutParams := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(quantity),
			},
		},
		Mode:       stripe.String(string(mode)),
		SuccessURL: stripe.String(params.SuccessURL),
		CancelURL:  stripe.String(params.CancelURL),
		Metadata: map[string]string{
			"userId":   params.UserID,
			"sub_type": "storage",
		},
	}

	// Try to use provided StripeCustomerID first, fallback to search by email
	if params.StripeCustomerID != "" {
		checkoutParams.Customer = stripe.String(params.StripeCustomerID)
	} else {
		cus, _ := a.FindCustomerByEmail(ctx, params.Email)
		if cus != nil {
			checkoutParams.Customer = stripe.String(cus.ID)
		} else {
			checkoutParams.CustomerEmail = stripe.String(params.Email)
		}
	}

	if params.IdempotencyKey != "" {
		checkoutParams.IdempotencyKey = stripe.String(params.IdempotencyKey)
	}

	s, err := session.New(checkoutParams)
	if err != nil {
		return "", err
	}

	return s.URL, nil
}

func (a *StripeAdapter) GetSessionLineItems(ctx context.Context, sessionID string) ([]app.LineItem, error) {
	params := &stripe.CheckoutSessionListLineItemsParams{}
	params.Session = stripe.String(sessionID)
	params.Context = ctx
	// We expand price.product so we can read product metadata
	params.AddExpand("data.price.product")

	i := session.ListLineItems(params)
	var items []app.LineItem
	for i.Next() {
		li := i.LineItem()
		
		metadata := make(map[string]string)
		if li.Price != nil && li.Price.Product != nil {
			for k, v := range li.Price.Product.Metadata {
				metadata[k] = v
			}
		}

		items = append(items, app.LineItem{
			PriceID:  li.Price.ID,
			Quantity: int(li.Quantity),
			Metadata: metadata,
		})
	}
	if err := i.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (a *StripeAdapter) GetSubscription(ctx context.Context, id string) (*stripe.Subscription, error) {
	// Try cache first
	if a.cacheRepo != nil {
		cached, _ := a.cacheRepo.GetByID(ctx, id)
		if cached != nil && cached.Type == domain.StripeObjectSubscription {
			if sub, ok := cached.Data.(*stripe.Subscription); ok {
				return sub, nil
			}
		}
	}

	params := &stripe.SubscriptionParams{}
	params.Context = ctx
	sub, err := subscription.Get(id, params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Save(ctx, &domain.StripeCacheEntry{
			ID:   sub.ID,
			Type: domain.StripeObjectSubscription,
			Data: sub,
		})
	}
	return sub, err
}

func (a *StripeAdapter) CreateSubscription(ctx context.Context, customerID, priceRef string, metadata map[string]string, idempotencyKey string) (*stripe.Subscription, error) {
	priceID, err := a.resolvePriceID(ctx, priceRef)
	if err != nil {
		return nil, err
	}

	params := &stripe.SubscriptionParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(priceID),
			},
		},
		Metadata: metadata,
	}
	params.Context = ctx
	if idempotencyKey != "" {
		params.IdempotencyKey = stripe.String(idempotencyKey)
	}
	sub, err := subscription.New(params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Save(ctx, &domain.StripeCacheEntry{
			ID:   sub.ID,
			Type: domain.StripeObjectSubscription,
			Data: sub,
		})
	}
	return sub, err
}

func (a *StripeAdapter) UpdateSubscription(ctx context.Context, subID string, params *stripe.SubscriptionParams, idempotencyKey string) (*stripe.Subscription, error) {
	params.Context = ctx
	if idempotencyKey != "" {
		params.IdempotencyKey = stripe.String(idempotencyKey)
	}
	sub, err := subscription.Update(subID, params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Save(ctx, &domain.StripeCacheEntry{
			ID:   sub.ID,
			Type: domain.StripeObjectSubscription,
			Data: sub,
		})
	}
	return sub, err
}

func (a *StripeAdapter) CancelSubscription(ctx context.Context, subID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionCancelParams{}
	params.Context = ctx
	sub, err := subscription.Cancel(subID, params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Delete(ctx, subID)
	}
	return sub, err
}

// Customer CRUD
func (a *StripeAdapter) CreateCustomer(ctx context.Context, email, name string, metadata map[string]string, idempotencyKey string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		Email:    stripe.String(email),
		Name:     stripe.String(name),
		Metadata: metadata,
	}
	params.Context = ctx
	if idempotencyKey != "" {
		params.IdempotencyKey = stripe.String(idempotencyKey)
	}
	cus, err := customer.New(params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Save(ctx, &domain.StripeCacheEntry{
			ID:   cus.ID,
			Type: domain.StripeObjectCustomer,
			Data: cus,
		})
	}
	return cus, err
}

func (a *StripeAdapter) UpdateCustomer(ctx context.Context, customerID string, params *stripe.CustomerParams, idempotencyKey string) (*stripe.Customer, error) {
	params.Context = ctx
	if idempotencyKey != "" {
		params.IdempotencyKey = stripe.String(idempotencyKey)
	}
	cus, err := customer.Update(customerID, params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Save(ctx, &domain.StripeCacheEntry{
			ID:   cus.ID,
			Type: domain.StripeObjectCustomer,
			Data: cus,
		})
	}
	return cus, err
}

func (a *StripeAdapter) GetCustomer(ctx context.Context, customerID string) (*stripe.Customer, error) {
	if a.cacheRepo != nil {
		cached, _ := a.cacheRepo.GetByID(ctx, customerID)
		if cached != nil && cached.Type == domain.StripeObjectCustomer {
			if cus, ok := cached.Data.(*stripe.Customer); ok {
				return cus, nil
			}
		}
	}

	params := &stripe.CustomerParams{}
	params.Context = ctx
	cus, err := customer.Get(customerID, params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Save(ctx, &domain.StripeCacheEntry{
			ID:   cus.ID,
			Type: domain.StripeObjectCustomer,
			Data: cus,
		})
	}
	return cus, err
}

func (a *StripeAdapter) DeleteCustomer(ctx context.Context, customerID string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{}
	params.Context = ctx
	cus, err := customer.Del(customerID, params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Delete(ctx, customerID)
	}
	return cus, err
}

func (a *StripeAdapter) FindCustomerByEmail(ctx context.Context, email string) (*stripe.Customer, error) {
	params := &stripe.CustomerListParams{
		Email: stripe.String(email),
	}
	params.Context = ctx
	params.Limit = stripe.Int64(1)

	i := customer.List(params)
	if i.Next() {
		return i.Customer(), nil
	}

	if err := i.Err(); err != nil {
		return nil, err
	}

	return nil, nil // Not found is not an error
}

// PaymentMethod CRUD
func (a *StripeAdapter) ListPaymentMethods(ctx context.Context, customerID string) ([]*stripe.PaymentMethod, error) {
	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerID),
		Type:     stripe.String("card"),
	}
	params.Context = ctx
	i := paymentmethod.List(params)
	var pms []*stripe.PaymentMethod
	for i.Next() {
		pms = append(pms, i.PaymentMethod())
	}
	return pms, i.Err()
}

func (a *StripeAdapter) GetPaymentMethod(ctx context.Context, id string) (*stripe.PaymentMethod, error) {
	if a.cacheRepo != nil {
		if cached, err := a.cacheRepo.GetByID(ctx, id); err == nil && cached != nil {
			if pm, ok := cached.Data.(*stripe.PaymentMethod); ok {
				return pm, nil
			}
		}
	}

	params := &stripe.PaymentMethodParams{}
	params.Context = ctx
	pm, err := paymentmethod.Get(id, params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Save(ctx, &domain.StripeCacheEntry{
			ID:   pm.ID,
			Type: domain.StripeObjectPaymentMethod,
			Data: pm,
		})
	}
	return pm, err
}

func (a *StripeAdapter) AttachPaymentMethod(ctx context.Context, pmID, customerID string) (*stripe.PaymentMethod, error) {
	// 1. Fetch the PM to get its fingerprint
	newPM, err := a.GetPaymentMethod(ctx, pmID)
	if err != nil {
		return nil, err
	}

	if newPM.Card == nil {
		return nil, fmt.Errorf("only card payment methods are supported for duplicate check")
	}

	// 2. List existing PMs for this customer
	existingPMs, err := a.ListPaymentMethods(ctx, customerID)
	if err != nil {
		return nil, err
	}

	// 3. Check for matching fingerprint
	for _, pm := range existingPMs {
		if pm.Card != nil && pm.Card.Fingerprint == newPM.Card.Fingerprint {
			return nil, fmt.Errorf("this card is already attached to your account")
		}
	}

	params := &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(customerID),
	}
	params.Context = ctx
	pm, err := paymentmethod.Attach(pmID, params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Save(ctx, &domain.StripeCacheEntry{
			ID:   pm.ID,
			Type: domain.StripeObjectPaymentMethod,
			Data: pm,
		})
	}
	return pm, err
}

func (a *StripeAdapter) DirectCharge(ctx context.Context, customerID, priceRef string, quantity int, pmID string, metadata map[string]string, idempotencyKey string) (*stripe.PaymentIntent, error) {
	priceID, err := a.resolvePriceID(ctx, priceRef)
	if err != nil {
		return nil, err
	}

	pParams := &stripe.PriceParams{}
	pParams.Context = ctx
	p, err := price.Get(priceID, pParams)
	if err != nil {
		return nil, err
	}

	if quantity < 1 {
		quantity = 1
	}

	amount := p.UnitAmount * int64(quantity)

	// Combine metadata
	finalMetadata := map[string]string{
		"priceId":  priceID,
		"quantity": fmt.Sprintf("%d", quantity),
	}
	for k, v := range metadata {
		finalMetadata[k] = v
	}

	params := &stripe.PaymentIntentParams{
		Customer:      stripe.String(customerID),
		Amount:        stripe.Int64(amount),
		Currency:      stripe.String(string(p.Currency)),
		PaymentMethod: stripe.String(pmID),
		Confirm:       stripe.Bool(true),
		OffSession:    stripe.Bool(true), // Since user is using a saved card
		Metadata:      finalMetadata,
	}
	params.Context = ctx
	if idempotencyKey != "" {
		params.IdempotencyKey = stripe.String(idempotencyKey)
	}

	return paymentintent.New(params)
}

func (a *StripeAdapter) DirectSubscription(ctx context.Context, customerID, priceRef string, pmID string, metadata map[string]string, idempotencyKey string) (*stripe.Subscription, error) {
	priceID, err := a.resolvePriceID(ctx, priceRef)
	if err != nil {
		return nil, err
	}

	params := &stripe.SubscriptionParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(priceID),
			},
		},
		DefaultPaymentMethod: stripe.String(pmID),
		PaymentBehavior:      stripe.String("allow_incomplete"), // Standard for initial sub
		Metadata:             metadata,
	}
	params.Context = ctx
	if idempotencyKey != "" {
		params.IdempotencyKey = stripe.String(idempotencyKey)
	}

	sub, err := subscription.New(params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Save(ctx, &domain.StripeCacheEntry{
			ID:   sub.ID,
			Type: domain.StripeObjectSubscription,
			Data: sub,
		})
	}
	return sub, err
}

func (a *StripeAdapter) DetachPaymentMethod(ctx context.Context, pmID string) (*stripe.PaymentMethod, error) {
	params := &stripe.PaymentMethodDetachParams{}
	params.Context = ctx
	pm, err := paymentmethod.Detach(pmID, params)
	if err == nil && a.cacheRepo != nil {
		a.cacheRepo.Delete(ctx, pmID)
	}
	return pm, err
}

func (a *StripeAdapter) SetDefaultPaymentMethod(ctx context.Context, customerID, pmID string) error {
	params := &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(pmID),
		},
	}
	params.Context = ctx
	_, err := customer.Update(customerID, params)
	return err
}

func (a *StripeAdapter) CreateSetupIntent(ctx context.Context, customerID string) (*stripe.SetupIntent, error) {
	params := &stripe.SetupIntentParams{
		Customer: stripe.String(customerID),
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
	}
	params.Context = ctx
	return setupintent.New(params)
}

func (a *StripeAdapter) ListInvoices(ctx context.Context, customerID string, limit int) ([]*stripe.Invoice, error) {
	params := &stripe.InvoiceListParams{
		Customer: stripe.String(customerID),
	}
	params.Context = ctx
	if limit > 0 {
		params.Limit = stripe.Int64(int64(limit))
	}

	i := invoice.List(params)
	var invoices []*stripe.Invoice
	for i.Next() {
		invoices = append(invoices, i.Invoice())
	}
	return invoices, i.Err()
}

func (a *StripeAdapter) CreatePortalSession(ctx context.Context, stripeCustomerID, returnURL string) (string, error) {
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(stripeCustomerID),
		ReturnURL: stripe.String(returnURL),
	}
	s, err := portalsession.New(params)
	if err != nil {
		return "", err
	}
	return s.URL, nil
}

func (a *StripeAdapter) resolvePriceID(ctx context.Context, priceRef string) (string, error) {
	// If it already looks like a Price ID, return it
	if len(priceRef) > 6 && priceRef[:6] == "price_" {
		return priceRef, nil
	}

	// Otherwise, treat it as a lookup key
	params := &stripe.PriceListParams{}
	params.LookupKeys = stripe.StringSlice([]string{priceRef})
	params.Context = ctx
	params.Active = stripe.Bool(true)

	i := price.List(params)
	if i.Next() {
		return i.Price().ID, nil
	}

	if err := i.Err(); err != nil{
		return "", err
	}

	return "", fmt.Errorf("no active price found for lookup key: %s", priceRef)
}
func (a *StripeAdapter) ListActivePlans(ctx context.Context) ([]app.PlanDTO, error) {
	return a.listActiveProducts(ctx, "plan")
}

func (a *StripeAdapter) ListActiveAddons(ctx context.Context) ([]app.PlanDTO, error) {
	return a.listActiveProducts(ctx, "addon_ai", "addon_storage")
}

func (a *StripeAdapter) listActiveProducts(ctx context.Context, types ...string) ([]app.PlanDTO, error) {
	// 1. Fetch active products
	prodParams := &stripe.ProductListParams{}
	prodParams.Active = stripe.Bool(true)
	prodParams.Context = ctx
	
	i := product.List(prodParams)
	var plans []app.PlanDTO
	
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	for i.Next() {
		p := i.Product()
		// Filter by metadata type
		t := p.Metadata["type"]
		if !typeMap[t] {
			continue
		}
		
		plan := app.PlanDTO{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Metadata:    p.Metadata,
		}
		
		// Extract features from metadata if present
		if features, ok := p.Metadata["features"]; ok {
			plan.Features = []string{features} 
		}
		
		// 2. Fetch prices for this product
		priceParams := &stripe.PriceListParams{
			Product: stripe.String(p.ID),
			Active:  stripe.Bool(true),
		}
		priceParams.Context = ctx
		
		pi := price.List(priceParams)
		for pi.Next() {
			pr := pi.Price()
			interval := ""
			if pr.Recurring != nil {
				interval = string(pr.Recurring.Interval)
			}
			
			plan.Prices = append(plan.Prices, app.PriceDTO{
				ID:         pr.ID,
				LookupKey:  pr.LookupKey,
				UnitAmount: pr.UnitAmount,
				Currency:   string(pr.Currency),
				Interval:   interval,
				Nickname:   pr.Nickname,
				Metadata:   pr.Metadata,
			})
		}
		
		plans = append(plans, plan)
	}
	
	if err := i.Err(); err != nil {
		return nil, err
	}
	return plans, nil
}
