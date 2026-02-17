package tooling

import (
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func validateNamedGlobPatternInput(
	validationContextLabel string,
	fieldPath string,
	globPattern string,
) error {
	if strings.TrimSpace(globPattern) == "" {
		return fmt.Errorf("%s: %s is required", validationContextLabel, fieldPath)
	}
	if strings.TrimSpace(globPattern) != globPattern {
		return fmt.Errorf(
			"%s: %s must not include surrounding whitespace",
			validationContextLabel,
			fieldPath,
		)
	}

	normalizedPattern := strings.ReplaceAll(globPattern, "\\", "/")
	if !doublestar.ValidatePattern(normalizedPattern) {
		return fmt.Errorf(
			"%s: %s must be a valid glob pattern; got %q",
			validationContextLabel,
			fieldPath,
			globPattern,
		)
	}

	return nil
}
