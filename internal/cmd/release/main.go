package main

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/internal/cmd/internal/tooling"
)

type release_app struct {
	tooling.App
}

func main() {
	app := release_app{}
	if err := app.run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func (app *release_app) run(args []string) error {
	tool_app, command, rest, err := tooling.Args(args).NewApp()
	if err != nil {
		return err
	}
	app.App = tool_app
	switch command {
	case "", "help":
		app.print_help()
		return nil
	case "prepare":
		return app.prepare_release(rest)
	case "publish-go":
		return app.publish_go(rest)
	default:
		return fmt.Errorf("unknown command %q; run `go run ./internal/cmd/release help`", command)
	}
}

func (app release_app) require_no_args(command string, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("%s does not take arguments", command)
	}
	return nil
}

func (app release_app) print_help() {
	fmt.Println("Vorma release")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./internal/cmd/release [--yes] [--dry-run] <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println(
		"  prepare      update release versions, run enforcer gate, and print npm publish commands",
	)
	fmt.Println(
		"  publish-go   verify npm publish, commit/push the release, then publish the Go tag",
	)
}
