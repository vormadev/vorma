package vormabuild

import "strings"

func normalizeRouteDefinitionPatternsInInputOrder(
	routeDefinitionPatterns []string,
) []string {
	normalizedPatterns := make([]string, 0, len(routeDefinitionPatterns))
	seenPatterns := make(map[string]struct{}, len(routeDefinitionPatterns))
	for _, routeDefinitionPattern := range routeDefinitionPatterns {
		trimmedRouteDefinitionPattern := strings.TrimSpace(routeDefinitionPattern)
		if trimmedRouteDefinitionPattern == "" {
			continue
		}
		if _, hasSeenPattern := seenPatterns[trimmedRouteDefinitionPattern]; hasSeenPattern {
			continue
		}

		seenPatterns[trimmedRouteDefinitionPattern] = struct{}{}
		normalizedPatterns = append(normalizedPatterns, trimmedRouteDefinitionPattern)
	}
	return normalizedPatterns
}
