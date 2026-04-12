package api

import (
	"fmt"
	"strings"

	"velarix/core"
)

const (
	maxFactMapKeys          = 50
	maxFactStringValueChars = 10000
	maxJustificationSets    = 100
	maxJustificationParents = 100
	maxLLMOutputChars       = 32000
	maxSessionContextChars  = 2000
)

func validatePassword(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}
	return nil
}

func validateExtractRequestSizes(llmOutput string, sessionContext string) error {
	if len([]rune(llmOutput)) > maxLLMOutputChars {
		return fmt.Errorf("llm_output exceeds %d characters", maxLLMOutputChars)
	}
	if len([]rune(sessionContext)) > maxSessionContextChars {
		return fmt.Errorf("session_context exceeds %d characters", maxSessionContextChars)
	}
	return nil
}

func validateFactInput(fact *core.Fact) error {
	if fact == nil {
		return fmt.Errorf("fact is required")
	}
	if err := validateFactMap("payload", fact.Payload); err != nil {
		return err
	}
	if err := validateFactMap("metadata", fact.Metadata); err != nil {
		return err
	}
	if len(fact.JustificationSets) > maxJustificationSets {
		return fmt.Errorf("justification_sets cannot exceed %d sets", maxJustificationSets)
	}
	for i, set := range fact.JustificationSets {
		if len(set) > maxJustificationParents {
			return fmt.Errorf("justification_sets[%d] cannot exceed %d parent IDs", i, maxJustificationParents)
		}
		for j, parentID := range set {
			if len([]rune(strings.TrimSpace(parentID))) > maxFactStringValueChars {
				return fmt.Errorf("justification_sets[%d][%d] exceeds %d characters", i, j, maxFactStringValueChars)
			}
		}
	}
	return nil
}

func validateFactMap(name string, values map[string]interface{}) error {
	if values == nil {
		return nil
	}
	if len(values) > maxFactMapKeys {
		return fmt.Errorf("%s cannot exceed %d keys", name, maxFactMapKeys)
	}
	for key, value := range values {
		if len([]rune(key)) > maxFactStringValueChars {
			return fmt.Errorf("%s key exceeds %d characters", name, maxFactStringValueChars)
		}
		if err := validateStringValueLength(value); err != nil {
			return fmt.Errorf("%s[%q] %w", name, key, err)
		}
	}
	return nil
}

func validateStringValueLength(value interface{}) error {
	switch typed := value.(type) {
	case string:
		if len([]rune(typed)) > maxFactStringValueChars {
			return fmt.Errorf("contains a string value longer than %d characters", maxFactStringValueChars)
		}
	case []interface{}:
		for _, item := range typed {
			if err := validateStringValueLength(item); err != nil {
				return err
			}
		}
	case map[string]interface{}:
		for _, item := range typed {
			if err := validateStringValueLength(item); err != nil {
				return err
			}
		}
	}
	return nil
}
