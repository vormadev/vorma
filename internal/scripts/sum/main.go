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

	repoconcat.Concat(repoconcat.Config{})
}
