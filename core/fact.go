package core

// Status is documented here.
type Status float64

const (
	Invalid Status = 0.0 // invalid if it's 0.0, the closer it is to 0.0, the lower it's validity
	Valid   Status = 1.0 // Valid if it's 1, the closer it is to 1, the higher it's validity
)

// JustificationSet is documented here.
type JustificationSet struct {
	ID                    string
	ChildFactID           string
	ParentFactIDs         []string
	PositiveParentFactIDs []string
	NegativeParentFactIDs []string
	TargetValidParents    int    // How many parents MUST be valid
	CurrentValidParents   int    // How many are CURRENTLY valid above threshold
	Confidence            Status // Minimum confidence of its valid parents

	// Dominator Tree
	IDom string // Immediate Dominator (Fact ID)
}

// AssertionKind classifies the epistemic nature of a fact. It affects how
// confidence tiers are labelled and whether the fact participates in
// cross-scope contradiction detection.
//
//   - ""  / "empirical"    – default; a factual claim about the real world
//   - "uncertain"          – logically consistent but epistemically hedged
//                            (e.g. "X is probably the CEO"); confidence is
//                            capped at "probable" regardless of numeric value
//   - "hypothetical"       – what-if / scoped reasoning; does not contradict
//                            empirical facts
//   - "fictional"          – narrative / creative content; does not contradict
//                            empirical or hypothetical facts
const (
	AssertionKindEmpirical   = "empirical"
	AssertionKindUncertain   = "uncertain"
	AssertionKindHypothetical = "hypothetical"
	AssertionKindFictional   = "fictional"
)

// Fact is documented here.
type Fact struct {
	ID string `json:"id"`

	// Arbitrary belief payload (validated against user schema)
	Payload map[string]interface{} `json:"payload,omitempty"`

	// Internal system metadata (ignored by schema validation)
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Optional semantic embedding used for retrieval and consistency scans.
	Embedding []float64 `json:"embedding,omitempty"`

	// Root control
	IsRoot       bool    `json:"is_root"`
	ManualStatus Status  `json:"manual_status"`
	Entrenchment float64 `json:"entrenchment,omitempty"`
	ReviewStatus string  `json:"review_status,omitempty"`
	ReviewReason string  `json:"review_reason,omitempty"`
	ReviewedAt   int64   `json:"reviewed_at,omitempty"`

	// Temporal decay: Unix millisecond timestamp after which this fact is
	// automatically treated as Invalid. Zero means the fact never expires.
	ValidUntil int64 `json:"valid_until,omitempty"`

	// Epistemic classification. See AssertionKind* constants.
	// Controls confidence-tier labelling and contradiction-scope matching.
	AssertionKind string `json:"assertion_kind,omitempty"`

	// Computed only by the engine
	DerivedStatus           Status `json:"derived_status"`
	ResolvedStatus          Status `json:"resolved_status"`
	ValidJustificationCount int    `json:"valid_justification_count"` // How many JustificationSets are fully valid

	// OR-of-AND justification (for API/Journal backwards compatibility)
	JustificationSets [][]string `json:"justification_sets,omitempty"`

	// Schema Validation
	ValidationErrors []string `json:"validation_errors,omitempty"`

	// Dominator Tree
	IDom      string `json:"idom,omitempty"`
	PreOrder  int    `json:"preOrder,omitempty"`
	PostOrder int    `json:"postOrder,omitempty"`
}
