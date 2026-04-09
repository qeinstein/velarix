package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"velarix/store"
)

type complianceExportEnvelope struct {
	GeneratedAt     int64                      `json:"generated_at"`
	Organization    *store.Organization        `json:"organization,omitempty"`
	Billing         *store.BillingSubscription `json:"billing,omitempty"`
	Users           []string                   `json:"users,omitempty"`
	Policies        []store.Policy             `json:"policies,omitempty"`
	Integrations    []store.Integration        `json:"integrations,omitempty"`
	Sessions        []store.OrgSessionMeta     `json:"sessions,omitempty"`
	Usage           map[string]uint64          `json:"usage,omitempty"`
	UsageBreakdown  *store.UsageBreakdown      `json:"usage_breakdown,omitempty"`
	RecentActivity  []store.JournalEntry       `json:"recent_activity,omitempty"`
	RecentAccessLog []store.AccessLogEntry     `json:"recent_access_logs,omitempty"`
}

func (s *Server) handleComplianceExport(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "ndjson" {
		http.Error(w, "unsupported export format (json|ndjson)", http.StatusBadRequest)
		return
	}

	limit := 500
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 5000 {
			limit = parsed
		}
	}

	org, err := s.Store.GetOrganization(orgID)
	if err != nil {
		http.Error(w, "org not found", http.StatusNotFound)
		return
	}
	billing, _ := s.Store.GetBilling(orgID)
	users, _ := s.Store.ListOrgUsers(orgID)
	policies, _ := s.Store.ListPolicies(orgID)
	integrations, _ := s.Store.ListIntegrations(orgID)
	usage, _ := s.Store.GetOrgUsage(orgID)
	usageBreakdown, _ := s.Store.GetOrgUsageBreakdown(orgID)

	sessions := make([]store.OrgSessionMeta, 0, limit)
	sessionCursor := ""
	for len(sessions) < limit {
		page, next, pageErr := s.Store.ListOrgSessions(orgID, sessionCursor, minInt(limit-len(sessions), 200))
		if pageErr != nil || len(page) == 0 {
			break
		}
		sessions = append(sessions, page...)
		if next == "" {
			break
		}
		sessionCursor = next
	}

	activity, _, _ := s.Store.ListOrgActivityPage(orgID, "", minInt(limit, 1000))
	accessLogs, _, _ := s.Store.ListAccessLogsPage(orgID, "", minInt(limit, 1000))

	export := complianceExportEnvelope{
		GeneratedAt:     time.Now().UnixMilli(),
		Organization:    org,
		Billing:         billing,
		Users:           users,
		Policies:        policies,
		Integrations:    integrations,
		Sessions:        sessions,
		Usage:           usage,
		UsageBreakdown:  usageBreakdown,
		RecentActivity:  activity,
		RecentAccessLog: accessLogs,
	}

	entry := store.JournalEntry{
		Type:      store.EventAdminAction,
		SessionID: "",
		ActorID:   getActorID(r),
		Payload: map[string]interface{}{
			"action": "compliance_export",
			"format": format,
			"limit":  limit,
		},
		Timestamp: time.Now().UnixMilli(),
	}
	_ = s.Store.AppendOrgActivity(orgID, entry)

	filename := fmt.Sprintf("velarix_compliance_%s_%d.%s", orgID, export.GeneratedAt, format)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if format == "ndjson" {
		w.Header().Set("Content-Type", "application/x-ndjson")
		records := []map[string]interface{}{
			{"kind": "organization", "value": export.Organization},
			{"kind": "billing", "value": export.Billing},
			{"kind": "users", "value": export.Users},
			{"kind": "policies", "value": export.Policies},
			{"kind": "integrations", "value": export.Integrations},
			{"kind": "sessions", "value": export.Sessions},
			{"kind": "usage", "value": export.Usage},
			{"kind": "usage_breakdown", "value": export.UsageBreakdown},
			{"kind": "recent_activity", "value": export.RecentActivity},
			{"kind": "recent_access_logs", "value": export.RecentAccessLog},
		}
		enc := json.NewEncoder(w)
		for _, record := range records {
			if err := enc.Encode(record); err != nil {
				return
			}
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, export)
}
