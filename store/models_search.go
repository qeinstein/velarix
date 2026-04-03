package store

type SearchDocument struct {
	ID         string                 `json:"id"`
	OrgID      string                 `json:"org_id"`
	SessionID  string                 `json:"session_id,omitempty"`
	DocumentType string               `json:"document_type"`
	Title      string                 `json:"title,omitempty"`
	Body       string                 `json:"body,omitempty"`
	Status     string                 `json:"status,omitempty"`
	SubjectRef string                 `json:"subject_ref,omitempty"`
	TargetRef  string                 `json:"target_ref,omitempty"`
	FactID     string                 `json:"fact_id,omitempty"`
	DecisionID string                 `json:"decision_id,omitempty"`
	CreatedAt  int64                  `json:"created_at"`
	UpdatedAt  int64                  `json:"updated_at"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type SearchDocumentsFilter struct {
	Query        string
	DocumentType string
	Status       string
	SubjectRef   string
	FromMs       int64
	ToMs         int64
	Limit        int
	Cursor       string
}

type RetentionReport struct {
	ActivityDeleted      int `json:"activity_deleted"`
	AccessLogsDeleted    int `json:"access_logs_deleted"`
	NotificationsDeleted int `json:"notifications_deleted"`
}
