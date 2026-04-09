package api

import (
	"strings"
	"time"

	"velarix/store"
)

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
