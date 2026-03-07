package wavebuild

import (
	"errors"
	"log/slog"
)

// BuildOptions configures one Wave2 build/dev run.
type BuildOptions struct {
	// ConfigPath is the exact config file path relative to the literal process
	// CWD.
	ConfigPath string

	// Logger is the optional build/dev logger used by Wave2.
	Logger *slog.Logger
}

// Build runs one Wave2 build/dev entrypoint.
func Build(opts BuildOptions) error {
	return errors.New("wavebuild: Build is not implemented")
}
