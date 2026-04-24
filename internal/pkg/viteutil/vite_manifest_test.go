package viteutil

import (
	"reflect"
	"testing"
)

func TestViteManifestFindAllDepsPreservesManifestOrder(t *testing.T) {
	manifest := ViteManifest{
		"entry.ts": {
			File:    "entry.js",
			CSS:     []string{"entry.css", "shared.css"},
			Imports: []string{"dep_a.ts", "dep_b.ts"},
		},
		"dep_a.ts": {
			File:    "dep-a.js",
			CSS:     []string{"dep-a.css", "shared.css"},
			Imports: []string{"dep_c.ts"},
		},
		"dep_b.ts": {
			File: "dep-b.js",
			CSS:  []string{"dep-b.css"},
		},
		"dep_c.ts": {
			File: "dep-c.js",
			CSS:  []string{"dep-c.css"},
		},
	}

	got := manifest.FindAllDeps("entry.ts")

	expected_modules := []string{
		"entry.js",
		"dep-a.js",
		"dep-c.js",
		"dep-b.js",
	}
	if !reflect.DeepEqual(got.Modules, expected_modules) {
		t.Fatalf("expected modules %v, got %v", expected_modules, got.Modules)
	}

	expected_css_bundles := []string{
		"entry.css",
		"shared.css",
		"dep-a.css",
		"dep-c.css",
		"dep-b.css",
	}
	if !reflect.DeepEqual(got.CSSBundles, expected_css_bundles) {
		t.Fatalf(
			"expected CSS bundles %v, got %v",
			expected_css_bundles,
			got.CSSBundles,
		)
	}
}
