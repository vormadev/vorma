package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	catalogPath := "spec/PACKAGE_CATALOG.json"
	templatePath := "spec/_templates/package/spec.json"

	catalogRaw, err := os.ReadFile(catalogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "missing %s\n", catalogPath)
		os.Exit(1)
	}
	templateRaw, err := os.ReadFile(templatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "missing %s\n", templatePath)
		os.Exit(1)
	}
	template := string(templateRaw)

	catalog := struct {
		SchemaVersion string `json:"schema_version"`
		Slots         []struct {
			SlotID        string `json:"slot_id"`
			PriorityGroup string `json:"priority_group"`
			SpecPath      string `json:"spec_path"`
			SourceRoots   string `json:"source_roots"`
		} `json:"slots"`
	}{}
	if err := json.Unmarshal(catalogRaw, &catalog); err != nil {
		fmt.Fprintf(os.Stderr, "invalid %s JSON: %v\n", catalogPath, err)
		os.Exit(1)
	}
	if catalog.SchemaVersion != "1.0.0" {
		fmt.Fprintf(os.Stderr, "invalid %s schema_version\n", catalogPath)
		os.Exit(1)
	}

	nonAlphaNum := regexp.MustCompile(`[^A-Z0-9]+`)

	for _, slot := range catalog.Slots {
		specPath := slot.SpecPath
		sourceRoots := slot.SourceRoots

		if err := os.MkdirAll(specPath, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", specPath, err)
			os.Exit(1)
		}

		entries, err := os.ReadDir(specPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read dir %s: %v\n", specPath, err)
			os.Exit(1)
		}
		for _, entry := range entries {
			if entry.Name() == "spec.json" {
				continue
			}
			if err := os.RemoveAll(filepath.Join(specPath, entry.Name())); err != nil {
				fmt.Fprintf(os.Stderr, "remove %s/%s: %v\n", specPath, entry.Name(), err)
				os.Exit(1)
			}
		}

		packageRel := strings.TrimPrefix(specPath, "spec/packages/")
		reqPrefix := strings.ToUpper(packageRel)
		reqPrefix = nonAlphaNum.ReplaceAllString(reqPrefix, "-")
		reqPrefix = strings.Trim(reqPrefix, "-")

		rendered := template
		rendered = strings.ReplaceAll(rendered, "@@PACKAGE_NAME@@", packageRel)
		rendered = strings.ReplaceAll(rendered, "@@SPEC_PATH@@", specPath)
		rendered = strings.ReplaceAll(rendered, "@@SOURCE_ROOTS@@", sourceRoots)
		rendered = strings.ReplaceAll(rendered, "@@REQ_PREFIX@@", reqPrefix)

		if err := os.WriteFile(filepath.Join(specPath, "spec.json"), []byte(rendered), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s/spec.json: %v\n", specPath, err)
			os.Exit(1)
		}
	}

	fmt.Println("scaffolded package specs (JSON) from spec/PACKAGE_CATALOG.json")
}
