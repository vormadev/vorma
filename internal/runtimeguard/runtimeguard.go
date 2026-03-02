// Package runtimeguard centralizes runtime/buildtime dependency-boundary checks.
//
// Keeping roots, forbidden prefixes, dependency graph collection, and violation
// formatting in one place prevents drift between CI command checks and test
// contract checks.
package runtimeguard

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// RuntimeDependencyBoundaryCheck defines one runtime package root whose
// transitive dependency closure must exclude build/dev-time packages.
type RuntimeDependencyBoundaryCheck struct {
	RootPackagePath string
}

// DependencyViolation records one runtime dependency-boundary failure.
type DependencyViolation struct {
	RootPackagePath string
	DependencyPath  string
	MatchedPrefix   string
}

// EvaluationReport captures one full dependency-boundary evaluation run.
type EvaluationReport struct {
	CheckedRuntimeRoots []string
	ForbiddenPrefixes   []string
	Violations          []DependencyViolation
}

var defaultForbiddenRuntimeDependencyPathPrefixes = []string{
	"github.com/vormadev/vorma/vormabuild",
	"github.com/vormadev/vorma/wave/buildtime",
	"github.com/vormadev/vorma/lab/vitecmd",
	"github.com/evanw/esbuild",
	"github.com/fsnotify/fsnotify",
	"github.com/gorilla/websocket",
}

var defaultRuntimeRootPackagePaths = []string{
	"github.com/vormadev/vorma/wave",
	"github.com/vormadev/vorma/internal/vormaruntime",
	"github.com/vormadev/vorma",
}

const defaultBuildtimeSentinelRootPackagePath = "github.com/vormadev/vorma/wave/buildtime/devserver"

var defaultBuildtimeSentinelDependencyPrefixes = []string{
	"github.com/vormadev/vorma/lab/vitecmd",
	"github.com/evanw/esbuild",
	"github.com/fsnotify/fsnotify",
}

// DefaultRuntimeBoundaryChecks returns the default runtime roots checked by the
// guard.
func DefaultRuntimeBoundaryChecks() []RuntimeDependencyBoundaryCheck {
	checks := make([]RuntimeDependencyBoundaryCheck, 0, len(defaultRuntimeRootPackagePaths))
	for _, runtimeRootPackagePath := range defaultRuntimeRootPackagePaths {
		checks = append(
			checks,
			RuntimeDependencyBoundaryCheck{
				RootPackagePath: runtimeRootPackagePath,
			},
		)
	}
	return checks
}

// DefaultForbiddenRuntimeDependencyPathPrefixes returns forbidden build/dev
// dependency prefixes for runtime roots.
func DefaultForbiddenRuntimeDependencyPathPrefixes() []string {
	return append([]string(nil), defaultForbiddenRuntimeDependencyPathPrefixes...)
}

// DefaultBuildtimeSentinelRootPackagePath returns the default buildtime root
// used by sentinel-coverage checks.
func DefaultBuildtimeSentinelRootPackagePath() string {
	return defaultBuildtimeSentinelRootPackagePath
}

// DefaultBuildtimeSentinelDependencyPrefixes returns default buildtime
// dependency prefixes expected to appear in a known buildtime root closure.
func DefaultBuildtimeSentinelDependencyPrefixes() []string {
	return append([]string(nil), defaultBuildtimeSentinelDependencyPrefixes...)
}

// EvaluateDefaultRuntimeDependencyBoundaries evaluates default runtime roots
// against default forbidden build/dev dependency prefixes.
func EvaluateDefaultRuntimeDependencyBoundaries() (*EvaluationReport, error) {
	return EvaluateRuntimeDependencyBoundaries(
		DefaultRuntimeBoundaryChecks(),
		DefaultForbiddenRuntimeDependencyPathPrefixes(),
	)
}

// EvaluateRuntimeDependencyBoundaries evaluates runtime roots against forbidden
// dependency prefixes.
func EvaluateRuntimeDependencyBoundaries(
	boundaryChecks []RuntimeDependencyBoundaryCheck,
	forbiddenDependencyPathPrefixes []string,
) (*EvaluationReport, error) {
	if len(boundaryChecks) == 0 {
		return nil, fmt.Errorf("runtime dependency boundary checks are empty")
	}
	if len(forbiddenDependencyPathPrefixes) == 0 {
		return nil, fmt.Errorf("forbidden dependency prefix list is empty")
	}

	report := &EvaluationReport{
		CheckedRuntimeRoots: make([]string, 0, len(boundaryChecks)),
		ForbiddenPrefixes: append(
			[]string(nil),
			forbiddenDependencyPathPrefixes...,
		),
		Violations: make([]DependencyViolation, 0),
	}

	for _, boundaryCheck := range boundaryChecks {
		rootPackagePath := strings.TrimSpace(boundaryCheck.RootPackagePath)
		if rootPackagePath == "" {
			return nil, fmt.Errorf("runtime root package path is empty")
		}
		report.CheckedRuntimeRoots = append(
			report.CheckedRuntimeRoots,
			rootPackagePath,
		)

		dependencies, listDependenciesError := listPackageDependencies(
			rootPackagePath,
		)
		if listDependenciesError != nil {
			return nil, listDependenciesError
		}

		violations := findBoundaryViolations(
			rootPackagePath,
			dependencies,
			forbiddenDependencyPathPrefixes,
		)
		report.Violations = append(report.Violations, violations...)
	}

	sort.Strings(report.CheckedRuntimeRoots)
	sort.Strings(report.ForbiddenPrefixes)
	sort.Slice(
		report.Violations,
		func(leftIndex int, rightIndex int) bool {
			left := report.Violations[leftIndex]
			right := report.Violations[rightIndex]
			if left.RootPackagePath != right.RootPackagePath {
				return left.RootPackagePath < right.RootPackagePath
			}
			if left.DependencyPath != right.DependencyPath {
				return left.DependencyPath < right.DependencyPath
			}
			return left.MatchedPrefix < right.MatchedPrefix
		},
	)

	return report, nil
}

// FindPresentDependencyPrefixMatches returns candidate prefixes that are
// actually present in one root package dependency closure.
func FindPresentDependencyPrefixMatches(
	rootPackagePath string,
	candidatePrefixes []string,
) ([]string, error) {
	dependencies, listDependenciesError := listPackageDependencies(rootPackagePath)
	if listDependenciesError != nil {
		return nil, listDependenciesError
	}

	matchingPrefixes := make([]string, 0, len(candidatePrefixes))
	for _, candidatePrefix := range candidatePrefixes {
		if hasDependencyWithPrefix(dependencies, candidatePrefix) {
			matchingPrefixes = append(matchingPrefixes, candidatePrefix)
		}
	}
	sort.Strings(matchingPrefixes)
	return matchingPrefixes, nil
}

// HasViolations reports whether any boundary violations were found.
func (report *EvaluationReport) HasViolations() bool {
	return report != nil && len(report.Violations) > 0
}

// FormatViolationReport formats evaluation output for CI and human debugging.
func (report *EvaluationReport) FormatViolationReport() string {
	if report == nil {
		return "runtime dependency guard report is nil"
	}

	var reportBuilder strings.Builder
	reportBuilder.WriteString(
		"Build/dev dependencies leaked into runtime dependency closures.\n",
	)
	if len(report.Violations) == 0 {
		reportBuilder.WriteString("Violations:\n- none\n")
	} else {
		reportBuilder.WriteString("Violations:\n")
		for _, violation := range report.Violations {
			reportBuilder.WriteString("- ")
			reportBuilder.WriteString(violation.RootPackagePath)
			reportBuilder.WriteString(" depends on forbidden ")
			reportBuilder.WriteString(violation.DependencyPath)
			reportBuilder.WriteString(" (matched ")
			reportBuilder.WriteString(fmt.Sprintf("%q", violation.MatchedPrefix))
			reportBuilder.WriteString(")\n")
		}
	}
	reportBuilder.WriteString("\nChecked runtime roots:\n")
	for _, rootPackagePath := range report.CheckedRuntimeRoots {
		reportBuilder.WriteString("- ")
		reportBuilder.WriteString(rootPackagePath)
		reportBuilder.WriteByte('\n')
	}
	reportBuilder.WriteString("\nForbidden dependency path prefixes:\n")
	for _, forbiddenDependencyPathPrefix := range report.ForbiddenPrefixes {
		reportBuilder.WriteString("- ")
		reportBuilder.WriteString(forbiddenDependencyPathPrefix)
		reportBuilder.WriteByte('\n')
	}

	return strings.TrimRight(reportBuilder.String(), "\n")
}

func listPackageDependencies(rootPackagePath string) ([]string, error) {
	command := exec.Command(
		"go",
		"list",
		"-deps",
		"-f",
		"{{.ImportPath}}",
		rootPackagePath,
	)
	var commandStdout bytes.Buffer
	var commandStderr bytes.Buffer
	command.Stdout = &commandStdout
	command.Stderr = &commandStderr

	if commandErr := command.Run(); commandErr != nil {
		return nil, fmt.Errorf(
			"go list -deps %s failed: %w\n%s",
			rootPackagePath,
			commandErr,
			strings.TrimSpace(commandStderr.String()),
		)
	}

	uniqueDependencies := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(commandStdout.Bytes()))
	for scanner.Scan() {
		dependencyPath := strings.TrimSpace(scanner.Text())
		if dependencyPath == "" {
			continue
		}
		uniqueDependencies[dependencyPath] = struct{}{}
	}
	if scannerErr := scanner.Err(); scannerErr != nil {
		return nil, fmt.Errorf(
			"parse go list output for %s: %w",
			rootPackagePath,
			scannerErr,
		)
	}
	if len(uniqueDependencies) == 0 {
		return nil, fmt.Errorf(
			"go list returned zero dependencies for %s",
			rootPackagePath,
		)
	}
	if _, hasRootPackage := uniqueDependencies[rootPackagePath]; !hasRootPackage {
		return nil, fmt.Errorf(
			"go list output for %s omitted package itself",
			rootPackagePath,
		)
	}

	dependencies := make([]string, 0, len(uniqueDependencies))
	for dependencyPath := range uniqueDependencies {
		dependencies = append(dependencies, dependencyPath)
	}
	sort.Strings(dependencies)
	return dependencies, nil
}

func findBoundaryViolations(
	rootPackagePath string,
	dependencies []string,
	forbiddenDependencyPathPrefixes []string,
) []DependencyViolation {
	violations := make([]DependencyViolation, 0)
	for _, dependencyPath := range dependencies {
		forbiddenDependencyPathPrefix, isForbiddenDependency := findMatchingPrefix(
			dependencyPath,
			forbiddenDependencyPathPrefixes,
		)
		if !isForbiddenDependency {
			continue
		}
		violations = append(
			violations,
			DependencyViolation{
				RootPackagePath: rootPackagePath,
				DependencyPath:  dependencyPath,
				MatchedPrefix:   forbiddenDependencyPathPrefix,
			},
		)
	}
	return violations
}

func hasDependencyWithPrefix(
	dependencies []string,
	prefix string,
) bool {
	for _, dependencyPath := range dependencies {
		if dependencyPath == prefix {
			return true
		}
		if strings.HasPrefix(dependencyPath, prefix+"/") {
			return true
		}
	}
	return false
}

func findMatchingPrefix(
	importPath string,
	candidatePrefixes []string,
) (string, bool) {
	for _, candidatePrefix := range candidatePrefixes {
		if importPath == candidatePrefix {
			return candidatePrefix, true
		}
		if strings.HasPrefix(importPath, candidatePrefix+"/") {
			return candidatePrefix, true
		}
	}
	return "", false
}
