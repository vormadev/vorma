package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func main() {
	dispatch, err := specutil.ParseDispatchFile("spec/MINING_DISPATCH.md")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	allOpen := len(dispatch.Rows) > 0
	for _, row := range dispatch.Rows {
		if row.Status != "OPEN" {
			allOpen = false
			break
		}
	}
	if allOpen {
		return
	}

	for _, file := range []string{"spec/DECISIONS.md", "spec/TRACEABILITY.md"} {
		changed, err := specutil.IsGitFileChanged(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		if !changed {
			continue
		}

		diff, err := specutil.GitDiff(file, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		for _, line := range strings.Split(diff, "\n") {
			if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "@@") || strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "index ") {
				continue
			}
			if strings.HasPrefix(line, "-") {
				fmt.Fprintf(os.Stderr, "%s is append-only during mining; deletions/modifications are prohibited\n", file)
				os.Exit(1)
			}
		}
	}
}
