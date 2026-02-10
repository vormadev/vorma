package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func main() {
	phaseStatus, err := specutil.ParsePhaseStatusFile("spec/PHASE_STATUS.md")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	phase1Status := strings.TrimSpace(phaseStatus.Phase1Status)
	if phase1Status == "COMPLETE" {
		return
	}

	changedFiles, err := specutil.ChangedFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	violations := make([]string, 0)
	for _, file := range changedFiles {
		if !strings.HasPrefix(file, "spec/") {
			violations = append(violations, file)
		}
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "phase1_status=%s, edits outside spec/** are prohibited:\n", phase1Status)
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, " - %s\n", v)
		}
		os.Exit(1)
	}
}
