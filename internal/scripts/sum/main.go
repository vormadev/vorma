package main

import (
	"os"

	"github.com/vormadev/vorma/kit/lab/repoconcat"
)

func main() {
	// create dir called __LLM_CONCAT.local if it doesn't exist
	if _, err := os.Stat("__LLM_CONCAT.local"); os.IsNotExist(err) {
		os.Mkdir("__LLM_CONCAT.local", 0755)
	}

	///////// WAVE
	repoconcat.Concat(repoconcat.Config{
		Output:      "__LLM_CONCAT.local/___LLM__WAVE.local.txt",
		IncludeDirs: []string{"wave"},
		IgnoreFiles: []string{
			"**/*_test.go",
		}, Verbose: true,
	})

	/////// VORMA
	repoconcat.Concat(repoconcat.Config{
		Output:       "__LLM_CONCAT.local/___LLM__VORMA.local.txt",
		IncludeFiles: []string{"internal/framework/*.go"},
		IgnoreFiles: []string{
			"**/*_test.go",
		}, Verbose: true,
	})

	///////// MUX
	repoconcat.Concat(repoconcat.Config{
		Output:       "__LLM_CONCAT.local/___LLM__MUX_GO.local.txt",
		IncludeFiles: []string{"kit/mux/*.go"},
		IgnoreFiles: []string{
			"**/*_test.go",
		}, Verbose: true,
	})

	///////// BOOSTRAP
	repoconcat.Concat(repoconcat.Config{
		Output:       "__LLM_CONCAT.local/___LLM__BOOTSTRAP_GO.local.txt",
		IncludeFiles: []string{"bootstrap/**/*"},
		IgnoreFiles: []string{
			"**/*_test.go",
		}, Verbose: true,
	})
}
