package extractor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	factRegex   = regexp.MustCompile(`(?i)^fact\s+([a-zA-Z0-9_-]+)\s*:\s*"(.*?)"(?:\s*\(confidence:\s*([0-9.]+)\))?$`)
	deriveRegex = regexp.MustCompile(`(?i)^derive\s+([a-zA-Z0-9_-]+)\s*:\s*"(.*?)"(?:.*)?$`)
	reqRegex    = regexp.MustCompile(`(?i)requires\s*\((.*?)\)`)
	rejRegex    = regexp.MustCompile(`(?i)rejects\s*\((.*?)\)`)
)

// ParseVLogic parses a V-Logic DSL script into ExtractedFacts.
func ParseVLogic(script string) ([]ExtractedFact, error) {
	lines := strings.Split(script, "\n")
	var facts []ExtractedFact
	seenIDs := make(map[string]bool)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if match := factRegex.FindStringSubmatch(line); match != nil {
			id := match[1]
			claim := match[2]
			confStr := match[3]
			
			if seenIDs[id] {
				return nil, fmt.Errorf("line %d: duplicate ID '%s'", i+1, id)
			}
			seenIDs[id] = true

			conf := 0.9
			if confStr != "" {
				if c, err := strconv.ParseFloat(confStr, 64); err == nil {
					conf = c
				}
			}

			facts = append(facts, ExtractedFact{
				ID:         id,
				Claim:      claim,
				IsRoot:     true,
				Confidence: conf,
				SourceType: "v-logic",
				Polarity:   "positive",
			})
			continue
		}

		if match := deriveRegex.FindStringSubmatch(line); match != nil {
			id := match[1]
			claim := match[2]

			if seenIDs[id] {
				return nil, fmt.Errorf("line %d: duplicate ID '%s'", i+1, id)
			}
			seenIDs[id] = true

			var dependsOn []string
			if reqMatch := reqRegex.FindStringSubmatch(line); reqMatch != nil {
				reqs := strings.Split(reqMatch[1], ",")
				for _, req := range reqs {
					req = strings.TrimSpace(req)
					if req != "" {
						dependsOn = append(dependsOn, req)
					}
				}
			}

			if rejMatch := rejRegex.FindStringSubmatch(line); rejMatch != nil {
				rejs := strings.Split(rejMatch[1], ",")
				for _, rej := range rejs {
					rej = strings.TrimSpace(rej)
					if rej != "" {
						dependsOn = append(dependsOn, "!"+rej)
					}
				}
			}

			facts = append(facts, ExtractedFact{
				ID:         id,
				Claim:      claim,
				IsRoot:     false,
				Confidence: 0.9,
				DependsOn:  dependsOn,
				SourceType: "v-logic",
				Polarity:   "positive",
			})
			continue
		}

		return nil, fmt.Errorf("line %d: syntax error: %s", i+1, line)
	}

	return facts, nil
}
