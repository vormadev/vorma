package main

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func main() {
	rows, err := specutil.ParseCatalogJSON("spec/PACKAGE_CATALOG.json")
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
	if len(paths) > 1 {
		fmt.Fprintln(os.Stderr, "multiple spec package paths changed in one patch (use one isolated worktree per agent):")
		for _, p := range paths {
			fmt.Fprintf(os.Stderr, " - %s\n", p)
		}
		os.Exit(1)
	}
}
