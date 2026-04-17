package api

import (
	"strings"
	"time"

	"velarix/store"
)

// planLimits returns (maxSessions, maxFactsPerSession) for the org's plan.
// Zero means "no limit enforced".
func planLimits(sub *store.BillingSubscription) (maxSessions, maxFactsPerSession int) {
	plan := "free"
	if sub != nil {
		plan = strings.ToLower(strings.TrimSpace(sub.Plan))
	}
	switch plan {
	case "pro":
		return 500, 0
	case "enterprise":
		return 0, 0 // unlimited
	default: // free / trial
		return 50, 0
	}
}

func effectiveRateLimitConfig(org *store.Organization, sub *store.BillingSubscription) (int, time.Duration) {
	limit := 60
	window := time.Minute

	if org != nil && org.Settings != nil {
		if raw, ok := org.Settings["rate_limit_rpm"]; ok {
			if parsed, ok := asInt(raw); ok && parsed > 0 && parsed <= 100000 {
				limit = parsed
			}
		}
		if raw, ok := org.Settings["rate_limit_window_seconds"]; ok {
			if parsed, ok := asInt(raw); ok && parsed > 0 && parsed <= 3600 {
				window = time.Duration(parsed) * time.Second
			}
		}
	}

	if sub != nil {
		switch strings.ToLower(strings.TrimSpace(sub.Plan)) {
		case "pro":
			if limit < 180 {
				limit = 180
			}
		case "enterprise":
			if limit < 600 {
				limit = 600
			}
		}
	}

	return limit, window
}
