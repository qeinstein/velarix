package core

import (
	"errors"
	"strings"
	"time"
)

const (
	ReviewPending  = "pending"
	ReviewApproved = "approved"
	ReviewWaived   = "waived"
	ReviewRejected = "rejected"
)

func NormalizeReviewStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case ReviewApproved:
		return ReviewApproved
	case ReviewWaived:
		return ReviewWaived
	case ReviewRejected:
		return ReviewRejected
	case ReviewPending:
		return ReviewPending
	default:
		return ""
	}
}

func ClampUnitFloat(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func MetadataBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	raw, ok := m[key]
	if !ok {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "required":
			return true
		}
	}
	return false
}

func MetadataString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func (f *Fact) EffectiveEntrenchment() float64 {
	if f == nil {
		return 0
	}
	if f.Entrenchment > 0 {
		return ClampUnitFloat(f.Entrenchment)
	}
	if raw := MetadataString(f.Metadata, "entrenchment"); raw != "" {
		if raw == "high" {
			return 0.95
		}
		if raw == "medium" {
			return 0.75
		}
	}
	return 0
}

func (f *Fact) RequiresHumanReview() bool {
	if f == nil {
		return false
	}
	if MetadataBool(f.Metadata, "requires_human_review") {
		status := NormalizeReviewStatus(f.ReviewStatus)
		return status != ReviewApproved && status != ReviewWaived
	}
	status := NormalizeReviewStatus(f.ReviewStatus)
	return status == ReviewPending
}

func (e *Engine) SetFactReview(factID string, status string, reason string, reviewedAt int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fact, ok := e.Facts[factID]
	if !ok {
		return errors.New("fact not found")
	}
	status = NormalizeReviewStatus(status)
	if status == "" {
		return errors.New("invalid review status")
	}
	if reviewedAt == 0 {
		reviewedAt = time.Now().UnixMilli()
	}
	fact.ReviewStatus = status
	fact.ReviewReason = strings.TrimSpace(reason)
	fact.ReviewedAt = reviewedAt
	if fact.Metadata == nil {
		fact.Metadata = map[string]interface{}{}
	}
	fact.Metadata["review_status"] = status
	fact.Metadata["review_reason"] = fact.ReviewReason
	fact.Metadata["reviewed_at"] = reviewedAt
	if status == ReviewApproved || status == ReviewWaived {
		fact.Metadata["requires_human_review"] = false
	}
	return nil
}
