package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ReasoningStep records one step in a persisted reasoning chain.
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

// ReasoningChain is a stored multi-step reasoning artifact.
type ReasoningChain struct {
	ChainID   string          `json:"chain_id"`
	Model     string          `json:"model,omitempty"`
	Mode      string          `json:"mode,omitempty"`
	Summary   string          `json:"summary,omitempty"`
	CreatedAt int64           `json:"created_at,omitempty"`
	Steps     []ReasoningStep `json:"steps"`
}

// ReasoningStepAudit records the audit result for one reasoning step.
type ReasoningStepAudit struct {
	StepID              string             `json:"step_id"`
	Valid               bool               `json:"valid"`
	MissingFactIDs      []string           `json:"missing_fact_ids,omitempty"`
	InvalidFactIDs      []string           `json:"invalid_fact_ids,omitempty"`
	OutputFactID        string             `json:"output_fact_id,omitempty"`
	ConsistencyFindings []ConsistencyIssue `json:"consistency_findings,omitempty"`
}

// ReasoningAuditReport summarises a reasoning-chain verification run.
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

// AuditReasoningChain checks a reasoning chain for stale, missing, or
// contradictory facts.
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
		stepAudit := ReasoningStepAudit{
			StepID:       step.ID,
			Valid:        true,
			OutputFactID: step.OutputFactID,
		}

		refIDs := uniqueIDs(step.EvidenceFactIDs, step.JustificationFactIDs, step.ContradictsFactIDs)
		for _, factID := range refIDs {
			fact, ok := e.Facts[factID]
			if !ok {
				stepAudit.Valid = false
				stepAudit.MissingFactIDs = append(stepAudit.MissingFactIDs, factID)
				continue
			}
			if e.effectiveStatusUnsafe(fact) < ConfidenceThreshold {
				stepAudit.Valid = false
				stepAudit.InvalidFactIDs = append(stepAudit.InvalidFactIDs, factID)
			}
		}

		if step.OutputFactID != "" {
			outputFact, ok := e.Facts[step.OutputFactID]
			if !ok {
				stepAudit.Valid = false
				stepAudit.MissingFactIDs = append(stepAudit.MissingFactIDs, step.OutputFactID)
			} else if e.effectiveStatusUnsafe(outputFact) < ConfidenceThreshold {
				stepAudit.Valid = false
				stepAudit.InvalidFactIDs = append(stepAudit.InvalidFactIDs, step.OutputFactID)
			}

			candidateIDs := uniqueSortedFactIDs(append([]string{step.OutputFactID}, priorOutputIDs...))
			issues := e.consistencyIssuesForIDsUnsafe(candidateIDs, false)
			if len(issues) > 0 {
				stepAudit.Valid = false
				stepAudit.ConsistencyFindings = append(stepAudit.ConsistencyFindings, issues...)
				report.Issues = append(report.Issues, issues...)
				for _, issue := range issues {
					for _, factID := range issue.FactIDs {
						if factID != step.OutputFactID {
							retractCandidates[factID] = struct{}{}
						}
					}
				}
			}

			for _, contradictedID := range step.ContradictsFactIDs {
				if _, ok := e.Facts[contradictedID]; ok {
					retractCandidates[contradictedID] = struct{}{}
				}
			}
			priorOutputIDs = append(priorOutputIDs, step.OutputFactID)
		}

		sort.Strings(stepAudit.MissingFactIDs)
		sort.Strings(stepAudit.InvalidFactIDs)
		if !stepAudit.Valid {
			report.Valid = false
		}
		report.StepAudits = append(report.StepAudits, stepAudit)
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
