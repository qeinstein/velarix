package api

import (
	"encoding/json"
	"fmt"
	"time"

	"velarix/core"
	"velarix/store"
)

type orgBackupEnvelope struct {
	GeneratedAt int64                      `json:"generated_at"`
	OrgID       string                     `json:"org_id"`
	Org         *store.Organization        `json:"org,omitempty"`
	Billing     *store.BillingSubscription `json:"billing,omitempty"`
	Users       []store.User               `json:"users,omitempty"`
	Sessions    []sessionBackup            `json:"sessions,omitempty"`
}

type sessionBackup struct {
	Meta         store.OrgSessionMeta      `json:"meta"`
	Config       *store.SessionConfig      `json:"config,omitempty"`
	Snapshot     *core.Snapshot            `json:"snapshot,omitempty"`
	History      []store.JournalEntry      `json:"history,omitempty"`
	Explanations []store.ExplanationRecord `json:"explanations,omitempty"`
}

func (s *Server) buildOrgBackup(orgID string) ([]byte, error) {
	if orgID == "" {
		return nil, fmt.Errorf("org id is required")
	}
	now := time.Now().UnixMilli()

	org, err := s.Store.GetOrganization(orgID)
	if err != nil {
		org = nil
	}
	billing, _ := s.Store.GetBilling(orgID)

	emails, _ := s.Store.ListOrgUsers(orgID)
	users := make([]store.User, 0, len(emails))
	for _, email := range emails {
		user, err := s.Store.GetUser(email)
		if err != nil || user == nil {
			continue
		}
		users = append(users, *user)
	}

	metas := []store.OrgSessionMeta{}
	cursor := ""
	for page := 0; page < 10_000; page++ {
		items, next, err := s.Store.ListOrgSessions(orgID, cursor, 200)
		if err != nil || len(items) == 0 {
			break
		}
		metas = append(metas, items...)
		if next == "" {
			break
		}
		cursor = next
	}

	sessions := make([]sessionBackup, 0, len(metas))
	for i := range metas {
		meta := metas[i]
		cfg, _ := s.Store.GetConfig(meta.ID)
		snap, _ := s.Store.GetLatestSnapshot(meta.ID)
		history, _ := s.Store.GetSessionHistory(meta.ID)
		explanations, _ := s.Store.GetSessionExplanations(meta.ID)
		sessions = append(sessions, sessionBackup{
			Meta:         meta,
			Config:       cfg,
			Snapshot:     snap,
			History:      history,
			Explanations: explanations,
		})
	}

	env := orgBackupEnvelope{
		GeneratedAt: now,
		OrgID:       orgID,
		Org:         org,
		Billing:     billing,
		Users:       users,
		Sessions:    sessions,
	}
	return json.Marshal(env)
}
