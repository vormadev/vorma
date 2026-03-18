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

	repoconcat.MustConcat(OUTDIR+"WAVE_CURRENT_CODE.txt", []string{
		"wave",
		"!**/*.test.ts",
		"!**/*.bench.ts",
		"!**/*_test.go",
		"!**/bench.txt",
	})

	// repoconcat.MustConcat(OUTDIR+"VORMA_1.txt", []string{
	// 	"/vorma.go",
	// 	"/vormabuild/**/*",
	// 	"/internal/artifactio/**/*",
	// 	"/internal/vormapublicfilemap/**/*",
	// 	"/internal/vormaruntime/**/*",
	// 	"!**/*.test.ts",
	// 	"!**/*.bench.ts",
	// 	"!**/*_test.go",
	// 	"!**/bench.txt",
	// })

	repoconcat.MustConcat(OUTDIR+"VORMA_2.txt", []string{
		"/vorma2/**/*",
		"!**/*.test.ts",
		"!**/*.bench.ts",
		"!**/*_test.go",
		"!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"FULL_SYSTEM.txt", []string{
		"/wave/**/*",
		"/internal/pkg/prodcache/**/*",

		"/vorma2/**/*",
		"/typescript/vorma/**/*",

		"/typescript/kit/matcher/**/*",
		"/typescript/kit/url/**/*",

		"/kit/headels/**/*",
		"/kit/htmlutil/**/*",
		"/kit/matcher/**/*",
		"/kit/mux/**/*",
		"/kit/response/**/*",
		"/kit/tasks/**/*",
		"/lab/tsgen/**/*",
		"/kit/validate/**/*",
		"/lab/viteutil/**/*",

		"!**/*.test.ts",
		"!**/*.bench.ts",
		"!**/*_test.go",
		"!**/bench.txt",
	})
}
