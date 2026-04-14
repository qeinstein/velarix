package core

import (
	"errors"
	"strings"
	"time"
)

const (
	VerificationUnverified = "unverified"
	VerificationVerified   = "verified"
	VerificationRejected   = "rejected"
)

func NormalizeVerificationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case VerificationUnverified:
		return VerificationUnverified
	case VerificationVerified:
		return VerificationVerified
	case VerificationRejected:
		return VerificationRejected
	default:
		return ""
	}
}

func FactVerificationStatus(f *Fact) string {
	if f == nil || f.Metadata == nil {
		return ""
	}
	if v, ok := f.Metadata["verification_status"].(string); ok {
		return NormalizeVerificationStatus(v)
	}
	return ""
}

func FactSourceType(f *Fact) string {
	if f == nil {
		return ""
	}
	// Prefer explicit provenance.
	if f.Metadata != nil {
		if prov, ok := f.Metadata["_provenance"].(map[string]interface{}); ok {
			if v, ok := prov["source_type"].(string); ok {
				return strings.TrimSpace(v)
			}
		}
		if v, ok := f.Metadata["source_type"].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func FactClaimKey(f *Fact) string {
	if f == nil {
		return ""
	}
	if f.Payload != nil {
		if v, ok := f.Payload["claim_key"].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	if f.Metadata != nil {
		if v, ok := f.Metadata["claim_key"].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// SetFactVerification updates verification metadata and propagates changes so
// execution-critical descendants can become valid once verified.
func (e *Engine) SetFactVerification(factID string, status string, method string, sourceRef string, reason string, verifiedAt int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fact, ok := e.Facts[factID]
	if !ok {
		return errors.New("fact not found")
	}
	status = NormalizeVerificationStatus(status)
	if status == "" {
		return errors.New("invalid verification status")
	}
	if verifiedAt == 0 {
		verifiedAt = time.Now().UnixMilli()
	}
	if fact.Metadata == nil {
		fact.Metadata = map[string]interface{}{}
	}
	fact.Metadata["verification_status"] = status
	if strings.TrimSpace(method) != "" {
		fact.Metadata["verification_method"] = strings.TrimSpace(method)
	}
	if strings.TrimSpace(sourceRef) != "" {
		fact.Metadata["verification_source_ref"] = strings.TrimSpace(sourceRef)
	}
	if strings.TrimSpace(reason) != "" {
		fact.Metadata["verification_reason"] = strings.TrimSpace(reason)
	}
	fact.Metadata["verified_at"] = verifiedAt
	if status == VerificationVerified {
		fact.Metadata["requires_verification"] = false
	}

	e.MutationCount++
	// Verification changes can affect dependency satisfaction without changing
	// the parent's DerivedStatus, so we must recompute child justification sets.
	queue := e.recomputeChildrenForParentUnsafe(factID)
	if len(queue) > 0 {
		e.propagate(queue)
	}
	return nil
}
