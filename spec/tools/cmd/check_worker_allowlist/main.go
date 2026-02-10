package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

func run(name string, args ...string) ([]string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	lines := strings.Split(out.String(), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func main() {
	diff, err := run("git", "diff", "--name-only", "--diff-filter=ACMRTD", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list changed files: %v\n", err)
		os.Exit(1)
	}
	untracked, err := run("git", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list untracked files: %v\n", err)
		os.Exit(1)
	}

	seen := map[string]struct{}{}
	all := make([]string, 0, len(diff)+len(untracked))
	for _, file := range append(diff, untracked...) {
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		all = append(all, file)
	}
	sort.Strings(all)

	allowedPackageFile := regexp.MustCompile(`^spec/packages/.+/spec\.json$`)
	violations := make([]string, 0)

	for _, file := range all {
		switch file {
		case "spec/MINING_DISPATCH.json", "spec/DECISIONS.json", "spec/TRACEABILITY.json":
			continue
		default:
			if allowedPackageFile.MatchString(file) {
				continue
			}
			violations = append(violations, file)
		}
	}

	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "worker allowlist violation: these files are not editable during mining:")
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, " - %s\n", v)
		}
		fmt.Fprintln(os.Stderr, "for parallel mining, use isolated worktrees so each worker sees only its own allowed changes")
		fmt.Fprintln(os.Stderr, "allowed paths: spec/packages/<claimed-path>/spec.json, spec/MINING_DISPATCH.json, spec/DECISIONS.json, spec/TRACEABILITY.json")
		os.Exit(1)
	}
}
