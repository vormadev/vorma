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

	repoconcat.Concat(repoconcat.Config{
		Output:  "__LLM_CONCAT.local/__CURRENT_VERSION.txt",
		Include: []string{"wave", "internal/framework", "kit/mux/nested_mux.go", "bootstrap"},
		Exclude: []string{"wave/internal/configschema", "**/_typescript", "**/*_test.go", "**/bench.txt"},
	})
}
