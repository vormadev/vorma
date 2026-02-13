package tooling

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/vormadev/vorma/lab/jsonschema"
)

func writeConfigSchema(b *Builder) error {
	schema := buildConfigSchemaRootEntry(b)

	jsonBytes, err := json.MarshalIndent(schema, "", "\t")
	if err != nil {
		return fmt.Errorf("configschema.Write: failed to marshal JSON schema: %w", err)
	}

	jsonBytes = append(jsonBytes, '\n')

	target := filepath.Join(b.cfg.Dist.Internal(), "schema.json")
	if err = os.WriteFile(target, jsonBytes, 0644); err != nil {
		return fmt.Errorf("configschema.Write: failed to write JSON schema: %w", err)
	}

	return nil
}

func buildConfigSchemaRootEntry(
	b *Builder,
) jsonschema.Entry {
	schema := jsonschema.Entry{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Type:        jsonschema.TypeObject,
		Description: "Wave configuration schema.",
		Required:    []string{"Core"},
		Properties: map[string]jsonschema.Entry{
			"Core":  coreSchema,
			"Vite":  viteSchema,
			"Watch": watchSchema,
		},
	}

	if len(b.cfg.FrameworkSchemaExtensions) == 0 {
		return schema
	}

	properties := schema.Properties.(map[string]jsonschema.Entry)
	maps.Copy(properties, b.cfg.FrameworkSchemaExtensions)
	return schema
}
