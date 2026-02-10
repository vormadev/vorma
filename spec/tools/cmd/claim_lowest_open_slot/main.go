package main

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: go run ./spec/tools/cmd/claim_lowest_open_slot <owner>")
	os.Exit(1)
}

func main() {
	owner := ""
	if len(os.Args) > 1 {
		owner = os.Args[1]
	}
	if owner == "" {
		if os.Getenv("USER") != "" {
			owner = os.Getenv("USER")
		}
	}
	if owner == "" {
		usage()
	}
	if owner == "-" {
		fmt.Fprintln(os.Stderr, "owner name '-' is reserved and cannot be used")
		os.Exit(1)
	}

	err := specutil.WithLock("spec/.dispatch.lock", func() error {
		doc, err := specutil.ParseDispatchFile("spec/MINING_DISPATCH.md")
		if err != nil {
			return err
		}

		allOpen := true
		for _, row := range doc.Rows {
			if row.Status != "OPEN" {
				allOpen = false
				break
			}
		}
		if allOpen {
			changedFiles, err := specutil.ChangedFiles()
			if err != nil {
				return err
			}
			if len(changedFiles) > 0 {
				return fmt.Errorf("cannot claim first slot: commit current spec scaffold baseline first")
			}
		}

		for _, row := range doc.Rows {
			if row.Status == "CLAIMED" && row.Owner == owner {
				return fmt.Errorf("owner already has an active claim: %s (%s)", owner, row.SlotID)
			}
		}

		openIndex := -1
		for i, row := range doc.Rows {
			if row.Status == "OPEN" {
				openIndex = i
				break
			}
		}
		if openIndex == -1 {
			return fmt.Errorf("no OPEN slot is available")
		}

		now := specutil.NowUTC()
		doc.Rows[openIndex].Status = "CLAIMED"
		doc.Rows[openIndex].Owner = owner
		doc.Rows[openIndex].UpdatedUTC = now
		doc.Rows[openIndex].Notes = "claimed"

		updated := specutil.RenderDispatch(doc)
		if err := os.WriteFile("spec/MINING_DISPATCH.md", []byte(updated), 0o644); err != nil {
			return err
		}

		fmt.Printf("claimed %s (%s) for owner %s\n", doc.Rows[openIndex].SlotID, doc.Rows[openIndex].SpecPath, owner)
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
