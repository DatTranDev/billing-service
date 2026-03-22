package main

import (
	"context"
	"log"
	"os"
	"strings"

	"log/slog"

	"github.com/teachingassistant/billing-service/internal/app/command"
	"github.com/teachingassistant/billing-service/internal/app/query"
	"github.com/teachingassistant/billing-service/internal/app/worker"
	"github.com/teachingassistant/billing-service/internal/infra/mongo"
	"github.com/teachingassistant/billing-service/internal/infra/stripe"
	"github.com/teachingassistant/billing-service/internal/interface/http"

	stdhttp "net/http"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/teachingassistant/billing-service/docs"
	mongo_driver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// @title Teaching Assistant Billing Service API
// @version 2.0
// @description API for managing subscriptions, AI credits, and Stripe integration.
// @host localhost:9003
// @BasePath /
func main() {
	Run()
}

func Run() {
	if err := godotenv.Overload(); err != nil {
		log.Println("No .env file found")
	}

	// 1. Infrastructure Setup
	mongoURI := os.Getenv("MONGO_URI")
	dbName := os.Getenv("MONGO_DB")
	client, err := mongo_driver.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	db := client.Database(dbName)
	testDb := client.Database("test") // Main user data is here

	walletRepo := mongo.NewWalletRepository(db)
	subRepo := mongo.NewSubscriptionRepository(db)
	usageRepo := mongo.NewUsageRepository(db)
	eventRepo := mongo.NewStripeEventRepository(db)
	cacheRepo := mongo.NewStripeCacheRepository(db)
	ledgerRepo := mongo.NewLedgerRepository(db)
	userRepo := mongo.NewUserRepository(testDb)

	stripeAdapter := stripe.NewStripeAdapter(os.Getenv("STRIPE_API_KEY"), cacheRepo)
	allowOrigins := parseAllowOrigins(os.Getenv("CORS_ALLOW_ORIGINS"))
	jwtSecret := os.Getenv("BILLING_JWT_SECRET")
	if strings.TrimSpace(jwtSecret) == "" {
		jwtSecret = os.Getenv("SECRET_KEY")
	}

	// 2. Application Layer (CQRS) Setup
	createCheckoutCmd := command.NewCreateCheckoutSessionHandler(stripeAdapter, subRepo)
	createAddonCmd := command.NewCreateAddonSessionHandler(stripeAdapter, subRepo)
	consumeUsageCmd := command.NewConsumeUsageHandler(walletRepo, subRepo, usageRepo)
	createPortalCmd := command.NewCreatePortalSessionHandler(stripeAdapter, subRepo)
	setupFreeTierCmd := command.NewSetupFreeTierHandler(stripeAdapter, subRepo, walletRepo)
 
	getBalanceQuery := query.NewGetWalletBalanceHandler(walletRepo)
	checkUsageQuery := query.NewCheckUsageHandler(walletRepo, subRepo, usageRepo)
	getStatusQuery := query.NewGetBillingStatusHandler(subRepo, walletRepo, usageRepo)
	createSetupCmd := command.NewCreateSetupIntentHandler(stripeAdapter, subRepo)
	paymentMethodCmd := command.NewPaymentMethodHandler(stripeAdapter, subRepo)
	getPMsQuery := query.NewGetPaymentMethodsHandler(stripeAdapter, subRepo)
	listInvoicesQuery := query.NewListInvoicesHandler(stripeAdapter, subRepo)
	directChargeCmd := command.NewDirectChargeHandler(stripeAdapter, subRepo)
	directSubCmd := command.NewDirectSubscriptionHandler(stripeAdapter, subRepo)
	getPlansQuery := query.NewGetPlansHandler(stripeAdapter)
	getAddonsQuery := query.NewGetAddonsHandler(stripeAdapter)
	getUserAddonsQuery := query.NewGetUserAddonsHandler(walletRepo, subRepo)
	getUsageLogQuery := query.NewGetUsageLogHandler(usageRepo)

	// Start Background Workers
	creditResetWorker := worker.NewResetCreditsWorker(subRepo, walletRepo)
	cronJob := creditResetWorker.Start()
	if cronJob != nil {
		defer cronJob.Stop()
	}

	// 3. Interface Layer Setup
	handler := http.NewBillingHandler(
		createCheckoutCmd,
		createAddonCmd,
		consumeUsageCmd,
		getBalanceQuery,
		checkUsageQuery,
		createPortalCmd,
		setupFreeTierCmd,
		getStatusQuery,
		createSetupCmd,
		paymentMethodCmd,
		getPMsQuery,
		listInvoicesQuery,
		directChargeCmd,
		directSubCmd,
		getPlansQuery,
		getAddonsQuery,
		getUserAddonsQuery,
		getUsageLogQuery,
		userRepo,
	)
	webhookHandler := http.NewWebhookHandler(stripeAdapter, eventRepo, subRepo, walletRepo, ledgerRepo, cacheRepo)

	e := echo.New()
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			slog.Info("request",
				slog.String("method", v.Method),
				slog.String("URI", v.URI),
				slog.Int("status", v.Status),
			)
			return nil
		},
	}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowOrigins,
		AllowMethods:     []string{stdhttp.MethodGet, stdhttp.MethodPost, stdhttp.MethodOptions, stdhttp.MethodDelete},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
	e.Use(middleware.Recover())

	publicBilling := e.Group("/api/v2/billing")
	billing := e.Group("/api/v2/billing", http.RequireAuthMiddleware(jwtSecret))
	billing.POST("/checkout", handler.CreateCheckout)
	billing.POST("/addon", handler.CreateAddon)
	billing.GET("/balance", handler.GetBalance)
	billing.GET("/check-usage", handler.CheckUsage)
	billing.POST("/deduct", handler.DeductUsage)
	billing.POST("/portal", handler.CreatePortal)
	billing.POST("/setup-free-tier", handler.SetupFreeTier)
	billing.GET("/sync-customer", handler.SyncCustomer) // Changed POST to GET if it was intended to be GET, but keeping as is for now if it was POST
	billing.POST("/sync-customer", handler.SyncCustomer)
	billing.GET("/status", handler.GetStatus)
	publicBilling.GET("/plans", handler.GetPlans)
	publicBilling.GET("/addons", handler.GetAddons)
	billing.GET("/user-addons", handler.GetUserAddons)
	billing.GET("/usage-log", handler.GetUsageLog)
	billing.POST("/setup-intent", handler.CreateSetupIntent)
	billing.GET("/payment-methods", handler.GetPaymentMethods)
	billing.POST("/payment-methods/default", handler.SetDefaultPaymentMethod)
	billing.DELETE("/payment-methods/:id", handler.DetachPaymentMethod)
	billing.GET("/invoices", handler.ListInvoices)
	billing.POST("/charge", handler.DirectCharge)
	billing.POST("/subscribe", handler.DirectSubscription)
	publicBilling.POST("/webhook", webhookHandler.Handle)

	e.GET("/swagger/*", func(c *echo.Context) error {
		httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"), // The url pointing to API definition
		)(c.Response(), c.Request())
		return nil
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "9003"
	}
	log.Printf("Billing service starting on port %s", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatal(err)
	}
}

func parseAllowOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
