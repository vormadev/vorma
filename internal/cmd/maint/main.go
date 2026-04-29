package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type maint_app struct {
	root    string
	yes     bool
	dry_run bool
	verbose bool
}

func main() {
	app := maint_app{}
	if err := app.run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "maint:", err)
		os.Exit(1)
	}
}

func (app *maint_app) run(args []string) error {
	root, err := app.find_repo_root()
	if err != nil {
		return err
	}
	app.root = root
	if err := os.Chdir(root); err != nil {
		return err
	}

	command, rest, err := app.parse_global_flags(args)
	if err != nil {
		return err
	}

	switch command {
	case "", "help":
		app.print_help()
		return nil
	case "install-js":
		return app.install_js()
	case "clean-js":
		return app.clean_js()
	case "fmt-ts":
		return app.fmt_ts()
	case "lint-ts":
		return app.lint_ts()
	case "typecheck-ts":
		return app.typecheck_ts()
	case "typecheck-ts-other":
		return app.typecheck_ts_other()
	case "typecheck-ts-framework":
		return app.typecheck_ts_framework()
	case "test-ts":
		return app.test_ts()
	case "test-ts-other":
		return app.test_ts_other()
	case "test-ts-framework":
		return app.test_ts_framework()
	case "build-ts":
		return app.build_ts()
	case "test-go":
		return app.test_go()
	case "test-go-other":
		return app.test_go_other()
	case "test-go-framework":
		return app.test_go_framework()
	case "test-root-go":
		return app.test_root_go()
	case "test-internal-go":
		return app.test_internal_go()
	case "test-kit":
		return app.test_kit()
	case "test-lab":
		return app.test_lab()
	case "test-docs":
		return app.test_docs()
	case "test-framework":
		return app.test_framework(rest)
	case "test-framework-prod":
		return app.test_framework_prod(rest)
	case "test-framework-dev":
		return app.test_framework_dev(rest)
	case "stress":
		return app.stress(rest)
	case "stress-other":
		return app.stress_other(rest)
	case "stress-framework":
		return app.stress_framework(rest)
	case "build-framework-fixture":
		return app.build_framework()
	case "serve-framework-dev":
		return app.serve_framework_dev(rest)
	case "inspect-framework-artifacts":
		return app.inspect_framework(rest)
	case "clean-framework-artifacts":
		return app.clean_framework()
	case "docs-dev":
		return app.docs_dev()
	case "build-docs":
		return app.build_docs()
	case "gate":
		return app.gate()
	case "prepare-release":
		return app.prepare_release(rest)
	case "publish-go":
		return app.publish_go(rest)
	case "create-local-test":
		return app.create_local_test()
	default:
		return fmt.Errorf("unknown command %q; run `go run ./internal/cmd/maint help`", command)
	}
}

func (app *maint_app) parse_global_flags(args []string) (string, []string, error) {
	for len(args) > 0 {
		switch args[0] {
		case "--yes", "-y":
			app.yes = true
			args = args[1:]
		case "--dry-run":
			app.dry_run = true
			args = args[1:]
		case "--verbose", "-v":
			app.verbose = true
			args = args[1:]
		default:
			command := args[0]
			return command, args[1:], nil
		}
	}
	return "", nil, nil
}

func (app maint_app) print_help() {
	fmt.Println("Vorma maintenance CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./internal/cmd/maint [--yes] [--dry-run] <command> [options]")
	fmt.Println()
	fmt.Println("Everyday:")
	fmt.Println("  install-js              install JavaScript dependencies in every workspace")
	fmt.Println("  clean-js                remove JavaScript dependencies after confirmation")
	fmt.Println("  fmt-ts                  format TypeScript")
	fmt.Println("  lint-ts                 lint TypeScript")
	fmt.Println("  typecheck-ts            typecheck TypeScript projects")
	fmt.Println("  typecheck-ts-other      typecheck non-framework TypeScript projects")
	fmt.Println("  typecheck-ts-framework  typecheck framework TypeScript projects")
	fmt.Println("  test-ts                 run all TypeScript package tests")
	fmt.Println("  test-ts-other           run non-framework TypeScript package tests")
	fmt.Println("  test-ts-framework       run framework TypeScript package tests")
	fmt.Println("  build-ts                build npm package output")
	fmt.Println("  test-go                 run all Go test domains")
	fmt.Println("  test-go-other           run non-framework Go test domains")
	fmt.Println("  test-root-go            run the root Go module with ./...")
	fmt.Println("  test-internal-go        run internal Go package tests")
	fmt.Println("  test-kit                run kit Go package tests")
	fmt.Println("  test-lab                run kit/lab Go package tests")
	fmt.Println("  test-docs               run docs Go module tests")
	fmt.Println("  test-go-framework       run framework Go module tests")
	fmt.Println("  test-framework          run framework Go/TS/Vitest/browser suites")
	fmt.Println("  build-framework-fixture build framework fixture output")
	fmt.Println("  serve-framework-dev     serve a framework dev fixture")
	fmt.Println("  docs-dev                run the docs development server")
	fmt.Println("  build-docs              build the docs site")
	fmt.Println("  create-local-test       run create-vorma against a local test directory")
	fmt.Println()
	fmt.Println("Stress and debug:")
	fmt.Println("  stress                  repeat non-framework and framework stress partitions")
	fmt.Println("  stress-other            repeat the non-framework stress partition")
	fmt.Println("  stress-framework        repeat framework Go/TS/Vitest/browser suites")
	fmt.Println("  inspect-framework-artifacts")
	fmt.Println("                           inspect framework artifacts")
	fmt.Println("  clean-framework-artifacts")
	fmt.Println("                           remove framework artifacts after confirmation")
	fmt.Println()
	fmt.Println("Release:")
	fmt.Println("  gate                    run the full repo confidence workflow")
	fmt.Println(
		"  prepare-release         update versions, run gate, and print npm publish commands",
	)
	fmt.Println("  publish-go              publish the prepared Go module tag")
}

func (app maint_app) command_error(command string, err error) error {
	if err == nil {
		return nil
	}
	if strings.TrimSpace(command) == "" {
		return err
	}
	return errors.Join(fmt.Errorf("%s failed", command), err)
}
