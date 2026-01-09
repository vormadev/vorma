package builder

import (
	"path/filepath"

	"github.com/vormadev/vorma/wave/internal/configschema"
)

// writeConfigSchema writes the JSON schema to the internal directory
func writeConfigSchema(internalDir string) error {
	return configschema.Write(filepath.Join(internalDir, "schema.json"))
}
