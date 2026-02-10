package main

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func main() {
	files, err := specutil.ChangedFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	for _, file := range files {
		fmt.Println(file)
	}
}
