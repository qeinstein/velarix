package store

type Decision struct {
	ID                 string                 `json:"decision_id"`
	SessionID          string                 `json:"session_id"`
	OrgID              string                 `json:"org_id"`
	FactID             string                 `json:"fact_id,omitempty"`
	DecisionType       string                 `json:"decision_type"`
	SubjectRef         string                 `json:"subject_ref"`
	TargetRef          string                 `json:"target_ref"`
	Status             string                 `json:"status"`
	ExecutionStatus    string                 `json:"execution_status"`
	RecommendedAction  string                 `json:"recommended_action,omitempty"`
	PolicyVersion      string                 `json:"policy_version,omitempty"`
	ExplanationSummary string                 `json:"explanation_summary,omitempty"`
	CreatedBy          string                 `json:"created_by"`
	CreatedAt          int64                  `json:"created_at"`
	UpdatedAt          int64                  `json:"updated_at"`
	LastCheckedAt      int64                  `json:"last_checked_at,omitempty"`
	ExecutedAt         int64                  `json:"executed_at,omitempty"`
	ExecutedBy         string                 `json:"executed_by,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

type DecisionDependency struct {
	DecisionID      string  `json:"decision_id"`
	SessionID       string  `json:"session_id"`
	FactID          string  `json:"fact_id"`
	DependencyType  string  `json:"dependency_type"`
	RequiredStatus  string  `json:"required_status"`
	CurrentStatus   float64 `json:"current_status,omitempty"`
	SourceRef       string  `json:"source_ref,omitempty"`
	PolicyVersion   string  `json:"policy_version,omitempty"`
	ExplanationHint string  `json:"explanation_hint,omitempty"`
}

type DecisionBlocker struct {
	FactID          string  `json:"fact_id"`
	DependencyType  string  `json:"dependency_type"`
	RequiredStatus  string  `json:"required_status"`
	CurrentStatus   float64 `json:"current_status"`
	ReasonCode      string  `json:"reason_code"`
	SourceRef       string  `json:"source_ref,omitempty"`
	PolicyVersion   string  `json:"policy_version,omitempty"`
	ExplanationHint string  `json:"explanation_hint,omitempty"`
}

type DecisionCheck struct {
	DecisionID          string               `json:"decision_id"`
	SessionID           string               `json:"session_id"`
	Executable          bool                 `json:"executable"`
	BlockedBy           []DecisionBlocker    `json:"blocked_by"`
	ReasonCodes         []string             `json:"reason_codes"`
	CheckedAt           int64                `json:"checked_at"`
	DecisionVersion     int64                `json:"decision_version,omitempty"`
	SessionVersion      int64                `json:"session_version,omitempty"`
	ExpiresAt           int64                `json:"expires_at,omitempty"`
	ExecutionToken      string               `json:"execution_token,omitempty"`
	ExplanationSummary  string               `json:"explanation_summary,omitempty"`
	DependencySnapshots []DecisionDependency `json:"dependency_snapshots,omitempty"`
}

type DecisionListFilter struct {
	Status     string
	SubjectRef string
	FromMs     int64
	ToMs       int64
	Limit      int
}
