package vormabuild

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const buildArtifactFileMode fs.FileMode = 0o644

type pathsJSONWriteDependencies struct {
	makePathsOutputDirectory func(string, fs.FileMode) error
	writePathsJSON           func(string, []byte, fs.FileMode) error
}

func writePathsJSONBytesToOutputPath(
	outputPath string,
	pathsAsJSON []byte,
	pathsWriteDeps pathsJSONWriteDependencies,
	makeOutputDirectoryErrorContext string,
	writePathsErrorContext string,
) error {
	if err := pathsWriteDeps.makePathsOutputDirectory(filepath.Dir(outputPath), os.ModePerm); err != nil {
		return fmt.Errorf("%s: %w", makeOutputDirectoryErrorContext, err)
	}

	if err := pathsWriteDeps.writePathsJSON(outputPath, pathsAsJSON, buildArtifactFileMode); err != nil {
		return fmt.Errorf("%s: %w", writePathsErrorContext, err)
	}
	return nil
}
