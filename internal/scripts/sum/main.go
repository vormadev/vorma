package main

import (
	"os"

	"github.com/vormadev/vorma/lab/repoconcat"
)

func main() {
	// create dir called __LLM_CONCAT.local if it doesn't exist
	if _, err := os.Stat("__LLM_CONCAT.local"); os.IsNotExist(err) {
		os.Mkdir("__LLM_CONCAT.local", 0755)
	}

	// SNIPPET:
	//
	// repoconcat.Concat(repoconcat.Config{
	// 	Output:  "__LLM_CONCAT.local/____________.txt",
	// 	Include: []string{},
	// 	Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	// })

	// repoconcat.Concat(repoconcat.Config{
	// 	Output: "__LLM_CONCAT.local/WAVE_OLD_CODE_BEFORE_REFACTOR.txt",
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
		Output: "__LLM_CONCAT.local/WAVE_CURRENT_CODE.txt",
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
	// 	Output: "__LLM_CONCAT.local/VORMA_OLD_CODE_BEFORE_REFACTOR.txt",
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
		Output: "__LLM_CONCAT.local/VORMA_CURRENT_CODE.txt",
		Include: []string{
			"fw",
			"vorma.go",
		},
		Exclude: []string{
			"**/*.test.ts",
			"**/*.bench.ts",
			"**/*_test.go",
			"**/bench.txt",
		},
	})

	// repoconcat.Concat(repoconcat.Config{
	// 	Output:  "__LLM_CONCAT.local/BOOTSTRAPPER.txt",
	// 	Include: []string{"bootstrap"},
	// 	Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	// })

	repoconcat.Concat(repoconcat.Config{
		Output:  "__LLM_CONCAT.local/TSGEN.txt",
		Include: []string{"lab/tsgen"},
		Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	})
}
