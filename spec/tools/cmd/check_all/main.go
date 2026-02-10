package main

import (
	"fmt"
	"os"
	"os/exec"
)

type step struct {
	name string
	args []string
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/go-build")
	return cmd.Run()
}

func main() {
	steps := []step{
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_phase_scope"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_worker_allowlist"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_single_package_scope"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_append_only_docs"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_dispatch_manual_edits"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_dispatch_claim"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_decisions"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_traceability"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_requirements_evidence"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_phase_completion"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_absolute_paths"}},
		{name: "go", args: []string{"run", "./spec/tools/cmd/check_no_mermaid"}},
		{name: "pnpm", args: []string{"prettier", "--check", "spec/**/*.md", "spec/**/*.json"}},
	}

	for _, s := range steps {
		if err := run(s.name, s.args...); err != nil {
			fmt.Fprintf(os.Stderr, "check step failed: %s %v\n", s.name, s.args)
			os.Exit(1)
		}
	}

	fmt.Println("all spec guardrail checks passed")
}
