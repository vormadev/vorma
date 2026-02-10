package main

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func main() {
	rows, err := specutil.ParseCatalogTSV("spec/PACKAGE_CATALOG.tsv")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	changedFiles, err := specutil.ChangedFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	paths, err := specutil.ChangedPackagePaths(changedFiles, rows)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	for _, p := range paths {
		fmt.Println(p)
	}
}
