package wavebuild

import (
	"errors"
	"log/slog"

	cfg_pkg "github.com/vormadev/vorma/wave2/internal/cfg"
)

/* INVARIANTS:

`BuildOptions.ConfigPath` must be relative to the user's literal process CWD
(so we have pure/simple determinism about where the config file is).

*/

// BuildOptions configures one Wave2 build/dev run.
type BuildOptions struct {
	// ConfigFile is the exact config file path relative to the literal process
	// CWD.
	ConfigFile string

	// Logger is the optional build/dev logger used by Wave2.
	Logger *slog.Logger
}

// Build runs one Wave2 build/dev entrypoint.
func Build(opts BuildOptions) error {
	raw_cfg := cfg_pkg.ConfigPathToRaw(opts.ConfigFile)
	parsed_cfg := cfg_pkg.RawToParsed(raw_cfg)
	_ = parsed_cfg
	return errors.New("wavebuild: Build is not implemented")
}
