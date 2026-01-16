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
	// repoconcat.Concat(repoconcat.Config{
	// 	Output:  "____________.txt",
	// 	Include: []string{},
	// 	Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	// })

	// repoconcat.Concat(repoconcat.Config{
	// 	Output: "WAVE_OLD_CODE_BEFORE_REFACTOR.txt",
	// 	Include: []string{
	// 		"__old__/wave",
	// 	},
	// 	Exclude: []string{
	// 		// "**/ARCHITECTURE_DECISIONS.md",
	// 		"**/*.test.ts",
	// 		"**/*.bench.ts",
	// 		"**/*_test.go",
	// 		"**/bench.txt",
	// 	},
	// })

	repoconcat.Concat(repoconcat.Config{
		Output: OUTDIR + "WAVE_CURRENT_CODE.txt",
		Include: []string{
			"wave",
		},
		Exclude: []string{
			"**/*.test.ts",
			"**/*.bench.ts",
			"**/*_test.go",
			"**/bench.txt",
		},
	})

	// repoconcat.Concat(repoconcat.Config{
	// 	Output: OUTDIR + "VORMA_OLD_CODE_BEFORE_REFACTOR.txt",
	// 	Include: []string{
	// 		"__old__/internal/framework",
	// 		"__old__/vorma.go",
	// 	},
	// 	Exclude: []string{
	// 		// "**/ARCHITECTURE_DECISIONS.md",
	// 		"**/*.test.ts",
	// 		"**/*.bench.ts",
	// 		"**/*_test.go",
	// 		"**/bench.txt",
	// 	},
	// })

	repoconcat.Concat(repoconcat.Config{
		Output: OUTDIR + "VORMA_CURRENT_CODE.txt",
		Include: []string{
			"vormaruntime",
			"vormabuild",
			"vorma.go",
			"ARCHITECTURE_DECISIONS.md",
			"internal/framework/_typescript/vite/vite.ts",
		},
		Exclude: []string{
			"**/*.test.ts",
			"**/*.bench.ts",
			"**/*_test.go",
			"**/bench.txt",
		},
	})

	repoconcat.Concat(repoconcat.Config{
		Output: OUTDIR + "VORMA_FRONTEND.txt",
		Include: []string{
			"internal/framework/_typescript/*",
		},
		Exclude: []string{
			"**/*.test.ts",
			"**/*.bench.ts",
			"**/*_test.go",
			"**/bench.txt",
		},
	})

	repoconcat.Concat(repoconcat.Config{
		Output:  OUTDIR + "BOOTSTRAPPER.txt",
		Include: []string{"bootstrap"},
		Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	})

	repoconcat.Concat(repoconcat.Config{
		Output:  OUTDIR + "TSGEN.txt",
		Include: []string{"lab/tsgen"},
		Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	})

	repoconcat.Concat(repoconcat.Config{
		Output:  OUTDIR + "MUX.txt",
		Include: []string{"kit/mux"},
		Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	})

	repoconcat.Concat(repoconcat.Config{
		Output:  OUTDIR + "VITE_UTIL.txt",
		Include: []string{"lab/viteutil"},
		Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	})
}
