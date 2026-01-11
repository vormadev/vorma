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

	repoconcat.Concat(repoconcat.Config{
		Output: "__LLM_CONCAT.local/__WAVE_AND_VORMA.txt",
		Include: []string{
			"wave",
			"internal/framework",
		},
		Exclude: []string{
			"wave/internal/configschema",
			"**/_typescript",
			"**/*.test.ts",
			"**/*.bench.ts",
			"**/*_test.go",
			"**/bench.txt",
		},
	})

	repoconcat.Concat(repoconcat.Config{
		Output:  "__LLM_CONCAT.local/BOOTSTRAPPER.txt",
		Include: []string{"bootstrap"},
		Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	})

	repoconcat.Concat(repoconcat.Config{
		Output:  "__LLM_CONCAT.local/TSGEN.txt",
		Include: []string{"lab/tsgen"},
		Exclude: []string{"**/*.test.ts", "**/*.bench.ts", "**/*_test.go", "**/bench.txt"},
	})
}
