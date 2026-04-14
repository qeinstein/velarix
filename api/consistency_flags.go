package api

import (
	"os"
	"strings"
	"time"

	"velarix/core"
	"velarix/store"
)

func autoFlagReviewOnContradiction() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("VELARIX_AUTO_FLAG_REVIEW_ON_CONTRADICTION")))
	if raw == "" {
		return true
	}
	switch raw {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func (s *Server) flagFactsForReviewOnIssues(sessionID, orgID string, engine *core.Engine, issues []core.ConsistencyIssue, reason string) {
	if !autoFlagReviewOnContradiction() {
		return
	}
	if s == nil || engine == nil || len(issues) == 0 {
		return
	}
	now := time.Now().UnixMilli()
	seen := map[string]struct{}{}
	for _, issue := range issues {
		for _, factID := range issue.FactIDs {
			factID = strings.TrimSpace(factID)
			if factID == "" {
				continue
			}
			if _, ok := seen[factID]; ok {
				continue
			}
			seen[factID] = struct{}{}

			// Mark pending review in-engine.
			_ = engine.SetFactReview(factID, core.ReviewPending, reason, now)

			// Persist the review flag for replay/audit.
			entry := store.JournalEntry{
				Type:      store.EventReview,
				SessionID: sessionID,
				ActorID:   "system",
				FactID:    factID,
				Payload: map[string]interface{}{
					"status":      core.ReviewPending,
					"reason":      reason,
					"reviewed_at": now,
				},
				Timestamp: now,
			}
			_ = s.Store.Append(entry)
			_ = s.Store.AppendOrgActivity(orgID, entry)
		}
	}
}
