package api

import (
	"strings"
	"time"

	"velarix/core"
	"velarix/store"
)

func truncateSnippet(body string) string {
	body = strings.TrimSpace(body)
	if len(body) <= 160 {
		return body
	}
	return body[:160]
}

func searchResultFromDocument(doc store.SearchDocument) searchResult {
	return searchResult{
		Type:      doc.DocumentType,
		SessionID: doc.SessionID,
		FactID:    doc.FactID,
		Timestamp: doc.UpdatedAt,
		Snippet:   truncateSnippet(firstNonEmpty(doc.Title, doc.Body, doc.Status)),
		Payload:   doc.Metadata,
	}
}

func (s *Server) syncSessionSearchDocuments(orgID, sessionID string, engine *core.Engine, config *store.SessionConfig) {
	if engine == nil || orgID == "" || sessionID == "" {
		return
	}
	now := time.Now().UnixMilli()
	docs := []store.SearchDocument{sessionSearchDocument(orgID, sessionID, config, now)}
	for _, fact := range engine.ListFacts() {
		fact.ResolvedStatus = engine.GetStatus(fact.ID)
		docs = append(docs, decisionFactSearchDocuments(orgID, sessionID, fact, now)...)
	}
	_ = s.Store.UpsertSearchDocuments(docs)
}

func (s *Server) syncFactSearchDocument(orgID, sessionID string, config *store.SessionConfig, fact *core.Fact, status core.Status) {
	if orgID == "" || sessionID == "" || fact == nil {
		return
	}
	now := time.Now().UnixMilli()
	fact.ResolvedStatus = status
	docs := []store.SearchDocument{sessionSearchDocument(orgID, sessionID, config, now)}
	docs = append(docs, decisionFactSearchDocuments(orgID, sessionID, fact, now)...)
	_ = s.Store.UpsertSearchDocuments(docs)
}
