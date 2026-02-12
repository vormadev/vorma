package vormabuild

import (
	"slices"
	"testing"
)

func TestVormaSchema_RequiredChildren(t *testing.T) {
	expectedRequiredChildren := []string{
		"UIVariant",
		"HTMLTemplateLocation",
		"ClientEntry",
		"ClientRouteDefinitionPatterns",
		"TSGenOutDir",
		"MainBuildEntry",
	}

	if VormaSchema.Type != "object" {
		t.Fatalf("VormaSchema.Type = %q, want %q", VormaSchema.Type, "object")
	}
	if !slices.Equal(VormaSchema.Required, expectedRequiredChildren) {
		t.Fatalf("VormaSchema.Required = %#v, want %#v", VormaSchema.Required, expectedRequiredChildren)
	}
}

func TestVormaSchema_FieldDefaultsAndEnums(t *testing.T) {
	if IncludeDefaultsSchema.Default != true {
		t.Fatalf("IncludeDefaultsSchema.Default = %#v, want true", IncludeDefaultsSchema.Default)
	}
	if BuildtimePublicURLFuncNameSchema.Default != "waveBuildtimeURL" {
		t.Fatalf(
			"BuildtimePublicURLFuncNameSchema.Default = %#v, want %q",
			BuildtimePublicURLFuncNameSchema.Default,
			"waveBuildtimeURL",
		)
	}

	expectedUIVariants := []string{"react", "preact", "solid"}
	if !slices.Equal(UIVariantSchema.Enum, expectedUIVariants) {
		t.Fatalf("UIVariantSchema.Enum = %#v, want %#v", UIVariantSchema.Enum, expectedUIVariants)
	}
}
