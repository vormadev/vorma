package main

import (
	"os"

	"github.com/vormadev/vorma/lab/repoconcat"
)

func main() {
	OUTDIR := "__LLM_CONCAT.local/"
	if _, err := os.Stat(OUTDIR); os.IsNotExist(err) {
		os.Mkdir(OUTDIR, 0755)
	}

	exclude := []string{
		"!typescript/vorma/black_box_dist_tests/**/*",
		"!typescript/vorma/client/vitest.dist.config.ts",
		"!kit/matcher/internal/matchercore/testutil/testutil.go",
		"!**/*.md",
		"!**/*.txt",
		"!**/*.test.ts",
		"!**/*.bench.ts",
		"!**/*_test.go",
		"!**/bench.txt",
	}

	repoconcat.MustConcat(OUTDIR+"WAVE_CURRENT_CODE.txt", []string{
		"wave",
		"!**/*.test.ts",
		"!**/*.bench.ts",
		"!**/*_test.go",
		"!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"VORMA_2.txt", append([]string{
		"vorma2/**/*",
	}, exclude...))

	// Full system (unchanged)
	repoconcat.MustConcat(OUTDIR+"FULL_SYSTEM.txt", append([]string{
		"wave/**/*",
		"internal/pkg/prodcache/**/*",
		"vorma2/**/*",
		"typescript/vorma/**/*",
		"typescript/kit/matcher/**/*",
		"typescript/kit/url/**/*",
		"kit/headels/**/*",
		"kit/htmlutil/**/*",
		"kit/matcher/**/*",
		"kit/mux/**/*",
		"kit/response/**/*",
		"kit/tasks/**/*",
		"lab/tsgen/**/*",
		"kit/validate/**/*",
		"lab/viteutil/**/*",
	}, exclude...))

	// Per-package breakdowns
	packages := []struct {
		name     string
		patterns []string
	}{
		{"wave", []string{"wave/**/*"}},
		{"vorma_go", []string{"vorma2/**/*"}},
		{"vorma_client_ts", []string{"typescript/vorma/client/**/*"}},
		{"vorma_create_ts", []string{"typescript/vorma/create/**/*"}},
		{
			"vorma_preact_ts",
			[]string{"typescript/vorma/ui-adapters/preact/**/*"},
		},
		{"vorma_react_ts", []string{"typescript/vorma/ui-adapters/react/**/*"}},
		{"vorma_solid_ts", []string{"typescript/vorma/ui-adapters/solid/**/*"}},
		{"vorma_vite_ts", []string{"typescript/vorma/vite/**/*"}},
		{"kit_matcher_ts", []string{"typescript/kit/matcher/**/*"}},
		{"kit_url_ts", []string{"typescript/kit/url/**/*"}},
		{"kit_headels_go", []string{"kit/headels/**/*"}},
		{"kit_htmlutil_go", []string{"kit/htmlutil/**/*"}},
		{"kit_matcher_go", []string{"kit/matcher/**/*"}},
		{"kit_mux_go", []string{"kit/mux/**/*"}},
		{"kit_response_go", []string{"kit/response/**/*"}},
		{"kit_tasks_go", []string{"kit/tasks/**/*"}},
		{"kit_tsgen_go", []string{"lab/tsgen/**/*"}},
		{"kit_validate_go", []string{"kit/validate/**/*"}},
		{"kit_viteutil_go", []string{"lab/viteutil/**/*"}},
		{"prodcache_go", []string{"internal/pkg/prodcache/**/*"}},
	}

	for _, pkg := range packages {
		repoconcat.MustConcat(
			OUTDIR+"PKG_"+pkg.name+".txt",
			append(pkg.patterns, exclude...),
		)
	}
}
