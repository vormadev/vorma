package main

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func main() {
	doc, err := specutil.ParseDispatchFile("spec/MINING_DISPATCH.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	catalogRows, err := specutil.ParseCatalogJSON("spec/PACKAGE_CATALOG.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if len(doc.Rows) != len(catalogRows) {
		fmt.Fprintf(os.Stderr, "dispatch row count (%d) does not match catalog row count (%d)\n", len(doc.Rows), len(catalogRows))
		os.Exit(1)
	}

	claimsByOwner := map[string]int{}
	firstOpen := -1
	bySlot := map[string]specutil.CatalogRow{}
	for _, row := range catalogRows {
		bySlot[row.SlotID] = row
	}

	for i, row := range doc.Rows {
		cat, ok := bySlot[row.SlotID]
		if !ok {
			fmt.Fprintf(os.Stderr, "dispatch row references unknown slot_id: %s\n", row.SlotID)
			os.Exit(1)
		}
		if row.PriorityGroup != cat.PriorityGroup || row.SpecPath != cat.SpecPath || row.SourceRoots != cat.SourceRoots {
			fmt.Fprintf(
				os.Stderr,
				"dispatch row static fields mismatch catalog for %s: expected (%s, %s, %s), got (%s, %s, %s)\n",
				row.SlotID,
				cat.PriorityGroup,
				cat.SpecPath,
				cat.SourceRoots,
				row.PriorityGroup,
				row.SpecPath,
				row.SourceRoots,
			)
			os.Exit(1)
		}

		switch row.Status {
		case "OPEN", "CLAIMED", "DONE":
		default:
			fmt.Fprintf(os.Stderr, "invalid status in dispatch row: %s => %s\n", row.SlotID, row.Status)
			os.Exit(1)
		}

		if row.Status == "OPEN" {
			if !(row.Owner == "-" && row.UpdatedUTC == "-" && row.Notes == "-") {
				fmt.Fprintf(os.Stderr, "OPEN row metadata must be owner=-, updated_utc=-, notes=-: %s\n", row.SlotID)
				os.Exit(1)
			}
			if firstOpen == -1 {
				firstOpen = i
			}
		}

		if row.Status == "CLAIMED" {
			if row.Owner == "-" || !specutil.IsUTCRFC3339(row.UpdatedUTC) || row.Notes != "claimed" {
				fmt.Fprintf(os.Stderr, "CLAIMED row metadata invalid: %s\n", row.SlotID)
				os.Exit(1)
			}
			claimsByOwner[row.Owner]++
		}

		if row.Status == "DONE" {
			if row.Owner == "-" || !specutil.IsUTCRFC3339(row.UpdatedUTC) || row.Notes != "done" {
				fmt.Fprintf(os.Stderr, "DONE row metadata invalid: %s\n", row.SlotID)
				os.Exit(1)
			}
		}
	}

	for owner, count := range claimsByOwner {
		if count > 1 {
			fmt.Fprintf(os.Stderr, "owner has more than one CLAIMED slot: %s\n", owner)
			os.Exit(1)
		}
	}

	if firstOpen != -1 {
		for i := firstOpen + 1; i < len(doc.Rows); i++ {
			if doc.Rows[i].Status != "OPEN" {
				fmt.Fprintln(os.Stderr, "dispatch order invalid: once OPEN begins, all later slots must be OPEN")
				os.Exit(1)
			}
		}
	}

	changedFiles, err := specutil.ChangedFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	touchedPaths, err := specutil.ChangedPackagePaths(changedFiles, catalogRows)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if len(touchedPaths) == 0 {
		return
	}
	if len(touchedPaths) > 1 {
		fmt.Fprintln(os.Stderr, "multiple changed package paths detected (use one isolated worktree per agent)")
		os.Exit(1)
	}

	touched := touchedPaths[0]
	touchedIndex := -1
	for i, row := range doc.Rows {
		if row.SpecPath == touched {
			touchedIndex = i
			if row.Status != "CLAIMED" && row.Status != "DONE" {
				fmt.Fprintf(os.Stderr, "changed package path is not CLAIMED or DONE in dispatch: %s (%s)\n", touched, row.Status)
				os.Exit(1)
			}
			break
		}
	}
	if touchedIndex == -1 {
		fmt.Fprintf(os.Stderr, "changed package path not found in dispatch: %s\n", touched)
		os.Exit(1)
	}

	for i := 0; i < touchedIndex; i++ {
		if doc.Rows[i].Status == "OPEN" {
			fmt.Fprintf(os.Stderr, "dispatch order invalid: %s package has OPEN slot before it\n", doc.Rows[touchedIndex].Status)
			fmt.Fprintf(os.Stderr, "changed: %s\n", touched)
			os.Exit(1)
		}
	}
}
