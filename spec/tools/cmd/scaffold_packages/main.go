package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	catalogPath := "spec/PACKAGE_CATALOG.tsv"
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

	lines := strings.Split(strings.TrimSpace(string(catalogRaw)), "\n")
	if len(lines) == 0 {
		fmt.Fprintf(os.Stderr, "empty %s\n", catalogPath)
		os.Exit(1)
	}
	if lines[0] != "slot_id\tpriority_group\tspec_path\tsource_roots" {
		fmt.Fprintf(os.Stderr, "invalid %s header\n", catalogPath)
		os.Exit(1)
	}

	nonAlphaNum := regexp.MustCompile(`[^A-Z0-9]+`)

	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			fmt.Fprintf(os.Stderr, "invalid catalog row: %s\n", line)
			os.Exit(1)
		}
		specPath := parts[2]
		sourceRoots := parts[3]

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

	fmt.Println("scaffolded package specs (JSON) from spec/PACKAGE_CATALOG.tsv")
}
