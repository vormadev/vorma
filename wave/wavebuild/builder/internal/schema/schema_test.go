package schema_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavebuild/builder/internal/schema"
)

func newParsedConfigForSchemaPackageTestsAtRoot(
	root string,
) *wave.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(root)
}

func TestWriteSchema_WritesSchemaJSON(t *testing.T) {
	cfg := newParsedConfigForSchemaPackageTestsAtRoot(t.TempDir())
	processor := schema.NewProcessor(cfg, nil)

	if writeError := processor.WriteSchema(); writeError != nil {
		t.Fatalf("WriteSchema returned error: %v", writeError)
	}

	schemaPath := filepath.Join(cfg.Dist.Internal(), "schema.json")
	schemaBytes, readError := os.ReadFile(schemaPath)
	if readError != nil {
		t.Fatalf("read schema file %q: %v", schemaPath, readError)
	}

	var schemaDocument struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if unmarshalError := json.Unmarshal(schemaBytes, &schemaDocument); unmarshalError != nil {
		t.Fatalf("unmarshal schema: %v", unmarshalError)
	}

	for _, requiredKey := range []string{"$schema", "Core", "Watch", "Vite"} {
		if _, exists := schemaDocument.Properties[requiredKey]; !exists {
			t.Fatalf(
				"schema missing required top-level section %q",
				requiredKey,
			)
		}
	}
}

func TestBuildSchemaDocument_IncludesFrameworkExtensions(t *testing.T) {
	cfg := newParsedConfigForSchemaPackageTestsAtRoot(t.TempDir())
	cfg.FrameworkSchemaExtensions = map[string]jsonschema.Entry{
		"CustomFramework": jsonschema.OptionalObject(jsonschema.Def{
			Properties: struct {
				Enabled jsonschema.Entry
			}{
				Enabled: jsonschema.OptionalBoolean(
					jsonschema.Def{Default: true},
				),
			},
		}),
	}
	processor := schema.NewProcessor(cfg, nil)

	schemaDocument, buildError := processor.BuildSchemaDocument()
	if buildError != nil {
		t.Fatalf("BuildSchemaDocument returned error: %v", buildError)
	}

	properties, ok := schemaDocument["properties"].(map[string]any)
	if !ok {
		t.Fatalf(
			"schema properties has unexpected type: %#v",
			schemaDocument["properties"],
		)
	}
	if _, exists := properties["CustomFramework"]; !exists {
		t.Fatalf(
			"expected framework schema extension to be included, got keys: %#v",
			properties,
		)
	}
}

func TestWriteSchema_NilConfigReturnsError(t *testing.T) {
	var nilProcessor *schema.Processor
	if writeError := nilProcessor.WriteSchema(); writeError == nil {
		t.Fatal("expected nil processor WriteSchema to fail")
	}
}

func TestBuildSchemaDocument_WatchSchemaIncludesHookTimeoutSections(
	t *testing.T,
) {
	cfg := newParsedConfigForSchemaPackageTestsAtRoot(t.TempDir())
	processor := schema.NewProcessor(cfg, nil)

	schemaDocument, buildError := processor.BuildSchemaDocument()
	if buildError != nil {
		t.Fatalf("BuildSchemaDocument returned error: %v", buildError)
	}

	properties, ok := schemaDocument["properties"].(map[string]any)
	if !ok {
		t.Fatalf(
			"schema properties has unexpected type: %#v",
			schemaDocument["properties"],
		)
	}
	watchSchema, ok := properties["Watch"].(map[string]any)
	if !ok {
		t.Fatalf("Watch schema has unexpected type: %#v", properties["Watch"])
	}
	watchProperties, ok := watchSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf(
			"Watch.properties has unexpected type: %#v",
			watchSchema["properties"],
		)
	}

	if _, exists := watchProperties["HookCommandTimeouts"]; !exists {
		t.Fatal("expected Watch.HookCommandTimeouts schema section")
	}
	if _, exists := watchProperties["HookCallbackTimeouts"]; !exists {
		t.Fatal("expected Watch.HookCallbackTimeouts schema section")
	}
}

func TestBuildSchemaDocument_HookTimingDescriptionUsesConcurrentDashNoWait(
	t *testing.T,
) {
	cfg := newParsedConfigForSchemaPackageTestsAtRoot(t.TempDir())
	processor := schema.NewProcessor(cfg, nil)

	schemaDocument, buildError := processor.BuildSchemaDocument()
	if buildError != nil {
		t.Fatalf("BuildSchemaDocument returned error: %v", buildError)
	}

	properties := schemaDocument["properties"].(map[string]any)
	watchSchema := properties["Watch"].(map[string]any)
	watchProperties := watchSchema["properties"].(map[string]any)
	includeSchema := watchProperties["Include"].(map[string]any)
	watchedFileSchema := includeSchema["items"].(map[string]any)
	watchedFileProperties := watchedFileSchema["properties"].(map[string]any)
	hooksSchema := watchedFileProperties["OnChangeHooks"].(map[string]any)
	hookSchema := hooksSchema["items"].(map[string]any)
	hookProperties := hookSchema["properties"].(map[string]any)
	timingSchema := hookProperties["Timing"].(map[string]any)
	timingDescription := timingSchema["description"].(string)

	if !strings.Contains(timingDescription, "concurrent-no-wait") {
		t.Fatalf(
			"expected timing description to mention concurrent-no-wait, got %q",
			timingDescription,
		)
	}
	if strings.Contains(timingDescription, "concurrent_no_wait") {
		t.Fatalf(
			"expected timing description to avoid underscore variant, got %q",
			timingDescription,
		)
	}
}

func TestBuildSchemaDocument_AllWaveOwnedFieldsHaveDescriptions(t *testing.T) {
	cfg := newParsedConfigForSchemaPackageTestsAtRoot(t.TempDir())
	processor := schema.NewProcessor(cfg, nil)

	schemaDocument, buildError := processor.BuildSchemaDocument()
	if buildError != nil {
		t.Fatalf("BuildSchemaDocument returned error: %v", buildError)
	}

	properties, ok := schemaDocument["properties"].(map[string]any)
	if !ok {
		t.Fatalf(
			"schema properties has unexpected type: %#v",
			schemaDocument["properties"],
		)
	}

	for _, rootFieldName := range []string{"$schema", "Core", "Watch", "Vite"} {
		rootFieldSchema, fieldExists := properties[rootFieldName].(map[string]any)
		if !fieldExists {
			t.Fatalf("schema missing root field %q", rootFieldName)
		}
		rootFieldDescription, _ := rootFieldSchema["description"].(string)
		if strings.TrimSpace(rootFieldDescription) == "" {
			t.Fatalf("schema field %q is missing description", rootFieldName)
		}
		assertSchemaPropertyDescriptionsForSchemaPackageTests(
			t,
			rootFieldSchema,
			rootFieldName,
		)
	}
}

func assertSchemaPropertyDescriptionsForSchemaPackageTests(
	t *testing.T,
	schemaNode map[string]any,
	path string,
) {
	t.Helper()

	propertiesValue, hasProperties := schemaNode["properties"]
	if hasProperties {
		propertiesMap, ok := propertiesValue.(map[string]any)
		if !ok {
			t.Fatalf(
				"schema properties at %q have unexpected type: %#v",
				path,
				propertiesValue,
			)
		}
		for propertyName, propertyValue := range propertiesMap {
			propertySchema, ok := propertyValue.(map[string]any)
			if !ok {
				t.Fatalf(
					"schema property at %q.%s has unexpected type: %#v",
					path,
					propertyName,
					propertyValue,
				)
			}
			description, _ := propertySchema["description"].(string)
			if strings.TrimSpace(description) == "" {
				t.Fatalf(
					"schema property %q.%s is missing description",
					path,
					propertyName,
				)
			}
			assertSchemaPropertyDescriptionsForSchemaPackageTests(
				t,
				propertySchema,
				path+"."+propertyName,
			)
		}
	}

	itemsValue, hasItems := schemaNode["items"]
	if hasItems {
		itemsSchema, ok := itemsValue.(map[string]any)
		if !ok {
			t.Fatalf(
				"schema items at %q have unexpected type: %#v",
				path,
				itemsValue,
			)
		}
		assertSchemaPropertyDescriptionsForSchemaPackageTests(
			t,
			itemsSchema,
			path+"[]",
		)
	}
}
