package core

type Status int

const (
	Invalid Status = iota
	Valid
)

type Fact struct{
	ID string

	// Arbitrary belief payload
	payload map[string]interface{}

	// Root control
	IsRoot	bool
	ManualStatus Status

	// Computed only by the engine
	DerivedStatus Status

	// OR-of_AND justification
	JustificationSets [][]string
	
}