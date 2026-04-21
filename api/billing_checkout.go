package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v76"
	stripecustomer "github.com/stripe/stripe-go/v76/customer"
	stripesession "github.com/stripe/stripe-go/v76/checkout/session"

	"velarix/store"
)

type checkoutSessionRequest struct {
	Plan       string `json:"plan"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

type checkoutSessionResponse struct {
	CheckoutURL string `json:"checkout_url"`
}

func (s *Server) handleCreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	if s.StripeSecretKey == "" {
		http.Error(w, "billing not configured", http.StatusServiceUnavailable)
		return
	}

	orgID := getOrgID(r)

	var req checkoutSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	plan := strings.ToLower(strings.TrimSpace(req.Plan))
	priceID := s.priceIDForPlan(plan)
	if priceID == "" {
		http.Error(w, fmt.Sprintf("unknown plan %q or price not configured", plan), http.StatusBadRequest)
		return
	}

	successURL := strings.TrimSpace(req.SuccessURL)
	cancelURL := strings.TrimSpace(req.CancelURL)
	if successURL == "" || cancelURL == "" {
		http.Error(w, "success_url and cancel_url are required", http.StatusBadRequest)
		return
	}

	stripe.Key = s.StripeSecretKey

	customerID, billingEmail := s.resolveStripeCustomer(orgID, getUserEmail(r))

	sessionParams := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(cancelURL),
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"velarix_org_id": orgID,
				"velarix_plan":   plan,
				"billing_email":  billingEmail,
			},
		},
	}

	if customerID != "" {
		sessionParams.Customer = stripe.String(customerID)
	} else if billingEmail != "" {
		sessionParams.CustomerEmail = stripe.String(billingEmail)
	}

	sess, err := stripesession.New(sessionParams)
	if err != nil {
		http.Error(w, "failed to create checkout session", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, checkoutSessionResponse{CheckoutURL: sess.URL})
}

// resolveStripeCustomer returns the existing Stripe customer ID for the org if
// one is recorded, and the billing email. If no customer exists yet, Stripe
// will create one during checkout.
func (s *Server) resolveStripeCustomer(orgID, fallbackEmail string) (customerID, email string) {
	sub, err := s.Store.GetBilling(orgID)
	if err != nil || sub == nil {
		return "", fallbackEmail
	}
	if sub.BillingEmail != "" {
		email = sub.BillingEmail
	} else {
		email = fallbackEmail
	}
	if sub.StripeCustomerID != "" {
		return sub.StripeCustomerID, email
	}

	// No customer ID recorded yet — create one so that future subscriptions are
	// attached to the same customer even if the user abandons checkout.
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"velarix_org_id": orgID,
		},
	}
	c, err := stripecustomer.New(params)
	if err != nil {
		return "", email
	}

	// Persist the new customer ID so subsequent calls reuse it.
	if sub == nil {
		sub = &store.BillingSubscription{Plan: "free", Status: "active", BillingEmail: email}
	}
	sub.StripeCustomerID = c.ID
	sub.UpdatedAt = time.Now().UnixMilli()
	_ = s.Store.SaveBilling(orgID, sub)

	return c.ID, email
}

func (s *Server) priceIDForPlan(plan string) string {
	switch plan {
	case "pro":
		return s.StripeProPriceID
	case "enterprise":
		return s.StripeEnterprisePriceID
	default:
		return ""
	}
}
