package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

	type catalogSlot struct {
		SlotID        string `json:"slot_id"`
		PriorityGroup string `json:"priority_group"`
		SpecPath      string `json:"spec_path"`
		SourceRoots   string `json:"source_roots"`
	}
	type catalogDoc struct {
		SchemaVersion string        `json:"schema_version"`
		Slots         []catalogSlot `json:"slots"`
	}
	catalogSlots := make([]catalogSlot, 0, len(rows))
	for _, r := range rows {
		catalogSlots = append(catalogSlots, catalogSlot{
			SlotID:        r.SlotID,
			PriorityGroup: r.PriorityGroup,
			SpecPath:      r.SpecPath,
			SourceRoots:   r.SourceRoots,
		})
	}
	catalogPayload, err := json.MarshalIndent(catalogDoc{
		SchemaVersion: "1.0.0",
		Slots:         catalogSlots,
	}, "", "\t")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if err := os.WriteFile("spec/PACKAGE_CATALOG.json", append(catalogPayload, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	type dispatchSlot struct {
		SlotID        string `json:"slot_id"`
		Status        string `json:"status"`
		PriorityGroup string `json:"priority_group"`
		SpecPath      string `json:"spec_path"`
		SourceRoots   string `json:"source_roots"`
		Owner         string `json:"owner"`
		UpdatedUTC    string `json:"updated_utc"`
		Notes         string `json:"notes"`
	}
	type dispatchDoc struct {
		SchemaVersion string         `json:"schema_version"`
		Slots         []dispatchSlot `json:"slots"`
	}
	dispatchSlots := make([]dispatchSlot, 0, len(rows))
	for _, r := range rows {
		dispatchSlots = append(dispatchSlots, dispatchSlot{
			SlotID:        r.SlotID,
			Status:        "OPEN",
			PriorityGroup: r.PriorityGroup,
			SpecPath:      r.SpecPath,
			SourceRoots:   r.SourceRoots,
			Owner:         "-",
			UpdatedUTC:    "-",
			Notes:         "-",
		})
	}
	dispatchPayload, err := json.MarshalIndent(dispatchDoc{
		SchemaVersion: "1.0.0",
		Slots:         dispatchSlots,
	}, "", "\t")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if err := os.WriteFile("spec/MINING_DISPATCH.json", append(dispatchPayload, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Println("generated spec/PACKAGE_CATALOG.json and spec/MINING_DISPATCH.json")
}
