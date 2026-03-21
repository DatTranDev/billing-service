package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/domain"
	"github.com/teachingassistant/billing-service/internal/infra/mongo"
)

type WebhookHandler struct {
	stripePort app.StripePort
	eventRepo  *mongo.StripeEventRepository
	subRepo    domain.SubscriptionRepository
	walletRepo domain.WalletRepository
	ledgerRepo domain.LedgerRepository
}

func NewWebhookHandler(
	stripePort app.StripePort,
	eventRepo *mongo.StripeEventRepository,
	subRepo domain.SubscriptionRepository,
	walletRepo domain.WalletRepository,
	ledgerRepo domain.LedgerRepository,
) *WebhookHandler {
	return &WebhookHandler{
		stripePort: stripePort,
		eventRepo:  eventRepo,
		subRepo:    subRepo,
		walletRepo: walletRepo,
		ledgerRepo: ledgerRepo,
	}
}

func (h *WebhookHandler) Handle(c *echo.Context) error {
	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		slog.Error("failed to read stripe webhook body", "error", err)
		return c.NoContent(http.StatusBadRequest)
	}

	sig := c.Request().Header.Get("Stripe-Signature")
	secret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	secretPrefix := secret
	if len(secret) > 8 {
		secretPrefix = secret[:8]
	}

	slog.Info("webhook received",
		"payloadLen", len(payload),
		"sigLen", len(sig),
		"hasSignature", sig != "",
		"secretConfigured", secret != "",
		"secretPrefix", secretPrefix,
		"secretLen", len(secret))

	if secret == "" {
		slog.Error("stripe webhook secret is missing")
		return c.NoContent(http.StatusInternalServerError)
	}
	if sig == "" {
		slog.Error("stripe signature header is missing")
		return c.NoContent(http.StatusBadRequest)
	}

	event, err := webhook.ConstructEventWithOptions(payload, sig, secret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		sampleLen := 16
		if len(payload) < 16 {
			sampleLen = len(payload)
		}
		slog.Error("failed to verify stripe signature",
			"error", err,
			"payloadSample", string(payload[:sampleLen]))
		return c.NoContent(http.StatusBadRequest)
	}

	// Idempotency check - do this synchronously to avoid processing the same event multiple times
	exists, err := h.eventRepo.Exists(context.Background(), event.ID)
	if err != nil {
		slog.Error("failed to check event idempotency", "eventID", event.ID, "error", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if exists {
		slog.Info("stripe event already processed", "eventID", event.ID)
		return c.NoContent(http.StatusNoContent)
	}

	// Process event asynchronously
	go func(evt stripe.Event) {
		// Use background context for async processing
		ctx := context.Background()

		err := h.processEvent(ctx, evt)
		if err != nil {
			slog.Error("failed to process stripe event", "eventID", evt.ID, "type", evt.Type, "error", err)
			return
		}

		// Save event ID to mark it as processed
		if err := h.eventRepo.Save(ctx, evt.ID); err != nil {
			slog.Error("failed to save processed stripe event", "eventID", evt.ID, "error", err)
		}

		slog.Info("successfully processed stripe event", "eventID", evt.ID, "type", evt.Type)
	}(event)

	// Return 200 quickly
	return c.NoContent(http.StatusOK)
}

func (h *WebhookHandler) processEvent(ctx context.Context, event stripe.Event) error {
	switch event.Type {
	case "checkout.session.completed":
		return h.handleCheckoutSessionCompleted(ctx, event)

	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		return h.handleSubscriptionEvent(ctx, event)

	case "subscription_schedule.created", "subscription_schedule.updated", "subscription_schedule.released", "subscription_schedule.canceled":
		return h.handleSubscriptionScheduleEvent(ctx, event)

	case "invoice.created", "invoice.finalized", "invoice.payment_failed", "invoice.voided", "invoice.payment_succeeded":
		return h.handleInvoiceEvent(ctx, event)

	case "payment_intent.succeeded", "payment_intent.payment_failed":
		return h.handlePaymentIntentEvent(ctx, event)

	case "charge.refunded":
		return h.handleChargeRefundEvent(ctx, event)

	case "payment_method.attached":
		return h.handlePaymentMethodAttached(ctx, event)

	case "setup_intent.succeeded":
		return h.handleSetupIntentSucceeded(ctx, event)

	default:
		slog.Info("unhandled stripe event type", "type", event.Type)
	}
	return nil
}

func (h *WebhookHandler) handleCheckoutSessionCompleted(ctx context.Context, event stripe.Event) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return fmt.Errorf("unmarshal checkout session: %w", err)
	}

	userID := sess.Metadata["userId"]
	if sess.Mode == stripe.CheckoutSessionModePayment {
		lineItems, err := h.stripePort.GetSessionLineItems(ctx, sess.ID)
		if err != nil {
			return fmt.Errorf("get session line items: %w", err)
		}
		for _, item := range lineItems {
			wallet, err := h.walletRepo.GetByUserID(ctx, userID)
			if err != nil {
				return fmt.Errorf("get wallet: %w", err)
			}

			creditsStr := item.Metadata["credits"]
			storageGbStr := item.Metadata["storage_gb"]
			itemType := item.Metadata["type"]

			if itemType == "addon_ai" && creditsStr != "" {
				var credits int
				fmt.Sscanf(creditsStr, "%d", &credits)
				totalCredits := item.Quantity * credits

				pack := domain.CreditPack{
					ID:              fmt.Sprintf("cp_%d", time.Now().UnixNano()),
					TotalAmount:     totalCredits,
					RemainingAmount: totalCredits,
					Type:            "ai",
					PurchasedAt:     time.Now(),
					ExpiresAt:       time.Now().AddDate(1, 0, 0), // 1 year expiry
				}
				wallet.CreditPacks = append(wallet.CreditPacks, pack)
			} else if itemType == "addon_storage" && storageGbStr != "" {
				// Handle storage add-on (usually updates subscription or adds a pack)
				// For now, we'll just track it as a pack
				var storageGb float64
				fmt.Sscanf(storageGbStr, "%f", &storageGb)

				pack := domain.CreditPack{
					ID:              fmt.Sprintf("sp_%d", time.Now().UnixNano()),
					TotalAmount:     int(storageGb * 1024 * 1024 * 1024), // Convert GB to bytes
					RemainingAmount: int(storageGb * 1024 * 1024 * 1024),
					Type:            "storage",
					PurchasedAt:     time.Now(),
					ExpiresAt:       time.Now().AddDate(0, 1, 0), // Monthly storage add-on
				}
				wallet.CreditPacks = append(wallet.CreditPacks, pack)
			} else if creditsStr != "" {
				// Fallback for legacy items
				var credits int
				fmt.Sscanf(creditsStr, "%d", &credits)
				wallet.AddLiquid(item.Quantity * credits)
			}

			if err := h.walletRepo.Update(ctx, wallet); err != nil {
				return fmt.Errorf("update wallet: %w", err)
			}
		}
	} else if sess.Mode == stripe.CheckoutSessionModeSubscription {
		h.syncSubscriptionData(ctx, sess.Subscription.ID, userID)
	}

	h.stripePort.GetCustomer(ctx, sess.Customer.ID)
	if sess.Subscription != nil {
		h.stripePort.GetSubscription(ctx, sess.Subscription.ID)
	}
	return nil
}

func (h *WebhookHandler) handleSubscriptionEvent(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("unmarshal subscription: %w", err)
	}

	userID := sub.Metadata["userId"]
	if userID == "" {
		// Fallback 1: Try to find by subscription ID in DB
		existingSub, err := h.subRepo.GetByStripeSubscriptionID(ctx, sub.ID)
		if err == nil && existingSub != nil {
			userID = existingSub.UserID
		} else {
			// Fallback 2: Try to find by customer ID in DB
			existingSub, err = h.subRepo.GetByStripeCustomerID(ctx, sub.Customer.ID)
			if err == nil && existingSub != nil {
				userID = existingSub.UserID
			}
		}
	}

	if userID != "" {
		h.syncSubscriptionData(ctx, sub.ID, userID)
	}
	h.stripePort.GetSubscription(ctx, sub.ID)
	return nil
}

func (h *WebhookHandler) handleSubscriptionScheduleEvent(ctx context.Context, event stripe.Event) error {
	var sched stripe.SubscriptionSchedule
	if err := json.Unmarshal(event.Data.Raw, &sched); err != nil {
		return fmt.Errorf("unmarshal subscription schedule: %w", err)
	}

	// Find the associated subscription and update the schedule ID
	if sched.Subscription != nil {
		sub, err := h.subRepo.GetByStripeSubscriptionID(ctx, sched.Subscription.ID)
		if err == nil && sub != nil {
			sub.StripeSubscriptionScheduleID = sched.ID
			if event.Type == "subscription_schedule.released" || event.Type == "subscription_schedule.canceled" {
				sub.StripeSubscriptionScheduleID = ""
			}
			h.subRepo.Save(ctx, sub)
		}
	}
	return nil
}

func (h *WebhookHandler) handleInvoiceEvent(ctx context.Context, event stripe.Event) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return fmt.Errorf("unmarshal invoice: %w", err)
	}

	if inv.Subscription == nil {
		return nil
	}

	var userID string
	existingSub, err := h.subRepo.GetByStripeSubscriptionID(ctx, inv.Subscription.ID)
	if err == nil && existingSub != nil {
		userID = existingSub.UserID
	} else if inv.Subscription.Metadata != nil {
		userID = inv.Subscription.Metadata["userId"]
	}

	if userID == "" {
		return nil
	}

	h.syncSubscriptionData(ctx, inv.Subscription.ID, userID)

	if event.Type == "invoice.payment_succeeded" {
		// Reset monthly credits on successful payment of recurring invoice
		if inv.BillingReason == stripe.InvoiceBillingReasonSubscriptionCycle {
			sub, _ := h.subRepo.GetByUserID(ctx, userID)
			wallet, _ := h.walletRepo.GetByUserID(ctx, userID)
			if sub != nil && wallet != nil {
				wallet.SetPlanBalance(sub.BaseAILimit)
				h.walletRepo.Update(ctx, wallet)
			}
		}

		// Record in ledger
		h.ledgerRepo.Save(ctx, &domain.LedgerEntry{
			ID:        "led_" + inv.ID,
			UserID:    userID,
			Amount:    inv.Total,
			Currency:  string(inv.Currency),
			Type:      domain.LedgerTypePayment,
			StripeID:  inv.ID,
			CreatedAt: time.Now(),
			Metadata:  map[string]string{"type": "invoice", "reason": string(inv.BillingReason)},
		})

		slog.Info("ledger: payment succeeded for invoice", "invoiceID", inv.ID, "userID", userID)
	} else if event.Type == "invoice.payment_failed" || event.Type == "invoice.voided" {
		// Update sub status to past_due or canceled
		sub, _ := h.subRepo.GetByUserID(ctx, userID)
		if sub != nil {
			if event.Type == "invoice.payment_failed" {
				sub.Status = domain.StatusPastDue
			} else {
				sub.Status = domain.StatusCanceled
			}
			h.subRepo.Save(ctx, sub)
		}
		slog.Warn("invoice failed or voided", "invoiceID", inv.ID, "type", event.Type, "userID", userID)
	}

	return nil
}

func (h *WebhookHandler) handlePaymentIntentEvent(ctx context.Context, event stripe.Event) error {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return fmt.Errorf("unmarshal payment intent: %w", err)
	}

	if event.Type == "payment_intent.succeeded" {
		userID := pi.Metadata["userId"]
		if userID != "" {
			h.ledgerRepo.Save(ctx, &domain.LedgerEntry{
				ID:        "led_" + pi.ID,
				UserID:    userID,
				Amount:    pi.Amount,
				Currency:  string(pi.Currency),
				Type:      domain.LedgerTypePayment,
				StripeID:  pi.ID,
				CreatedAt: time.Now(),
				Metadata:  map[string]string{"type": "payment_intent"},
			})
		}
	}

	slog.Info("ledger: payment intent event", "type", event.Type, "id", pi.ID, "status", pi.Status)
	return nil
}

func (h *WebhookHandler) handleChargeRefundEvent(ctx context.Context, event stripe.Event) error {
	var charge stripe.Charge
	if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
		return fmt.Errorf("unmarshal charge: %w", err)
	}

	userID := charge.Metadata["userId"]
	if userID != "" {
		h.ledgerRepo.Save(ctx, &domain.LedgerEntry{
			ID:        "led_ref_" + charge.ID,
			UserID:    userID,
			Amount:    charge.AmountRefunded,
			Currency:  string(charge.Currency),
			Type:      domain.LedgerTypeRefund,
			StripeID:  charge.ID,
			CreatedAt: time.Now(),
			Metadata:  map[string]string{"chargeId": charge.ID},
		})
	}

	slog.Info("ledger: charge refunded", "chargeID", charge.ID, "amount", charge.AmountRefunded)
	return nil
}

func (h *WebhookHandler) syncSubscriptionData(ctx context.Context, stripeSubID string, userID string) {
	slog.Info("syncing subscription data", "stripeSubID", stripeSubID, "userID", userID)

	// Fetch full subscription with items from Stripe
	sub, err := h.stripePort.GetSubscription(ctx, stripeSubID)
	if err != nil {
		slog.Error("failed to get stripe subscription", "error", err, "stripeSubID", stripeSubID)
		return
	}

	// Identify subscription type from metadata (set in adapter.go)
	subType := sub.Metadata["sub_type"]
	slog.Debug("identified sub_type from metadata", "subType", subType, "stripeSubID", stripeSubID)

	// Track whether incoming Stripe subscription contains a plan product.
	hasPlanProduct := false
	for _, item := range sub.Items.Data {
		if item.Price != nil && item.Price.Product != nil {
			p := item.Price.Product
			if p.Metadata["type"] == "plan" || p.Metadata["plan_type"] != "" {
				hasPlanProduct = true
				break
			}
		}
	}

	// Fallback to product metadata if sub_type is missing (e.g. from direct Stripe CLI or older sessions)
	if subType == "" && len(sub.Items.Data) > 0 {
		item := sub.Items.Data[0]
		if item.Price != nil && item.Price.Product != nil {
			p := item.Price.Product
			if p.Metadata["type"] == "plan" || p.Metadata["plan_type"] != "" {
				subType = "plan"
			} else if p.Metadata["addon_type"] == "storage" {
				subType = "storage"
			}
			slog.Debug("inferred sub_type from product metadata", "subType", subType, "stripeSubID", stripeSubID)
		}
	}

	// Force type to plan if product metadata clearly marks this subscription as plan.
	if hasPlanProduct {
		subType = "plan"
	}

	existingSub, _ := h.subRepo.GetByUserID(ctx, userID)
	if existingSub == nil {
		existingSub = &domain.Subscription{
			ID:     "sub_" + userID,
			UserID: userID,
			PlanID: "Free",
			Status: domain.StatusActive,
		}
	}

	prevPlanSubID := existingSub.StripePlanSubscriptionID
	prevStorageSubID := existingSub.StripeStorageSubscriptionID

	// Calculate status
	status := domain.StatusCanceled
	if sub.Status == stripe.SubscriptionStatusActive || sub.Status == stripe.SubscriptionStatusTrialing {
		status = domain.StatusActive
	} else if sub.Status == stripe.SubscriptionStatusIncomplete || sub.Status == stripe.SubscriptionStatusPastDue {
		status = domain.StatusIncomplete
	}

	// Handle replacement and limit updates based on subType
	if subType == "plan" {
		isActiveLikePlan := status == domain.StatusActive || status == domain.StatusIncomplete

		// Only replace if it's a DIFFERENT subscription
		if isActiveLikePlan && prevPlanSubID != "" && prevPlanSubID != sub.ID {
			slog.Info("replacing old plan subscription", "userID", userID, "oldSubID", prevPlanSubID, "newSubID", sub.ID)
			h.stripePort.CancelSubscription(ctx, prevPlanSubID)
		}

		if isActiveLikePlan {
			existingSub.StripePlanSubscriptionID = sub.ID
		}

		// Update limits from plan product metadata if active
		if status == domain.StatusActive {
			for _, item := range sub.Items.Data {
				if item.Price != nil && item.Price.Product != nil {
					p := item.Price.Product
					if p.Metadata["type"] == "plan" || p.Metadata["plan_type"] != "" {
						planID := p.Metadata["plan_type"]
						if planID == "" {
							planID = p.Metadata["plan_id"]
						}
						if planID == "" {
							planID = p.ID
						}
						existingSub.PlanID = planID
						// Use product name as human-readable plan name
						if p.Name != "" {
							existingSub.PlanName = p.Name
						}
						var aiLimit int
						fmt.Sscanf(p.Metadata["ai_limit"], "%d", &aiLimit)
						existingSub.BaseAILimit = aiLimit

						var storageGB int
						fmt.Sscanf(p.Metadata["storage_quota_gb"], "%d", &storageGB)
						existingSub.BaseStorageQuota = storageGB * 1024 * 1024 * 1024
					}
				}
			}
		} else {
			// Only revert to free if this canceled sub is the currently tracked plan.
			if prevPlanSubID == sub.ID {
				existingSub.StripePlanSubscriptionID = ""
				existingSub.PlanID = "Free"
				existingSub.PlanName = "Free Plan"
				existingSub.BaseAILimit = 100
				existingSub.BaseStorageQuota = 512 * 1024 * 1024
			}
		}

	} else if subType == "storage" {
		isActiveLikeStorage := status == domain.StatusActive || status == domain.StatusIncomplete

		// Only replace if it's a DIFFERENT subscription
		if isActiveLikeStorage && prevStorageSubID != "" && prevStorageSubID != sub.ID {
			slog.Info("replacing old storage subscription", "userID", userID, "oldSubID", prevStorageSubID, "newSubID", sub.ID)
			h.stripePort.CancelSubscription(ctx, prevStorageSubID)
		}

		if isActiveLikeStorage {
			existingSub.StripeStorageSubscriptionID = sub.ID
		}

		// Update addon quota from storage product metadata if active
		if status == domain.StatusActive {
			newAddonQuota := 0
			for _, item := range sub.Items.Data {
				if item.Price != nil && item.Price.Product != nil {
					p := item.Price.Product
					if p.Metadata["addon_type"] == "storage" {
						var addonGB int
						fmt.Sscanf(p.Metadata["storage_gb"], "%d", &addonGB)
						newAddonQuota += int(item.Quantity) * addonGB * 1024 * 1024 * 1024
					}
				}
			}
			existingSub.AddonStorageQuota = newAddonQuota
		} else {
			existingSub.AddonStorageQuota = 0
		}
	}

	// Common updates: status and period should be derived from PLAN subscription only.
	if subType == "plan" {
		isTrackedPlan := existingSub.StripePlanSubscriptionID == sub.ID || prevPlanSubID == sub.ID
		if isTrackedPlan {
			existingSub.Status = status
			existingSub.StripeSubscriptionID = sub.ID
			existingSub.CurrentPeriodStart = time.Unix(sub.CurrentPeriodStart, 0)
			existingSub.CurrentPeriodEnd = time.Unix(sub.CurrentPeriodEnd, 0)
		}
	}

	existingSub.StripeCustomerID = sub.Customer.ID

	h.subRepo.Save(ctx, existingSub)

	// Replenish AI credits if a plan was activated
	if subType == "plan" && status == domain.StatusActive {
		wallet, _ := h.walletRepo.GetByUserID(ctx, userID)
		if wallet != nil && wallet.PlanBalance < existingSub.BaseAILimit {
			wallet.SetPlanBalance(existingSub.BaseAILimit)
			h.walletRepo.Update(ctx, wallet)
		}
	}
}

func (h *WebhookHandler) handlePaymentMethodAttached(ctx context.Context, event stripe.Event) error {
	var pm stripe.PaymentMethod
	if err := json.Unmarshal(event.Data.Raw, &pm); err != nil {
		return fmt.Errorf("unmarshal payment method: %w", err)
	}

	if pm.Customer == nil {
		return nil
	}

	return h.preventDuplicateCard(ctx, pm.Customer.ID, pm.ID)
}

func (h *WebhookHandler) handleSetupIntentSucceeded(ctx context.Context, event stripe.Event) error {
	var si stripe.SetupIntent
	if err := json.Unmarshal(event.Data.Raw, &si); err != nil {
		return fmt.Errorf("unmarshal setup intent: %w", err)
	}

	if si.Customer == nil || si.PaymentMethod == nil {
		return nil
	}

	return h.preventDuplicateCard(ctx, si.Customer.ID, si.PaymentMethod.ID)
}

func (h *WebhookHandler) preventDuplicateCard(ctx context.Context, customerID, newPMID string) error {
	// 1. Get the new PM to get fingerprint
	newPM, err := h.stripePort.GetPaymentMethod(ctx, newPMID)
	if err != nil {
		return fmt.Errorf("get new payment method: %w", err)
	}

	if newPM.Card == nil {
		return nil // Not a card
	}

	// 2. List existing PMs
	pms, err := h.stripePort.ListPaymentMethods(ctx, customerID)
	if err != nil {
		return fmt.Errorf("list payment methods: %w", err)
	}

	// 3. Check for duplicates
	for _, pm := range pms {
		if pm.ID != newPM.ID && pm.Card != nil && pm.Card.Fingerprint == newPM.Card.Fingerprint {
			slog.Warn("duplicate card detected, detaching new payment method",
				"customerID", customerID,
				"newPMID", newPM.ID,
				"existingPMID", pm.ID,
				"fingerprint", newPM.Card.Fingerprint)

			_, err := h.stripePort.DetachPaymentMethod(ctx, newPM.ID)
			if err != nil {
				return fmt.Errorf("detach duplicate payment method: %w", err)
			}
			return nil
		}
	}

	return nil
}
