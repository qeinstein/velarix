package models

type CreateDecisionRequest struct {
	DecisionID         string                 `json:"decision_id,omitempty"`
	FactID             string                 `json:"fact_id,omitempty"`
	DecisionType       string                 `json:"decision_type"`
	SubjectRef         string                 `json:"subject_ref"`
	TargetRef          string                 `json:"target_ref"`
	RecommendedAction  string                 `json:"recommended_action,omitempty"`
	PolicyVersion      string                 `json:"policy_version,omitempty"`
	ExplanationSummary string                 `json:"explanation_summary,omitempty"`
	DependencyFactIDs  []string               `json:"dependency_fact_ids,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

type RecomputeDecisionRequest struct {
	FactID            string   `json:"fact_id,omitempty"`
	DependencyFactIDs []string `json:"dependency_fact_ids,omitempty"`
}

type ExecuteDecisionRequest struct {
	ExecutionRef   string `json:"execution_ref,omitempty"`
	ExecutionToken string `json:"execution_token,omitempty"`
}
