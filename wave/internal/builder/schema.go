package builder

import (
	"path/filepath"

	"github.com/vormadev/vorma/wave/internal/configschema"
)

// writeConfigSchema writes the JSON schema to the internal directory
func writeConfigSchema(b *Builder) error {
	return configschema.Write(
		filepath.Join(b.cfg.Dist.Internal(), "schema.json"),
		b.cfg.FrameworkSchemaExtensions,
	)
}
