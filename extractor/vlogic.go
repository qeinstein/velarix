package extractor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	factRegex   = regexp.MustCompile(`(?i)^fact\s+([a-zA-Z0-9_-]+)\s*:\s*"(.*?)"(?:\s*\((.*?)\))?$`)
	deriveRegex = regexp.MustCompile(`(?i)^derive\s+([a-zA-Z0-9_-]+)\s*:\s*"(.*?)"(?:\s*\((.*?)\))?(?:.*)?$`)
	reqRegex    = regexp.MustCompile(`(?i)requires\s*\((.*?)\)`)
	rejRegex    = regexp.MustCompile(`(?i)rejects\s*\((.*?)\)`)
	confRegex   = regexp.MustCompile(`(?i)\bconfidence\s*:\s*([0-9.]+)\b`)
	kindRegex   = regexp.MustCompile(`(?i)\bassertion_kind\s*:\s*(empirical|uncertain|hypothetical|fictional)\b`)
)

func parseVLogicConfidence(args string) float64 {
	confidence := 0.9
	match := confRegex.FindStringSubmatch(args)
	if match == nil {
		return confidence
	}
	parsed, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return confidence
	}
	return parsed
}

func parseVLogicAssertionKind(args string) string {
	kind := "empirical"
	match := kindRegex.FindStringSubmatch(args)
	if match == nil {
		return kind
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func parseCommaSeparatedList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func validateUniqueID(id string, lineNumber int, seenIDs map[string]struct{}) error {
	if _, ok := seenIDs[id]; ok {
		return fmt.Errorf("line %d: duplicate ID '%s'", lineNumber, id)
	}
	seenIDs[id] = struct{}{}
	return nil
}

func parseVLogicFactLine(line string, lineNumber int, seenIDs map[string]struct{}) (ExtractedFact, bool, error) {
	match := factRegex.FindStringSubmatch(line)
	if match == nil {
		return ExtractedFact{}, false, nil
	}

	id := match[1]
	if err := validateUniqueID(id, lineNumber, seenIDs); err != nil {
		return ExtractedFact{}, true, err
	}

	claim := match[2]
	args := match[3]

	return ExtractedFact{
		ID:            id,
		Claim:         claim,
		IsRoot:        true,
		Confidence:    parseVLogicConfidence(args),
		AssertionKind: parseVLogicAssertionKind(args),
		SourceType:    "v-logic",
		Polarity:      "positive",
	}, true, nil
}

func parseVLogicDeriveLine(line string, lineNumber int, seenIDs map[string]struct{}) (ExtractedFact, bool, error) {
	match := deriveRegex.FindStringSubmatch(line)
	if match == nil {
		return ExtractedFact{}, false, nil
	}

	id := match[1]
	if err := validateUniqueID(id, lineNumber, seenIDs); err != nil {
		return ExtractedFact{}, true, err
	}

	claim := match[2]
	args := match[3]

	var dependsOn []string
	if reqMatch := reqRegex.FindStringSubmatch(line); reqMatch != nil {
		dependsOn = append(dependsOn, parseCommaSeparatedList(reqMatch[1])...)
	}
	if rejMatch := rejRegex.FindStringSubmatch(line); rejMatch != nil {
		for _, rej := range parseCommaSeparatedList(rejMatch[1]) {
			dependsOn = append(dependsOn, "!"+rej)
		}
	}

	return ExtractedFact{
		ID:            id,
		Claim:         claim,
		IsRoot:        false,
		Confidence:    0.9,
		DependsOn:     dependsOn,
		AssertionKind: parseVLogicAssertionKind(args),
		SourceType:    "v-logic",
		Polarity:      "positive",
	}, true, nil
}

// ParseVLogic parses a V-Logic DSL script into ExtractedFacts.
func ParseVLogic(script string) ([]ExtractedFact, error) {
	lines := strings.Split(script, "\n")
	var facts []ExtractedFact
	seenIDs := make(map[string]struct{})

	for i, line := range lines {
		lineNumber := i + 1
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if fact, ok, err := parseVLogicFactLine(line, lineNumber, seenIDs); ok {
			if err != nil {
				return nil, err
			}
			facts = append(facts, fact)
			continue
		}

		if fact, ok, err := parseVLogicDeriveLine(line, lineNumber, seenIDs); ok {
			if err != nil {
				return nil, err
			}
			facts = append(facts, fact)
			continue
		}

		return nil, fmt.Errorf("line %d: syntax error: %s", lineNumber, line)
	}

	return facts, nil
}
