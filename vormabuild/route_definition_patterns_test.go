package vormabuild

import "testing"

func TestNormalizeRouteDefinitionPatternsInInputOrder(t *testing.T) {
	inputPatterns := []string{
		"",
		"  \t",
		" frontend/src/routes/core.vorma.routes.ts ",
		"frontend/src/routes/core.vorma.routes.ts",
		"\nfrontend/src/routes/extra.vorma.routes.ts\n",
		"frontend/src/routes/core.vorma.routes.ts",
	}

	gotPatterns := normalizeRouteDefinitionPatternsInInputOrder(inputPatterns)
	wantPatterns := []string{
		"frontend/src/routes/core.vorma.routes.ts",
		"frontend/src/routes/extra.vorma.routes.ts",
	}

	if len(gotPatterns) != len(wantPatterns) {
		t.Fatalf("len(gotPatterns) = %d, want %d (%#v)", len(gotPatterns), len(wantPatterns), gotPatterns)
	}
	for idx, wantPattern := range wantPatterns {
		if gotPatterns[idx] != wantPattern {
			t.Fatalf("gotPatterns[%d] = %q, want %q (%#v)", idx, gotPatterns[idx], wantPattern, gotPatterns)
		}
	}
}
