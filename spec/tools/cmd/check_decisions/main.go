package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

var (
	reDecisionID = regexp.MustCompile(`^DEC-[0-9]{4}$`)
	reDate       = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func containsPlaceholder(s string) bool {
	t := strings.ToUpper(strings.TrimSpace(s))
	if t == "" {
		return true
	}
	return strings.Contains(t, "TODO") || strings.Contains(t, "TBD")
}

func validatePackagePaths(line int, paths []string) {
	if len(paths) == 0 {
		fail("decisions line %d must include at least one package path", line)
	}
	for _, p := range paths {
		if containsPlaceholder(p) {
			fail("decisions line %d has placeholder package path", line)
		}
		if p == "cross-package" {
			continue
		}
		if p == "" {
			fail("decisions line %d has empty package path", line)
		}
		if !strings.HasPrefix(p, "spec/packages/") {
			fail("decisions line %d package path must start with spec/packages/: %s", line, p)
		}
		if !specutil.IsRelativeRepoPath(p) {
			fail("decisions line %d package path must be repository-relative: %s", line, p)
		}
	}
}

func main() {
	rows, err := specutil.ParseDecisionsFile("spec/DECISIONS.json")
	if err != nil {
		fail("%v", err)
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		if !reDecisionID.MatchString(row.DecisionID) {
			fail("decisions line %d has invalid Decision ID: %s", row.Line, row.DecisionID)
		}
		if _, exists := seen[row.DecisionID]; exists {
			fail("duplicate Decision ID: %s", row.DecisionID)
		}
		seen[row.DecisionID] = struct{}{}

		if !reDate.MatchString(row.Date) {
			fail("decisions line %d has invalid Date (expected YYYY-MM-DD): %s", row.Line, row.Date)
		}

		switch row.Status {
		case "OPEN", "RESOLVED":
		default:
			fail("decisions line %d has invalid Status: %s", row.Line, row.Status)
		}

		validatePackagePaths(row.Line, row.PackagePaths)

		if containsPlaceholder(row.Question) {
			fail("decisions line %d has placeholder Question", row.Line)
		}
		if len(row.OptionsConsidered) == 0 {
			fail("decisions line %d must include options_considered", row.Line)
		}
		for _, opt := range row.OptionsConsidered {
			if containsPlaceholder(opt) {
				fail("decisions line %d has placeholder Options Considered", row.Line)
			}
		}

		if row.Status == "OPEN" {
			if strings.TrimSpace(row.SelectedOption) != "-" ||
				strings.TrimSpace(row.Rationale) != "-" ||
				len(row.EvidenceRefs) != 1 || strings.TrimSpace(row.EvidenceRefs[0]) != "-" {
				fail("decisions line %d OPEN rows must set selected_option, rationale, and evidence_refs=['-']", row.Line)
			}
			continue
		}

		if containsPlaceholder(row.SelectedOption) || strings.TrimSpace(row.SelectedOption) == "-" {
			fail("decisions line %d RESOLVED row must provide Selected Option", row.Line)
		}
		if containsPlaceholder(row.Rationale) || strings.TrimSpace(row.Rationale) == "-" {
			fail("decisions line %d RESOLVED row must provide Rationale", row.Line)
		}
		if len(row.EvidenceRefs) == 0 {
			fail("decisions line %d RESOLVED row must provide evidence_refs", row.Line)
		}
		for _, ref := range row.EvidenceRefs {
			if containsPlaceholder(ref) || strings.TrimSpace(ref) == "-" {
				fail("decisions line %d RESOLVED row must provide Evidence", row.Line)
			}
		}
	}
}
