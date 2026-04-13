package core

import (
	"testing"
	"time"
)

func TestNormalizeReviewStatus(t *testing.T) {
	if s := NormalizeReviewStatus(" APPROVED "); s != ReviewApproved {
		t.Error(s)
	}
	if s := NormalizeReviewStatus("WAIVED"); s != ReviewWaived {
		t.Error(s)
	}
	if s := NormalizeReviewStatus("rejected"); s != ReviewRejected {
		t.Error(s)
	}
	if s := NormalizeReviewStatus("pending"); s != ReviewPending {
		t.Error(s)
	}
	if s := NormalizeReviewStatus("other"); s != "" {
		t.Error(s)
	}
}

func TestClampUnitFloat(t *testing.T) {
	if c := ClampUnitFloat(-1); c != 0 {
		t.Error(c)
	}
	if c := ClampUnitFloat(2); c != 1 {
		t.Error(c)
	}
	if c := ClampUnitFloat(0.5); c != 0.5 {
		t.Error(c)
	}
}

func TestMetadataBool(t *testing.T) {
	if MetadataBool(nil, "k") {
		t.Error("expected false")
	}
	m := map[string]interface{}{"b": true, "s1": "yes", "s2": "false", "i": 1}
	if !MetadataBool(m, "b") {
		t.Error()
	}
	if !MetadataBool(m, "s1") {
		t.Error()
	}
	if MetadataBool(m, "s2") {
		t.Error()
	}
	if MetadataBool(m, "i") {
		t.Error()
	}
	if MetadataBool(m, "missing") {
		t.Error()
	}
}

func TestMetadataString(t *testing.T) {
	if MetadataString(nil, "k") != "" {
		t.Error()
	}
	m := map[string]interface{}{"s": " foo ", "i": 1}
	if MetadataString(m, "s") != "foo" {
		t.Error()
	}
	if MetadataString(m, "i") != "" {
		t.Error()
	}
}

func TestEffectiveEntrenchment(t *testing.T) {
	var f *Fact
	if f.EffectiveEntrenchment() != 0 {
		t.Error()
	}
	f = &Fact{Entrenchment: 0.5}
	if f.EffectiveEntrenchment() != 0.5 {
		t.Error()
	}
	f = &Fact{Metadata: map[string]interface{}{"entrenchment": "high"}}
	if f.EffectiveEntrenchment() != 0.95 {
		t.Error()
	}
	f = &Fact{Metadata: map[string]interface{}{"entrenchment": "medium"}}
	if f.EffectiveEntrenchment() != 0.75 {
		t.Error()
	}
}

func TestRequiresHumanReview(t *testing.T) {
	var f *Fact
	if f.RequiresHumanReview() {
		t.Error()
	}

	f = &Fact{Metadata: map[string]interface{}{"requires_human_review": true}, ReviewStatus: ReviewPending}
	if !f.RequiresHumanReview() {
		t.Error()
	}

	f.ReviewStatus = ReviewApproved
	if f.RequiresHumanReview() {
		t.Error()
	}

	f.Metadata["requires_human_review"] = false
	f.ReviewStatus = ReviewPending
	if !f.RequiresHumanReview() {
		t.Error("pending implies requires review unless explicit false? wait, if explicit false it falls back to checking if pending")
	}
}

func TestEngine_SetFactReview(t *testing.T) {
	e := NewEngine()
	if err := e.SetFactReview("unknown", ReviewApproved, "", 0); err == nil {
		t.Error()
	}

	e.AssertFact(&Fact{ID: "f1", IsRoot: true, ManualStatus: Valid})
	if err := e.SetFactReview("f1", "invalid", "", 0); err == nil {
		t.Error()
	}

	if err := e.SetFactReview("f1", ReviewApproved, "ok", time.Now().UnixMilli()); err != nil {
		t.Error(err)
	}

	f, _ := e.GetFact("f1")
	if f.ReviewStatus != ReviewApproved || f.ReviewReason != "ok" {
		t.Error()
	}

	// Idempotent
	if err := e.SetFactReview("f1", ReviewApproved, "ok", 0); err != nil {
		t.Error(err)
	}
}
