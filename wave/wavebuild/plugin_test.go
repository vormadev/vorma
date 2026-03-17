package wavebuild

import (
	"testing"

	"github.com/vormadev/vorma/lab/jsonschema"
)

func TestValidatePluginConfig_AllowsJSONKeyWithoutParseFunc(t *testing.T) {
	plugin_cfg, err := validate_plugin_config(PluginConfig{
		JSONKey: "TestPlugin",
	})
	if err != nil {
		t.Fatalf("validate_plugin_config: %v", err)
	}
	if plugin_cfg == nil {
		t.Fatal("validate_plugin_config returned nil config")
	}
	if plugin_cfg.parse_fn != nil {
		t.Fatal("validate_plugin_config unexpectedly populated parse_fn")
	}
}

func TestBuildSchema_SkipsPluginConfigWithoutSchemaEntry(t *testing.T) {
	schema := build_schema([]*validated_plugin_config{
		{json_key: "TestPlugin"},
	})
	props, ok := schema.Properties.(map[string]jsonschema.Entry)
	if !ok {
		t.Fatalf("schema.Properties has unexpected type %T", schema.Properties)
	}
	if _, exists := props["TestPlugin"]; exists {
		t.Fatal("build_schema unexpectedly emitted TestPlugin property")
	}
}

func TestBuildLifecycleHookEntrySchema_RequiresAtLeastOneWatchPattern(
	t *testing.T,
) {
	schema := build_lifecycle_hook_entry_schema()
	props, ok := schema.Properties.(map[string]jsonschema.Entry)
	if !ok {
		t.Fatalf("schema.Properties has unexpected type %T", schema.Properties)
	}
	if got := props["WatchIncludePatterns"].MinItems; got != 1 {
		t.Fatalf("WatchIncludePatterns MinItems = %d, want 1", got)
	}
}
