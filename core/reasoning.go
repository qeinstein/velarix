package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ReasoningStep is documented here.
type ReasoningStep struct {
	ID                   string   `json:"id"`
	Kind                 string   `json:"kind,omitempty"`
	Content              string   `json:"content"`
	EvidenceFactIDs      []string `json:"evidence_fact_ids,omitempty"`
	JustificationFactIDs []string `json:"justification_fact_ids,omitempty"`
	OutputFactID         string   `json:"output_fact_id,omitempty"`
	ContradictsFactIDs   []string `json:"contradicts_fact_ids,omitempty"`
	Confidence           float64  `json:"confidence,omitempty"`
}

// ReasoningChain is documented here.
type ReasoningChain struct {
	ChainID   string          `json:"chain_id"`
	Model     string          `json:"model,omitempty"`
	Mode      string          `json:"mode,omitempty"`
	Summary   string          `json:"summary,omitempty"`
	CreatedAt int64           `json:"created_at,omitempty"`
	Steps     []ReasoningStep `json:"steps"`
}

// ReasoningStepAudit is documented here.
type ReasoningStepAudit struct {
	StepID              string             `json:"step_id"`
	Valid               bool               `json:"valid"`
	MissingFactIDs      []string           `json:"missing_fact_ids,omitempty"`
	InvalidFactIDs      []string           `json:"invalid_fact_ids,omitempty"`
	OutputFactID        string             `json:"output_fact_id,omitempty"`
	ConsistencyFindings []ConsistencyIssue `json:"consistency_findings,omitempty"`
}

// ReasoningAuditReport is documented here.
type ReasoningAuditReport struct {
	ChainID                 string               `json:"chain_id"`
	Valid                   bool                 `json:"valid"`
	Summary                 string               `json:"summary"`
	StepAudits              []ReasoningStepAudit `json:"step_audits"`
	Issues                  []ConsistencyIssue   `json:"issues,omitempty"`
	RetractCandidateFactIDs []string             `json:"retract_candidate_fact_ids,omitempty"`
	AutoRetractedFactIDs    []string             `json:"auto_retracted_fact_ids,omitempty"`
	VerifiedAt              int64                `json:"verified_at"`
}

func uniqueIDs(values ...[]string) []string {
	merged := []string{}
	for _, group := range values {
		merged = append(merged, group...)
	}
	return uniqueSortedFactIDs(merged)
}

func (e *Engine) validateFactRefsUnsafe(factIDs []string) (missing []string, invalid []string) {
	for _, factID := range factIDs {
		fact, ok := e.Facts[factID]
		if !ok {
			missing = append(missing, factID)
			continue
		}
		if e.effectiveStatusUnsafe(fact) < ConfidenceThreshold {
			invalid = append(invalid, factID)
		}
	}
	return missing, invalid
}

func addRetractCandidatesFromIssues(issues []ConsistencyIssue, outputFactID string, retractCandidates map[string]struct{}) {
	for _, issue := range issues {
		for _, factID := range issue.FactIDs {
			if factID == outputFactID {
				continue
			}
			retractCandidates[factID] = struct{}{}
		}
	}
}

func (e *Engine) auditReasoningStepUnsafe(step ReasoningStep, priorOutputIDs []string, retractCandidates map[string]struct{}) ReasoningStepAudit {
	stepAudit := ReasoningStepAudit{
		StepID:       step.ID,
		Valid:        true,
		OutputFactID: step.OutputFactID,
	}

	refIDs := uniqueIDs(step.EvidenceFactIDs, step.JustificationFactIDs, step.ContradictsFactIDs)
	stepAudit.MissingFactIDs, stepAudit.InvalidFactIDs = e.validateFactRefsUnsafe(refIDs)
	if len(stepAudit.MissingFactIDs) > 0 || len(stepAudit.InvalidFactIDs) > 0 {
		stepAudit.Valid = false
	}

	if step.OutputFactID != "" {
		missing, invalid := e.validateFactRefsUnsafe([]string{step.OutputFactID})
		if len(missing) > 0 {
			stepAudit.Valid = false
			stepAudit.MissingFactIDs = append(stepAudit.MissingFactIDs, missing...)
		}
		if len(invalid) > 0 {
			stepAudit.Valid = false
			stepAudit.InvalidFactIDs = append(stepAudit.InvalidFactIDs, invalid...)
		}

		candidateIDs := append([]string{step.OutputFactID}, priorOutputIDs...)
		issues := e.consistencyIssuesForIDsUnsafe(candidateIDs, false)
		if len(issues) > 0 {
			stepAudit.Valid = false
			stepAudit.ConsistencyFindings = append(stepAudit.ConsistencyFindings, issues...)
			addRetractCandidatesFromIssues(issues, step.OutputFactID, retractCandidates)
		}

		for _, contradictedID := range step.ContradictsFactIDs {
			if _, ok := e.Facts[contradictedID]; ok {
				retractCandidates[contradictedID] = struct{}{}
			}
		}
	}

	sort.Strings(stepAudit.MissingFactIDs)
	sort.Strings(stepAudit.InvalidFactIDs)
	return stepAudit
}

func (e *Engine) AuditReasoningChain(chain *ReasoningChain) *ReasoningAuditReport {
	report := &ReasoningAuditReport{
		ChainID:    "",
		Valid:      true,
		Summary:    "reasoning chain passed verification",
		StepAudits: []ReasoningStepAudit{},
		Issues:     []ConsistencyIssue{},
		VerifiedAt: time.Now().UnixMilli(),
	}
	if chain == nil {
		report.Valid = false
		report.Summary = "reasoning chain is missing"
		return report
	}
	report.ChainID = chain.ChainID

	e.mu.RLock()
	defer e.mu.RUnlock()

	priorOutputIDs := []string{}
	retractCandidates := map[string]struct{}{}

	for _, step := range chain.Steps {
		stepAudit := e.auditReasoningStepUnsafe(step, priorOutputIDs, retractCandidates)
		if !stepAudit.Valid {
			report.Valid = false
		}
		report.StepAudits = append(report.StepAudits, stepAudit)
		if step.OutputFactID != "" {
			priorOutputIDs = append(priorOutputIDs, step.OutputFactID)
			report.Issues = append(report.Issues, stepAudit.ConsistencyFindings...)
		}
	}

	for factID := range retractCandidates {
		report.RetractCandidateFactIDs = append(report.RetractCandidateFactIDs, factID)
	}
	sort.Strings(report.RetractCandidateFactIDs)

	if !report.Valid {
		report.Summary = fmt.Sprintf("reasoning chain %s requires review; one or more steps reference stale, missing, or contradictory beliefs", strings.TrimSpace(chain.ChainID))
	}
	return report
}
