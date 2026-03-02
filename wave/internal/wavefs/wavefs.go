// Package wavefs owns small filesystem helpers shared by Wave build/dev
// internals.
package wavefs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const directoryWriteMode = 0o755

// EnsureDirectoryForFile ensures the parent directory exists for filePath.
func EnsureDirectoryForFile(filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return errors.New("file path is empty")
	}
	parentDirectory := filepath.Dir(filePath)
	if mkdirError := os.MkdirAll(parentDirectory, directoryWriteMode); mkdirError != nil {
		return fmt.Errorf(
			"create directory for file %q: %w",
			filePath,
			mkdirError,
		)
	}
	return nil
}
