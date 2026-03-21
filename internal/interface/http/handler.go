package http

import (
	"net/http"
	"strconv"
	"github.com/teachingassistant/billing-service/internal/app"
	"github.com/teachingassistant/billing-service/internal/app/command"
	"github.com/teachingassistant/billing-service/internal/app/query"
	"github.com/stripe/stripe-go/v76"
	"github.com/teachingassistant/billing-service/internal/domain"
	"github.com/labstack/echo/v5"
	"log/slog"
)

type BillingHandler struct {
	createCheckoutCmd   *command.CreateCheckoutSessionHandler
	createAddonCmd      *command.CreateAddonSessionHandler
	getBalanceQuery     *query.GetWalletBalanceHandler
	checkUsageQuery     *query.CheckUsageHandler
	createPortalCmd     *command.CreatePortalSessionHandler
	setupFreeTierCmd    *command.SetupFreeTierHandler
	getStatusQuery      *query.GetBillingStatusHandler
	createSetupCmd      *command.CreateSetupIntentHandler
	paymentMethodCmd    *command.PaymentMethodHandler
	getPMsQuery         *query.GetPaymentMethodsHandler
	listInvoicesQuery   *query.ListInvoicesHandler
	directChargeCmd     *command.DirectChargeHandler
	directSubCmd        *command.DirectSubscriptionHandler
	getPlansQuery       *query.GetPlansHandler
	getAddonsQuery      *query.GetAddonsHandler
	getUserAddonsQuery  *query.GetUserAddonsHandler
	getUsageLogQuery    *query.GetUsageLogHandler
	userRepo            domain.UserRepository
}

func NewBillingHandler(
	createCheckoutCmd *command.CreateCheckoutSessionHandler,
	createAddonCmd *command.CreateAddonSessionHandler,
	getBalanceQuery *query.GetWalletBalanceHandler,
	checkUsageQuery *query.CheckUsageHandler,
	createPortalCmd *command.CreatePortalSessionHandler,
	setupFreeTierCmd *command.SetupFreeTierHandler,
	getStatusQuery *query.GetBillingStatusHandler,
	createSetupCmd *command.CreateSetupIntentHandler,
	paymentMethodCmd *command.PaymentMethodHandler,
	getPMsQuery *query.GetPaymentMethodsHandler,
	listInvoicesQuery *query.ListInvoicesHandler,
	directChargeCmd *command.DirectChargeHandler,
	directSubCmd *command.DirectSubscriptionHandler,
	getPlansQuery *query.GetPlansHandler,
	getAddonsQuery *query.GetAddonsHandler,
	getUserAddonsQuery *query.GetUserAddonsHandler,
	getUsageLogQuery *query.GetUsageLogHandler,
	userRepo domain.UserRepository,
) *BillingHandler {
	return &BillingHandler{
		createCheckoutCmd: createCheckoutCmd,
		createAddonCmd:    createAddonCmd,
		getBalanceQuery:   getBalanceQuery,
		checkUsageQuery:   checkUsageQuery,
		createPortalCmd:   createPortalCmd,
		setupFreeTierCmd:  setupFreeTierCmd,
		getStatusQuery:    getStatusQuery,
		createSetupCmd:    createSetupCmd,
		paymentMethodCmd:  paymentMethodCmd,
		getPMsQuery:       getPMsQuery,
		listInvoicesQuery: listInvoicesQuery,
		directChargeCmd:   directChargeCmd,
		directSubCmd:      directSubCmd,
		getPlansQuery:     getPlansQuery,
		getAddonsQuery:    getAddonsQuery,
		getUserAddonsQuery: getUserAddonsQuery,
		getUsageLogQuery:  getUsageLogQuery,
		userRepo:          userRepo,
	}
}

// CreateCheckout godoc
// @Summary Create a Stripe checkout session
// @Description Creates a new Stripe checkout session for a subscription plan or AI credit pack
// @Tags billing
// @Accept json
// @Produce json
// @Param request body app.CheckoutParams true "Checkout parameters"
// @Success 200 {object} map[string]string "Returns the checkout URL"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/checkout [post]
func (h *BillingHandler) CreateCheckout(c *echo.Context) error {
	var params app.CheckoutParams
	if err := (*c).Bind(&params); err != nil {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	url, err := h.createCheckoutCmd.Handle((*c).Request().Context(), params)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, map[string]string{"url": url})
}

// ListPlans godoc
// @Summary List available subscription plans
// @Description Fetches active products with metadata type="plan" from Stripe
// @Tags billing
// @Produce json
// @Success 200 {array} app.PlanDTO
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/plans [get]
func (h *BillingHandler) GetPlans(c *echo.Context) error {
	plans, err := h.getPlansQuery.Handle((*c).Request().Context())
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return (*c).JSON(http.StatusOK, plans)
}

// ListAddons godoc
// @Summary List available addon packs
// @Description Fetches active products with metadata type="addon_ai" or "addon_storage" from Stripe
// @Tags billing
// @Produce json
// @Success 200 {array} app.PlanDTO
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/addons [get]
func (h *BillingHandler) GetAddons(c *echo.Context) error {
	addons, err := h.getAddonsQuery.Handle((*c).Request().Context())
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return (*c).JSON(http.StatusOK, addons)
}

// GetUserAddons godoc
// @Summary Get current user's purchased addon packs
// @Description Returns AI credit packs and storage packs owned by the user
// @Tags billing
// @Produce json
// @Param userId query string true "User ID"
// @Success 200 {object} query.UserAddonsDTO
// @Failure 400 {object} map[string]string
// @Router /api/v2/billing/user-addons [get]
func (h *BillingHandler) GetUserAddons(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	if userID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}
	result, err := h.getUserAddonsQuery.Handle((*c).Request().Context(), userID)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return (*c).JSON(http.StatusOK, result)
}

// GetUsageLog godoc
// @Summary Get usage event log for the user
// @Description Returns a list of recent usage events (AI tokens or storage bytes)
// @Tags billing
// @Produce json
// @Param userId query string true "User ID"
// @Param type query string false "Usage type: AI_TOKEN or STORAGE_BYTE (omit for both)"
// @Param limit query int false "Max results (default 50)"
// @Success 200 {array} query.UsageEventDTO
// @Failure 400 {object} map[string]string
// @Router /api/v2/billing/usage-log [get]
func (h *BillingHandler) GetUsageLog(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	if userID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	usageType := domain.UsageType((*c).QueryParam("type"))
	limit := 50
	if l := (*c).QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events, err := h.getUsageLogQuery.Handle((*c).Request().Context(), userID, usageType, limit)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return (*c).JSON(http.StatusOK, events)
}


// CreateAddon godoc
// @Summary Create a Stripe session for an add-on
// @Description Creates a new Stripe checkout session for a storage add-on
// @Tags billing
// @Accept json
// @Produce json
// @Param request body app.AddonParams true "Add-on parameters"
// @Success 200 {object} map[string]string "Returns the checkout URL"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/addon [post]
func (h *BillingHandler) CreateAddon(c *echo.Context) error {
	var params app.AddonParams
	if err := (*c).Bind(&params); err != nil {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	url, err := h.createAddonCmd.Handle((*c).Request().Context(), params)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, map[string]string{"url": url})
}

// GetBalance godoc
// @Summary Get user wallet balance
// @Description Returns the current AI credit balance for a user
// @Tags billing
// @Produce json
// @Param userId query string true "User ID"
// @Success 200 {object} map[string]interface{} "Returns the wallet balance"
// @Failure 400 {object} map[string]string "userId is required"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/balance [get]
func (h *BillingHandler) GetBalance(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	if userID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	balance, err := h.getBalanceQuery.Handle((*c).Request().Context(), userID)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, map[string]interface{}{"balance": balance})
}

// CheckUsage godoc
// @Summary Check and deduct usage
// @Description Verifies if a user has enough credits/storage and deducts if necessary
// @Tags billing
// @Produce json
// @Param userId query string true "User ID"
// @Param type query string true "Usage type (ai_tokens, storage)"
// @Param amount query integer false "Amount to check/deduct (default: 1)"
// @Success 200 {object} map[string]interface{} "Status OK"
// @Failure 400 {object} map[string]string "Invalid parameters"
// @Failure 402 {object} map[string]string "Usage limit exceeded"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/check-usage [get]
func (h *BillingHandler) CheckUsage(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	if userID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	usageType := (*c).QueryParam("type")
	if usageType == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "type is required"})
	}

	amountStr := (*c).QueryParam("amount")
	amount := 1
	if amountStr != "" {
		var err error
		amount, err = strconv.Atoi(amountStr)
		if err != nil {
			return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "invalid amount"})
		}
	}

	err := h.checkUsageQuery.Handle((*c).Request().Context(), userID, domain.UsageType(usageType), amount)
	if err != nil {
		if err == query.ErrUsageExceeded {
			return (*c).JSON(http.StatusPaymentRequired, map[string]string{"error": err.Error()})
		}
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, map[string]interface{}{"status": "ok"})
}

// CreatePortal godoc
// @Summary Create a Stripe customer portal session
// @Description Creates a session for the Stripe customer portal where users can manage subscriptions
// @Tags billing
// @Produce json
// @Param userId query string true "User ID"
// @Param returnUrl query string false "URL to return to after leaving the portal"
// @Success 200 {object} map[string]string "Returns the portal URL"
// @Failure 400 {object} map[string]string "userId is required"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/portal [post]
func (h *BillingHandler) CreatePortal(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	if userID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	returnURL := (*c).QueryParam("returnUrl")
	if returnURL == "" {
		returnURL = "http://localhost:3000/billing" // Fallback
	}

	url, err := h.createPortalCmd.Handle((*c).Request().Context(), userID, returnURL)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, map[string]string{"url": url})
}

// SetupFreeTier godoc
// @Summary Setup free tier for a new user
// @Description Automatically assigns a Free tier subscription to a new user
// @Tags billing
// @Accept json
// @Produce json
// @Param request body command.SetupFreeTierParams true "Setup parameters (email, name, address, etc)"
// @Success 200 {object} map[string]string "Success message"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/setup-free-tier [post]
func (h *BillingHandler) SetupFreeTier(c *echo.Context) error {
	var params command.SetupFreeTierParams
	if err := (*c).Bind(&params); err != nil {
		return (*c).JSON(400, map[string]string{"error": "invalid request body"})
	}

	if err := h.setupFreeTierCmd.Handle((*c).Request().Context(), params); err != nil {
		return (*c).JSON(500, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(200, map[string]string{"message": "free tier setup successfully"})
}

// SyncCustomer godoc
// @Summary Sync customer data with Stripe
// @Description Synchronizes user profile changes (name, email, address, etc) with Stripe
// @Tags billing
// @Accept json
// @Produce json
// @Param request body object true "Sync parameters"
// @Success 200 {object} map[string]string "Success message"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 404 {object} map[string]string "Customer not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/sync-customer [post]
func (h *BillingHandler) SyncCustomer(c *echo.Context) error {
	var params struct {
		UserID   string      `json:"userId"`
		Email    string      `json:"email"`
		Name     string      `json:"name"`
		Address  app.Address `json:"address"`
		Timezone string      `json:"timezone"`
		Country  string      `json:"country"`
	}
	if err := (*c).Bind(&params); err != nil {
		return (*c).JSON(400, map[string]string{"error": "invalid request body"})
	}

	// Find subscription to get stripe customer id
	sub, err := h.checkUsageQuery.GetSubscription((*c).Request().Context(), params.UserID)
	if err != nil || sub == nil || sub.StripeCustomerID == "" {
		return (*c).JSON(404, map[string]string{"error": "stripe customer not found for user"})
	}

	stripeParams := &stripe.CustomerParams{
		Email: stripe.String(params.Email),
		Name:  stripe.String(params.Name),
		Address: &stripe.AddressParams{
			Line1:      stripe.String(params.Address.Line1),
			City:       stripe.String(params.Address.City),
			State:      stripe.String(params.Address.State),
			PostalCode: stripe.String(params.Address.PostalCode),
			Country:    stripe.String(params.Address.Country),
		},
		Metadata: map[string]string{
			"timezone": params.Timezone,
			"country":  params.Country,
		},
	}

	if _, err := h.createCheckoutCmd.Stripe().UpdateCustomer((*c).Request().Context(), sub.StripeCustomerID, stripeParams, ""); err != nil {
		return (*c).JSON(500, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(200, map[string]string{"message": "customer synced successfully"})
}

// GetStatus godoc
// @Summary Get billing status
// @Description Returns the full billing status, subscription details, usage, and wallet balance
// @Tags billing
// @Produce json
// @Param userId query string true "User ID"
// @Success 200 {object} query.BillingStatusDTO "Returns the billing status"
// @Failure 400 {object} map[string]string "userId is required"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v2/billing/status [get]
func (h *BillingHandler) GetStatus(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	slog.Info("GetStatus request received", "userId", userID)
	
	if userID == "" {
		slog.Warn("GetStatus failed: userId is required")
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	status, err := h.getStatusQuery.Handle((*c).Request().Context(), userID)
	if err != nil {
		if err == domain.ErrSubscriptionNotFound {
			slog.Info("subscription not found, attempting auto-creation", "userId", userID)
			
			// 1. Fetch user info
			user, uErr := h.userRepo.GetByID((*c).Request().Context(), userID)
			if uErr != nil {
				slog.Error("failed to fetch user for auto-creation", "userId", userID, "error", uErr)
				return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch user info for auto-creation"})
			}

			// 2. Setup Free Tier
			setupParams := command.SetupFreeTierParams{
				UserID:    user.ID,
				Email:     user.Email,
				FirstName: user.Name, // We can split or just use as is
			}
			if err := h.setupFreeTierCmd.Handle((*c).Request().Context(), setupParams); err != nil {
				slog.Error("failed to auto-create free tier", "userId", userID, "error", err)
				return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": "failed to setup free tier"})
			}

			// 3. Retry getting status
			status, err = h.getStatusQuery.Handle((*c).Request().Context(), userID)
			if err != nil {
				slog.Error("failed to fetch status after auto-creation", "userId", userID, "error", err)
				return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		} else {
			slog.Error("GetStatus query failed", "userId", userID, "error", err)
			return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	slog.Info("GetStatus response sent", "userId", userID, "status", status.Subscription.Status)
	return (*c).JSON(http.StatusOK, status)
}

func (h *BillingHandler) CreateSetupIntent(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	if userID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	si, err := h.createSetupCmd.Handle((*c).Request().Context(), userID)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, si)
}

func (h *BillingHandler) GetPaymentMethods(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	if userID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	pms, err := h.getPMsQuery.Handle((*c).Request().Context(), userID)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, pms)
}

func (h *BillingHandler) SetDefaultPaymentMethod(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	if userID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	var req struct {
		PaymentMethodID string `json:"paymentMethodId"`
	}
	if err := (*c).Bind(&req); err != nil {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := h.paymentMethodCmd.SetDefault((*c).Request().Context(), userID, req.PaymentMethodID); err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *BillingHandler) DetachPaymentMethod(c *echo.Context) error {
	pmID := c.Param("id")
	if pmID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "paymentMethodId is required"})
	}

	if err := h.paymentMethodCmd.Detach((*c).Request().Context(), pmID); err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *BillingHandler) ListInvoices(c *echo.Context) error {
	userID := (*c).QueryParam("userId")
	if userID == "" {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	limitStr := (*c).QueryParam("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	invoices, err := h.listInvoicesQuery.Handle((*c).Request().Context(), userID, limit)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, invoices)
}

func (h *BillingHandler) DirectCharge(c *echo.Context) error {
	var cmd command.DirectChargeCmd
	if err := (*c).Bind(&cmd); err != nil {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	pi, err := h.directChargeCmd.Handle((*c).Request().Context(), cmd)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, pi)
}

func (h *BillingHandler) DirectSubscription(c *echo.Context) error {
	var cmd command.DirectSubscriptionCmd
	if err := (*c).Bind(&cmd); err != nil {
		return (*c).JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	sub, err := h.directSubCmd.Handle((*c).Request().Context(), cmd)
	if err != nil {
		return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return (*c).JSON(http.StatusOK, sub)
}
