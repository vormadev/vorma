package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func main() {
	doc, err := specutil.ParseDispatchFile("spec/MINING_DISPATCH.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	failures := 0
	for _, row := range doc.Rows {
		if row.Status != "DONE" {
			continue
		}
		specFile := row.SpecPath + "/spec.json"
		if _, err := os.Stat(specFile); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] missing spec file: %s\n", row.SlotID, specFile)
			failures++
			continue
		}

		cmd := exec.Command("go", "run", "./spec/tools/cmd/validate_slot_spec",
			"--slot", row.SlotID,
			"--spec", specFile,
			"--owner", row.Owner,
			"--dispatch", "spec/MINING_DISPATCH.json",
			"--require-review-pass",
		)
		cmd.Env = append(os.Environ(), "GOCACHE=/tmp/go-build")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			failures++
		}
	}

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "requirement/evidence checks failed with %d issue(s)\n", failures)
		os.Exit(1)
	}
}
