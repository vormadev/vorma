package vormabuild

import "fmt"

func generateBuildIDWithPrefix(
	prefix string,
	generateBuildIDSuffix func() (string, error),
) (string, error) {
	buildIDSuffix, err := generateBuildIDSuffix()
	if err != nil {
		return "", fmt.Errorf("generate build ID: %w", err)
	}

	return prefix + buildIDSuffix, nil
}
