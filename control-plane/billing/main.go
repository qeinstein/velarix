package main

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

	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"

	"velarix/store"
	"velarix/store/postgres"
)

func main() {
	_ = godotenv.Load("../../.env")

	stripe.Key = strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
	webhookSecret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	postgresDSN := strings.TrimSpace(os.Getenv("VELARIX_POSTGRES_DSN"))
	env := strings.ToLower(strings.TrimSpace(os.Getenv("VELARIX_ENV")))
	if env == "" {
		env = "prod"
	}
	devLike := env == "dev" || env == "test"
	if stripe.Key == "" || webhookSecret == "" {
		if !devLike {
			fatal("STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET are required outside dev/test")
		}
		slog.Warn("Stripe keys not configured; billing webhook verification will fail")
	}
	if postgresDSN == "" {
		if !devLike {
			fatal("VELARIX_POSTGRES_DSN is required outside dev/test")
		}
		slog.Warn("VELARIX_POSTGRES_DSN is not configured; billing updates will be ignored")
	}

	var billingStore store.BillingStore
	if postgresDSN != "" {
		pgStore, err := postgres.Open(context.Background(), postgresDSN)
		if err != nil {
			fatal("failed to open postgres billing store", "error", err)
		}
		defer pgStore.Close()
		billingStore = pgStore
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/webhooks/stripe", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if billingStore == nil {
			http.Error(w, "billing store is not configured", http.StatusServiceUnavailable)
			return
		}

		req.Body = http.MaxBytesReader(w, req.Body, 65536)
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusServiceUnavailable)
			return
		}

		event, err := webhook.ConstructEvent(payload, req.Header.Get("Stripe-Signature"), webhookSecret)
		if err != nil {
			http.Error(w, "invalid webhook signature", http.StatusBadRequest)
			return
		}

		switch event.Type {
		case "customer.subscription.created", "customer.subscription.updated":
			if err := handleSubscriptionEvent(billingStore, event, false); err != nil {
				slog.Error("billing update failed", "event_type", event.Type, "error", err)
				http.Error(w, "billing update failed", http.StatusAccepted)
				return
			}
		case "customer.subscription.deleted":
			if err := handleSubscriptionEvent(billingStore, event, true); err != nil {
				slog.Error("billing cancellation failed", "error", err)
				http.Error(w, "billing cancellation failed", http.StatusAccepted)
				return
			}
		default:
			slog.Info("Unhandled Stripe event type", "event_type", event.Type)
		}

		w.WriteHeader(http.StatusOK)
	})

	port := strings.TrimSpace(os.Getenv("BILLING_PORT"))
	if port == "" {
		port = "8081"
	}

	slog.Info("Control Plane billing webhook server listening", "port", port)
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	fatalOnError(httpServer.ListenAndServe())
}

func handleSubscriptionEvent(billingStore store.BillingStore, event stripe.Event, deleted bool) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return fmt.Errorf("parse subscription payload: %w", err)
	}

	orgID := firstNonEmpty(
		subscription.Metadata["velarix_org_id"],
		subscription.Metadata["org_id"],
	)
	if orgID == "" {
		return fmt.Errorf("missing org metadata on subscription %s", subscription.ID)
	}

	current, _ := billingStore.GetBilling(orgID)
	if current == nil {
		current = &store.BillingSubscription{
			Plan:         "free",
			Status:       "active",
			BillingEmail: subscription.Metadata["billing_email"],
		}
	}

	current.Plan = resolvePlan(&subscription, deleted)
	current.Status = strings.ToLower(strings.TrimSpace(string(subscription.Status)))
	if deleted {
		current.Status = "canceled"
	}
	current.StripeSubscriptionID = subscription.ID
	current.StripeCustomerID = subscription.Customer.ID
	current.CurrentPeriodEnd = subscription.CurrentPeriodEnd
	current.Seats = 1
	if subscription.Items != nil {
		for _, item := range subscription.Items.Data {
			if item == nil {
				continue
			}
			if item.Quantity > 0 {
				current.Seats = int(item.Quantity)
				break
			}
		}
	}
	if current.BillingEmail == "" {
		current.BillingEmail = subscription.Metadata["billing_email"]
	}
	current.Features = featuresForPlan(current.Plan)
	current.Metadata = map[string]string{
		"org_id":          orgID,
		"stripe_event_id": event.ID,
		"updated_from":    string(event.Type),
	}
	current.UpdatedAt = time.Now().UnixMilli()

	return billingStore.SaveBilling(orgID, current)
}

func resolvePlan(subscription *stripe.Subscription, deleted bool) string {
	if deleted {
		return "free"
	}
	if subscription == nil {
		return "free"
	}
	for _, key := range []string{"velarix_plan", "plan"} {
		if plan := strings.ToLower(strings.TrimSpace(subscription.Metadata[key])); plan != "" {
			return plan
		}
	}
	return "pro"
}

func featuresForPlan(plan string) map[string]bool {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "enterprise":
		return map[string]bool{
			"compliance_export": true,
			"human_review":      true,
			"priority_support":  true,
		}
	case "pro":
		return map[string]bool{
			"compliance_export": true,
			"human_review":      true,
		}
	default:
		return map[string]bool{}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func fatalOnError(err error) {
	if err != nil {
		fatal("billing webhook server stopped", "error", err)
	}
}
