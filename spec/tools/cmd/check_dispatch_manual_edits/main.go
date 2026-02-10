package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func main() {
	path := "spec/MINING_DISPATCH.json"
	changed, err := specutil.IsGitFileChanged(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if !changed {
		return
	}

	headRaw, err := specutil.GitFileAtHEAD(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	currentDoc, err := specutil.ParseDispatchFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	headDoc, err := specutil.ParseDispatchJSON(headRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if len(headDoc.Rows) != len(currentDoc.Rows) {
		fmt.Fprintln(os.Stderr, "spec/MINING_DISPATCH.json must not change slot count")
		os.Exit(1)
	}

	for i := range headDoc.Rows {
		oldRow := headDoc.Rows[i]
		newRow := currentDoc.Rows[i]

		if oldRow.SlotID != newRow.SlotID ||
			oldRow.PriorityGroup != newRow.PriorityGroup ||
			oldRow.SpecPath != newRow.SpecPath ||
			oldRow.SourceRoots != newRow.SourceRoots {
			fmt.Fprintf(os.Stderr, "spec/MINING_DISPATCH.json static slot fields must not be manually edited: %s\n", oldRow.SlotID)
			os.Exit(1)
		}

		if oldRow.Status == newRow.Status {
			if oldRow.Owner != newRow.Owner ||
				oldRow.UpdatedUTC != newRow.UpdatedUTC ||
				oldRow.Notes != newRow.Notes ||
				oldRow.ClaimBranch != newRow.ClaimBranch ||
				oldRow.ClaimContext != newRow.ClaimContext {
				fmt.Fprintf(os.Stderr, "spec/MINING_DISPATCH.json metadata changed without status transition: %s\n", oldRow.SlotID)
				os.Exit(1)
			}
			continue
		}

		allowed := (oldRow.Status == "OPEN" && newRow.Status == "CLAIMED") ||
			(oldRow.Status == "CLAIMED" && newRow.Status == "DONE")
		if !allowed {
			fmt.Fprintf(os.Stderr, "spec/MINING_DISPATCH.json invalid manual status transition: %s (%s -> %s)\n", oldRow.SlotID, oldRow.Status, newRow.Status)
			os.Exit(1)
		}
	}
}
