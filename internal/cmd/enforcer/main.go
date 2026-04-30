package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/vormadev/vorma/internal/cmd/internal/tooling"
	"github.com/vormadev/vorma/kit/enum"
)

type enforcer_app struct {
	tooling.App
}

type action_name string

var action_enum = enum.New[action_name, string](struct {
	Install   action_name
	Format    action_name
	Lint      action_name
	Fix       action_name
	Typecheck action_name
	Test      action_name
	Build     action_name
	Stress    action_name
	Gate      action_name
}{
	Install:   "install",
	Format:    "fmt",
	Lint:      "lint",
	Fix:       "fix",
	Typecheck: "typecheck",
	Test:      "test",
	Build:     "build",
	Stress:    "stress",
	Gate:      "gate",
})

var action_vals = action_enum.Get()

type lang_scope string

var lang_scope_enum = enum.New[lang_scope, string](struct {
	All lang_scope
	Go  lang_scope
	Ts  lang_scope
}{
	All: "all",
	Go:  "go",
	Ts:  "ts",
})

var lang_scope_vals = lang_scope_enum.Get()

type target_scope string

var target_scope_enum = enum.New[target_scope, string](struct {
	All   target_scope
	Fw    target_scope
	Other target_scope
}{
	All:   "all",
	Fw:    "fw",
	Other: "other",
})

var target_scope_vals = target_scope_enum.Get()

type enforcer_request struct {
	actions   []action_name
	lang      lang_scope
	scope     target_scope
	intensity int
}

func main() {
	app := enforcer_app{}
	if err := app.run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "enforcer:", err)
		os.Exit(1)
	}
}

func (app *enforcer_app) run(args []string) error {
	tool_app, command, rest, err := tooling.Args(args).NewApp()
	if err != nil {
		return err
	}
	app.App = tool_app

	if command == "" || command == "help" {
		app.print_help()
		return nil
	}
	if command == "clean-js" {
		return app.require_no_args_then(command, rest, app.clean_js)
	}

	req, err := app.parse_request(append([]string{command}, rest...))
	if err != nil {
		return err
	}
	return app.run_request(req)
}

func (app *enforcer_app) parse_request(args []string) (enforcer_request, error) {
	req := enforcer_request{
		lang:  lang_scope_vals.All,
		scope: target_scope_vals.All,
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--yes" || arg == "-y":
			app.Yes = true
		case arg == "--dry-run":
			app.DryRun = true
		case arg == "--verbose" || arg == "-v":
			app.Verbose = true
		case arg == "--lang":
			i++
			if i >= len(args) {
				return req, fmt.Errorf("--lang requires a value")
			}
			lang, err := parse_lang_scope(args[i])
			if err != nil {
				return req, err
			}
			req.lang = lang
		case strings.HasPrefix(arg, "--lang="):
			lang, err := parse_lang_scope(strings.TrimPrefix(arg, "--lang="))
			if err != nil {
				return req, err
			}
			req.lang = lang
		case arg == "--scope":
			i++
			if i >= len(args) {
				return req, fmt.Errorf("--scope requires a value")
			}
			scope, err := parse_target_scope(args[i])
			if err != nil {
				return req, err
			}
			req.scope = scope
		case strings.HasPrefix(arg, "--scope="):
			scope, err := parse_target_scope(strings.TrimPrefix(arg, "--scope="))
			if err != nil {
				return req, err
			}
			req.scope = scope
		case arg == "--intensity":
			i++
			if i >= len(args) {
				return req, fmt.Errorf("--intensity requires a value")
			}
			intensity, err := parse_intensity(args[i])
			if err != nil {
				return req, err
			}
			req.intensity = intensity
		case strings.HasPrefix(arg, "--intensity="):
			intensity, err := parse_intensity(strings.TrimPrefix(arg, "--intensity="))
			if err != nil {
				return req, err
			}
			req.intensity = intensity
		case strings.HasPrefix(arg, "-"):
			return req, fmt.Errorf("unknown option %q", arg)
		default:
			action, err := parse_action(arg)
			if err != nil {
				return req, err
			}
			req.actions = append(req.actions, action)
		}
	}
	if len(req.actions) == 0 {
		return req, fmt.Errorf("at least one action is required")
	}
	return req, nil
}

func parse_action(value string) (action_name, error) {
	action, ok := action_enum.Parse(value)
	if ok {
		return action, nil
	}
	return "", fmt.Errorf(
		"unknown action %q; use one of: %s",
		value,
		strings.Join(action_enum.NativeValues(), ", "),
	)
}

func parse_lang_scope(value string) (lang_scope, error) {
	if value == "" {
		return lang_scope_vals.All, nil
	}
	if value == "js" {
		return lang_scope_vals.Ts, nil
	}
	lang, ok := lang_scope_enum.Parse(value)
	if ok {
		return lang, nil
	}
	return "", fmt.Errorf(
		"unknown language %q; use one of: %s",
		value,
		strings.Join(lang_scope_enum.NativeValues(), ", "),
	)
}

func parse_target_scope(value string) (target_scope, error) {
	if value == "" {
		return target_scope_vals.All, nil
	}
	if value == "framework" {
		return target_scope_vals.Fw, nil
	}
	scope, ok := target_scope_enum.Parse(value)
	if ok {
		return scope, nil
	}
	return "", fmt.Errorf(
		"unknown scope %q; use one of: %s",
		value,
		strings.Join(target_scope_enum.NativeValues(), ", "),
	)
}

func parse_intensity(value string) (int, error) {
	intensity, err := strconv.Atoi(value)
	if err != nil || intensity <= 0 {
		return 0, fmt.Errorf("--intensity requires a positive integer")
	}
	return intensity, nil
}

func (app enforcer_app) require_no_args_then(
	command string,
	args []string,
	run func() error,
) error {
	if len(args) > 0 {
		return fmt.Errorf("%s does not take arguments", command)
	}
	return run()
}

func (app enforcer_app) print_help() {
	fmt.Println("Vorma enforcer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./internal/cmd/enforcer [--yes] [--dry-run] <action...> [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --lang all|go|ts      language scope, default all")
	fmt.Println("  --scope all|fw|other  target scope, default all")
	fmt.Println("  --intensity N         positive repeat count for stress")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  install    enforce Go and TypeScript prerequisites")
	fmt.Println("  fmt        format source")
	fmt.Println("  lint       lint source")
	fmt.Println("  fix        apply safe fixes")
	fmt.Println("  typecheck  compile/typecheck source")
	fmt.Println("  test       run tests")
	fmt.Println("  build      build outputs")
	fmt.Println("  stress     repeat stress checks")
	fmt.Println("  gate       run install, fix, fmt, lint, typecheck, build, and test")
	fmt.Println("  clean-js   remove every node_modules directory after confirmation")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run ./internal/cmd/enforcer fmt lint --lang go --scope fw")
	fmt.Println("  go run ./internal/cmd/enforcer gate --scope other")
}
