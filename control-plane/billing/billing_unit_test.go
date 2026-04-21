package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v76"
	"velarix/store"
)

func TestFirstNonEmpty_FirstNonEmptyString_ReturnsFirstTrimmed(t *testing.T) {
	if res := firstNonEmpty("", "  ", "hello", "world"); res != "hello" {
		t.Errorf("expected hello, got %s", res)
	}
	if res := firstNonEmpty(); res != "" {
		t.Errorf("expected empty, got %s", res)
	}
}

func TestFeaturesForPlan_Plans_ReturnExpectedFeatures(t *testing.T) {
	ent := featuresForPlan("enterprise")
	if !ent["compliance_export"] || !ent["human_review"] || !ent["priority_support"] {
		t.Errorf("enterprise missing features: %v", ent)
	}
	pro := featuresForPlan(" pro ")
	if !pro["compliance_export"] || !pro["human_review"] || pro["priority_support"] {
		t.Errorf("pro missing features: %v", pro)
	}
	free := featuresForPlan("free")
	if len(free) != 0 {
		t.Errorf("free should have no features, got: %v", free)
	}
}

func TestResolvePlan_MetadataAndDeletion_ReturnExpectedPlan(t *testing.T) {
	if resolvePlan(nil, true) != "free" {
		t.Error("deleted should be free")
	}
	if resolvePlan(nil, false) != "free" {
		t.Error("nil should be free")
	}

	sub := &stripe.Subscription{
		Metadata: map[string]string{
			"velarix_plan": "Enterprise ",
		},
	}
	if resolvePlan(sub, false) != "enterprise" {
		t.Error("expected enterprise")
	}

	sub.Metadata = map[string]string{
		"plan": "Pro",
	}
	if resolvePlan(sub, false) != "pro" {
		t.Error("expected pro")
	}

	sub.Metadata = map[string]string{}
	if resolvePlan(sub, false) != "pro" {
		t.Error("expected default pro")
	}
}

type mockBillingStore struct {
	store.BillingStore
	saved map[string]*store.BillingSubscription
}

func (m *mockBillingStore) GetBilling(orgID string) (*store.BillingSubscription, error) {
	return m.saved[orgID], nil
}

func (m *mockBillingStore) SaveBilling(orgID string, billing *store.BillingSubscription) error {
	if m.saved == nil {
		m.saved = make(map[string]*store.BillingSubscription)
	}
	m.saved[orgID] = billing
	return nil
}

func (m *mockBillingStore) IsStripeEventProcessed(_ string) (bool, error) { return false, nil }
func (m *mockBillingStore) MarkStripeEventProcessed(_ string) error       { return nil }

func TestHandleSubscriptionEvent_UpsertAndDelete_PersistsBillingSubscription(t *testing.T) {
	ms := &mockBillingStore{}

	sub := stripe.Subscription{
		ID:     "sub_123",
		Status: stripe.SubscriptionStatusActive,
		Customer: &stripe.Customer{
			ID: "cus_123",
		},
		CurrentPeriodEnd: time.Now().Unix(),
		Metadata: map[string]string{
			"org_id": "org_1",
			"plan":   "enterprise",
		},
		Items: &stripe.SubscriptionItemList{
			Data: []*stripe.SubscriptionItem{
				{Quantity: 5},
			},
		},
	}

	raw, _ := json.Marshal(sub)
	event := stripe.Event{
		ID:   "evt_123",
		Type: "customer.subscription.updated",
		Data: &stripe.EventData{
			Raw: json.RawMessage(raw),
		},
	}

	err := handleSubscriptionEvent(ms, event, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, _ := ms.GetBilling("org_1")
	if b == nil || b.Plan != "enterprise" || b.Seats != 5 || b.StripeSubscriptionID != "sub_123" {
		t.Errorf("unexpected billing state: %+v", b)
	}

	err = handleSubscriptionEvent(ms, event, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, _ = ms.GetBilling("org_1")
	if b == nil || b.Plan != "free" || b.Status != "canceled" {
		t.Errorf("unexpected billing state after deletion: %+v", b)
	}

	subNoOrg := sub
	subNoOrg.Metadata = map[string]string{}
	rawNoOrg, _ := json.Marshal(subNoOrg)
	eventNoOrg := event
	eventNoOrg.Data.Raw = json.RawMessage(rawNoOrg)

	err = handleSubscriptionEvent(ms, eventNoOrg, false)
	if err == nil {
		t.Error("expected error for missing org_id")
	}
}
