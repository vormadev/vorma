package main

import (
	"os"

	"github.com/vormadev/vorma/lab/repoconcat"
)

func main() {
	OUTDIR := "__LLM_CONCAT.local/"

	// create dir called __LLM_CONCAT.local if it doesn't exist
	if _, err := os.Stat(OUTDIR); os.IsNotExist(err) {
		os.Mkdir(OUTDIR, 0755)
	}

	// SNIPPET:
	//
	// repoconcat.MustConcat(repoconcat.Config{
	// 	Output:  "____________.txt",
	// 	Patterns: []string{},
	// 	Exclude: []string{
	// "!**/*.test.ts", "!**/*.bench.ts", "!**/*_test.go", "!**/bench.txt"},
	// })

	// repoconcat.MustConcat(repoconcat.Config{
	// 	Output: "WAVE_OLD_CODE_BEFORE_REFACTOR.txt",
	// 	Patterns: []string{
	// 		"__old__/wave",
	// 	},
	// 	Exclude: []string{
	// 		// "**/canonical_refactor/LOCKED_DECISIONS_AND_ARCHITECTURE.md",
	// 		"**/*.test.ts",
	// 		"**/*.bench.ts",
	// 		"**/*_test.go",
	// 		"**/bench.txt",
	// 	},
	// })

	repoconcat.MustConcat(OUTDIR+"WAVE_CURRENT_CODE.txt", []string{
		"wave",
		"!**/*.test.ts",
		"!**/*.bench.ts",
		"!**/*_test.go",
		"!**/bench.txt",
	})

	// repoconcat.MustConcat(repoconcat.Config{
	// 	Output: OUTDIR + "VORMA_OLD_CODE_BEFORE_REFACTOR.txt",
	// 	Patterns: []string{
	// 		"__old__/vormaclient",
	// 		"__old__/vorma.go",
	// 	},
	// 	Exclude: []string{
	// 		// "**/canonical_refactor/LOCKED_DECISIONS_AND_ARCHITECTURE.md",
	// 		"**/*.test.ts",
	// 		"**/*.bench.ts",
	// 		"**/*_test.go",
	// 		"**/bench.txt",
	// 	},
	// })

	repoconcat.MustConcat(OUTDIR+"VORMA_CURRENT_CODE.txt", []string{
		"internal/vormaruntime",
		"vormabuild",
		"vorma.go",
		"canonical_refactor/LOCKED_DECISIONS_AND_ARCHITECTURE.md",
		"vormaclient/vite/vite.ts",
		"!**/*.test.ts",
		"!**/*.bench.ts",
		"!**/*_test.go",
		"!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"VORMA_FRONTEND.txt", []string{
		"vormaclient/*",
		"!**/*.test.ts",
		"!**/*.bench.ts",
		"!**/*_test.go",
		"!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"BOOTSTRAPPER.txt", []string{
		"bootstrap", "!**/*.test.ts", "!**/*.bench.ts", "!**/*_test.go", "!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"TSGEN.txt", []string{
		"lab/tsgen", "!**/*.test.ts", "!**/*.bench.ts", "!**/*_test.go", "!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"MUX.txt", []string{
		"kit/mux", "!**/*.test.ts", "!**/*.bench.ts", "!**/*_test.go", "!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"VITE_UTIL.txt", []string{
		"lab/viteutil", "!**/*.test.ts", "!**/*.bench.ts", "!**/*_test.go", "!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"RESPONSE.txt", []string{
		"kit/response", "!**/*.test.ts", "!**/*.bench.ts", "!**/*_test.go", "!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"HEADELS.txt", []string{
		"kit/headels", "!**/*.test.ts", "!**/*.bench.ts", "!**/*_test.go", "!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"MATCHER.txt", []string{
		"kit/matcher", "**/*.test.ts", "**/*.bench.ts", "**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"TASKS.txt", []string{
		"kit/tasks", "!**/*.test.ts", "!**/*.bench.ts", "!**/*_test.go", "!**/bench.txt",
	})

	repoconcat.MustConcat(OUTDIR+"VALIDATE.txt", []string{
		"kit/validate", "!**/*.test.ts", "!**/*.bench.ts", "!**/*_test.go", "!**/bench.txt",
	})
}
