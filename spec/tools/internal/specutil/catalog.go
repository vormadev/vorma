package specutil

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type CatalogRow struct {
	SlotID        string
	PriorityGroup string
	SpecPath      string
	SourceRoots   string
}

func ParseCatalogTSV(path string) ([]CatalogRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}
	if lines[0] != "slot_id\tpriority_group\tspec_path\tsource_roots" {
		return nil, fmt.Errorf("invalid %s header", path)
	}
	rows := make([]CatalogRow, 0, len(lines)-1)
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			return nil, fmt.Errorf("invalid catalog row: %s", line)
		}
		rows = append(rows, CatalogRow{
			SlotID:        parts[0],
			PriorityGroup: parts[1],
			SpecPath:      parts[2],
			SourceRoots:   parts[3],
		})
	}
	return rows, nil
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
