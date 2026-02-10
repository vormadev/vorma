package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: go run ./spec/tools/cmd/mark_slot_done SLOT-XXX [owner]")
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	if len(args) < 1 || len(args) > 2 {
		usage()
	}

	slotID := args[0]
	actorOwner := "unknown"
	if len(args) == 2 {
		actorOwner = args[1]
	} else if os.Getenv("USER") != "" {
		actorOwner = os.Getenv("USER")
	}

	err := specutil.WithLock("spec/.dispatch.lock", func() error {
		doc, err := specutil.ParseDispatchFile("spec/MINING_DISPATCH.json")
		if err != nil {
			return err
		}

		idx, row := specutil.FindRowBySlotID(doc.Rows, slotID)
		if row == nil {
			return fmt.Errorf("slot not found: %s", slotID)
		}
		if row.Status != "CLAIMED" && row.Status != "DONE" {
			return fmt.Errorf("slot must be CLAIMED or DONE before marking done: %s (%s)", slotID, row.Status)
		}
		if row.Owner == "-" {
			return fmt.Errorf("slot owner is invalid for %s", slotID)
		}
		if row.Owner != actorOwner {
			return fmt.Errorf("owner mismatch: slot %s is owned by %s, but actor is %s", slotID, row.Owner, actorOwner)
		}

		specFile := row.SpecPath + "/spec.json"
		if _, err := os.Stat(specFile); err != nil {
			return fmt.Errorf("missing spec file: %s", specFile)
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
			return fmt.Errorf("validation failed")
		}

		doc.Rows[idx].Status = "DONE"
		doc.Rows[idx].UpdatedUTC = specutil.NowUTC()
		doc.Rows[idx].Notes = "done"

		updated := specutil.RenderDispatch(doc)
		if err := os.WriteFile("spec/MINING_DISPATCH.json", []byte(updated), 0o644); err != nil {
			return err
		}

		fmt.Printf("marked %s DONE (%s)\n", slotID, row.SpecPath)
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
