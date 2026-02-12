package tooling

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/vormadev/vorma/lab/jsonschema"
)

func TestWriteConfigSchema_IncludesFrameworkSchemaExtensions(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.FrameworkSchemaExtensions = map[string]jsonschema.Entry{
		"FrameworkSection": jsonschema.RequiredString(jsonschema.Def{
			Description: "framework schema extension",
		}),
	}

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	if err := writeConfigSchema(builder); err != nil {
		t.Fatalf("writeConfigSchema returned error: %v", err)
	}

	data, err := os.ReadFile(cfg.Dist.Internal() + "/schema.json")
	if err != nil {
		t.Fatalf("failed reading schema file: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed unmarshalling schema JSON: %v", err)
	}

	propertiesRaw, ok := parsed["properties"]
	if !ok {
		t.Fatalf("schema JSON missing properties field: %#v", parsed)
	}
	properties, ok := propertiesRaw.(map[string]any)
	if !ok {
		t.Fatalf("schema properties has unexpected type: %T", propertiesRaw)
	}

	if _, found := properties["FrameworkSection"]; !found {
		t.Fatalf("expected framework schema extension in properties, got keys: %#v", properties)
	}
}
