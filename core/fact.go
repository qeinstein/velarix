package core

type Status float64

const (
	Invalid Status = 0.0
	Valid   Status = 1.0
)

type JustificationSet struct {
	ID                  string
	ChildFactID         string
	ParentFactIDs       []string
	TargetValidParents  int    // How many parents MUST be valid
	CurrentValidParents int    // How many are CURRENTLY valid above threshold
	Confidence          Status // Minimum confidence of its valid parents

	// Dominator Tree
	IDom string // Immediate Dominator (Fact ID)
}

type Fact struct {
	ID string

	// Arbitrary belief payload (validated against user schema)
	Payload map[string]interface{} `json:"payload,omitempty"`

	// Internal system metadata (ignored by schema validation)
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Root control
	IsRoot       bool
	ManualStatus Status

	// Computed only by the engine
	DerivedStatus           Status
	ValidJustificationCount int // How many JustificationSets are fully valid

	// OR-of-AND justification (for API/Journal backwards compatibility)
	JustificationSets [][]string `json:"justification_sets,omitempty"`

	// Schema Validation
	ValidationErrors []string `json:"validation_errors,omitempty"`

	// Dominator Tree
	IDom      string // Immediate Dominator (Fact ID)
	PreOrder  int
	PostOrder int
}
