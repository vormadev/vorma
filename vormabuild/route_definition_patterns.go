package vormabuild

import (
	"fmt"
	"strings"
)

func normalizeRouteDefinitionPatternsInInputOrder(
	routeDefinitionPatterns []string,
) ([]string, error) {
	normalizedPatterns := make([]string, 0, len(routeDefinitionPatterns))
	seenPatterns := make(map[string]struct{}, len(routeDefinitionPatterns))
	for index, routeDefinitionPattern := range routeDefinitionPatterns {
		trimmedRouteDefinitionPattern := strings.TrimSpace(
			routeDefinitionPattern,
		)
		if trimmedRouteDefinitionPattern == "" {
			return nil, fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d] cannot be empty or whitespace",
				index,
			)
		}
		if trimmedRouteDefinitionPattern != routeDefinitionPattern {
			return nil, fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d]=%q must not contain surrounding whitespace",
				index,
				routeDefinitionPattern,
			)
		}
		if _, hasSeenPattern := seenPatterns[trimmedRouteDefinitionPattern]; hasSeenPattern {
			return nil, fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d]=%q duplicates an earlier pattern",
				index,
				trimmedRouteDefinitionPattern,
			)
		}

		seenPatterns[trimmedRouteDefinitionPattern] = struct{}{}
		normalizedPatterns = append(
			normalizedPatterns,
			trimmedRouteDefinitionPattern,
		)
	}
	return normalizedPatterns, nil
}
