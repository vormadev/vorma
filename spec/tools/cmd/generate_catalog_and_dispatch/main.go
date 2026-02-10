package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type row struct {
	SlotID        string
	PriorityGroup string
	SpecPath      string
	SourceRoots   string
}

func collectDirs(root string) ([]string, error) {
	dirs := []string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

func hasDirectFile(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true, nil
		}
	}
	return false, nil
}

func main() {
	rows := make([]row, 0)
	slot := 1
	add := func(group, specPath, sourceRoots string) {
		rows = append(rows, row{
			SlotID:        fmt.Sprintf("SLOT-%03d", slot),
			PriorityGroup: group,
			SpecPath:      specPath,
			SourceRoots:   sourceRoots,
		})
		slot++
	}

	add("vorma", "spec/packages/vormaroot", "vorma.go")
	add("vorma", "spec/packages/vormabuild", "vormabuild/**")
	add("vorma", "spec/packages/vormaruntime", "vormaruntime/**")
	add("vorma", "spec/packages/vormaclient/client", "vormaclient/client/**")
	add("vorma", "spec/packages/vormaclient/react", "vormaclient/react/**")
	add("vorma", "spec/packages/vormaclient/preact", "vormaclient/preact/**")
	add("vorma", "spec/packages/vormaclient/solid", "vormaclient/solid/**")
	add("vorma", "spec/packages/vormaclient/vite", "vormaclient/vite/**")

	add("wave", "spec/packages/wave", "wave/*.go")
	add("wave", "spec/packages/wave/tooling", "wave/tooling/**")

	kitDirs, err := collectDirs("kit")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	for _, d := range kitDirs {
		if d == "kit" {
			continue
		}
		hasFile, err := hasDirectFile(d)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		if hasFile {
			add("kit", "spec/packages/"+d, d+"/**")
		}
	}

	labDirs, err := collectDirs("lab")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	for _, d := range labDirs {
		if d == "lab" {
			continue
		}
		hasFile, err := hasDirectFile(d)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		if hasFile {
			add("lab", "spec/packages/"+d, d+"/**")
		}
	}

	add("bootstrap-create", "spec/packages/bootstrap", "bootstrap/**")
	add("bootstrap-create", "spec/packages/vormaclient/create", "vormaclient/create/**")

	catalog := strings.Builder{}
	catalog.WriteString("slot_id\tpriority_group\tspec_path\tsource_roots\n")
	for _, r := range rows {
		catalog.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n", r.SlotID, r.PriorityGroup, r.SpecPath, r.SourceRoots))
	}
	if err := os.WriteFile("spec/PACKAGE_CATALOG.tsv", []byte(catalog.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	dispatch := strings.Builder{}
	dispatch.WriteString("# Mining Dispatch\n\n")
	dispatch.WriteString("Claim rules:\n\n")
	dispatch.WriteString("1. Claim the lowest-numbered slot with status `OPEN`.\n")
	dispatch.WriteString("2. Set slot status to `CLAIMED` before editing package artifacts.\n")
	dispatch.WriteString("3. Edit only the claimed `spec_path` under `spec/packages/**`.\n")
	dispatch.WriteString("4. One active `CLAIMED` slot per owner.\n")
	dispatch.WriteString("5. Only the slot owner may mark that slot `DONE`.\n")
	dispatch.WriteString("6. Set status to `DONE` only after passing all guard checks.\n")
	dispatch.WriteString("7. A slot must remain `CLAIMED` until both required independent review passes are recorded as `PASS_NO_NOTES`.\n\n")
	dispatch.WriteString("Preferred commands:\n\n")
	dispatch.WriteString("- Claim: `go run ./spec/tools/cmd/claim_lowest_open_slot <owner>`\n")
	dispatch.WriteString("- Review: `go run ./spec/tools/cmd/record_review_pass SLOT-XXX <reviewer_owner> <reviewer_claim_slot> <pass(1|2)> <PASS_NO_NOTES|FAIL_NOTES> <notes_ref>`\n")
	dispatch.WriteString("- Done: `go run ./spec/tools/cmd/mark_slot_done SLOT-XXX <owner>`\n\n")
	dispatch.WriteString("Edit only rows in the TSV block below when claiming or completing work.\n\n")
	dispatch.WriteString("```tsv\n")
	dispatch.WriteString("slot_id\tstatus\tpriority_group\tspec_path\tsource_roots\towner\tupdated_utc\tnotes\n")
	for _, r := range rows {
		dispatch.WriteString(fmt.Sprintf("%s\tOPEN\t%s\t%s\t%s\t-\t-\t-\n", r.SlotID, r.PriorityGroup, r.SpecPath, r.SourceRoots))
	}
	dispatch.WriteString("```\n")

	if err := os.WriteFile("spec/MINING_DISPATCH.md", []byte(dispatch.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Println("generated spec/PACKAGE_CATALOG.tsv and spec/MINING_DISPATCH.md")
}
