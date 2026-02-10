package specutil

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type CatalogRow struct {
	SlotID        string `json:"slot_id"`
	PriorityGroup string `json:"priority_group"`
	SpecPath      string `json:"spec_path"`
	SourceRoots   string `json:"source_roots"`
}

type CatalogDoc struct {
	SchemaVersion string       `json:"schema_version"`
	Slots         []CatalogRow `json:"slots"`
}

func ParseCatalogJSON(path string) ([]CatalogRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc := CatalogDoc{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("invalid catalog JSON in %s: %w", path, err)
	}
	if doc.SchemaVersion != "1.0.0" {
		return nil, fmt.Errorf("invalid catalog schema_version in %s: %s", path, doc.SchemaVersion)
	}
	if len(doc.Slots) == 0 {
		return nil, fmt.Errorf("%s has no slots", path)
	}

	seen := map[string]struct{}{}
	for _, row := range doc.Slots {
		if row.SlotID == "" || row.PriorityGroup == "" || row.SpecPath == "" || row.SourceRoots == "" {
			return nil, fmt.Errorf("invalid catalog row with empty field: %+v", row)
		}
		if _, ok := seen[row.SlotID]; ok {
			return nil, fmt.Errorf("duplicate catalog slot_id: %s", row.SlotID)
		}
		seen[row.SlotID] = struct{}{}
	}

	return doc.Slots, nil
}

// ParseCatalogTSV is kept as a temporary compatibility wrapper.
func ParseCatalogTSV(path string) ([]CatalogRow, error) {
	return ParseCatalogJSON(path)
}

func ChangedPackagePaths(changedFiles []string, catalogRows []CatalogRow) ([]string, error) {
	packagePaths := make([]string, 0, len(catalogRows))
	for _, row := range catalogRows {
		packagePaths = append(packagePaths, row.SpecPath)
	}
	sort.Slice(packagePaths, func(i, j int) bool {
		return len(packagePaths[i]) > len(packagePaths[j])
	})

	touched := map[string]struct{}{}
	for _, file := range changedFiles {
		if !strings.HasPrefix(file, "spec/packages/") {
			continue
		}
		matched := ""
		for _, pkg := range packagePaths {
			if file == pkg || strings.HasPrefix(file, pkg+"/") {
				matched = pkg
				break
			}
		}
		if matched == "" {
			return nil, fmt.Errorf("changed package file does not map to a catalog package: %s", file)
		}
		touched[matched] = struct{}{}
	}

	out := make([]string, 0, len(touched))
	for p := range touched {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}
