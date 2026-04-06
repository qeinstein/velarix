package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
)

// This is a standalone microservice for the Control Plane.
// It listens for Stripe webhooks and updates the central Velarix Postgres database
// to grant or revoke Enterprise access based on payment status.

func main() {
	_ = godotenv.Load("../../.env") // Load root env if running locally

	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")

	if stripe.Key == "" || webhookSecret == "" {
		log.Println("WARNING: Stripe keys not configured. Billing webhook server will fail on requests.")
	}

	http.HandleFunc("/webhooks/stripe", func(w http.ResponseWriter, req *http.Request) {
		const MaxBodyBytes = int64(65536)
		req.Body = http.MaxBytesReader(w, req.Body, MaxBodyBytes)
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading request body: %v\n", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		event, err := webhook.ConstructEvent(payload, req.Header.Get("Stripe-Signature"), webhookSecret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error verifying webhook signature: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch event.Type {
		case "customer.subscription.created", "customer.subscription.updated":
			var sub stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &sub)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			log.Printf("Subscription [%s] updated for customer [%s]. Status: %s\n", sub.ID, sub.Customer.ID, sub.Status)
			
			// TODO: Update the Postgres `billing_subscriptions` table via shared store
			// store.UpdateBillingStatusByStripeCustomerID(sub.Customer.ID, string(sub.Status))

		case "customer.subscription.deleted":
			var sub stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &sub)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			log.Printf("Subscription [%s] canceled for customer [%s]. Downgrading to Free.\n", sub.ID, sub.Customer.ID)
			
			// TODO: Downgrade in Postgres
			// store.UpdateBillingStatusByStripeCustomerID(sub.Customer.ID, "canceled")

		default:
			fmt.Fprintf(os.Stderr, "Unhandled event type: %s\n", event.Type)
		}

		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("BILLING_PORT")
	if port == "" {
		port = "8081"
	}
	
	log.Printf("Control Plane: Billing Webhook Server listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
