package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

var allowedRow = regexp.MustCompile(`^[+-]SLOT-[0-9]{3}\t(OPEN|CLAIMED|DONE)\t`)

func main() {
	changed, err := specutil.IsGitFileChanged("spec/MINING_DISPATCH.md")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if !changed {
		return
	}

	diff, err := specutil.GitDiff("spec/MINING_DISPATCH.md", 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	bad := false
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "@@ ") || line == "@@" {
			continue
		}
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			if allowedRow.MatchString(line) {
				continue
			}
			fmt.Fprintln(os.Stderr, line)
			bad = true
		}
	}
	if bad {
		fmt.Fprintln(os.Stderr, "spec/MINING_DISPATCH.md must only change via slot row transitions (use claim/mark scripts).")
		os.Exit(1)
	}
}
