package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vormadev/vorma/internal/coalescepath"
	"github.com/vormadev/vorma/lab/coalescecmd"
)

const (
	e2eWorkingDirectoryPath = "./internal/e2e"
)

var errInvalidArguments = errors.New("invalid arguments")

type commandInvocation struct {
	subcommand          string
	playwrightArguments []string
}

type runCommandInput struct {
	workingDirectoryPath string
	commandPath          string
	commandArguments     []string
	environment          []string
}

func main() {
	runError := run(os.Args[1:])
	if runError == nil {
		return
	}

	exitCode := 1
	if errors.Is(runError, errInvalidArguments) {
		exitCode = 2
	}
	fmt.Fprintf(os.Stderr, "e2e command error: %v\n", runError)
	os.Exit(exitCode)
}

func run(rawArguments []string) error {
	invocation, parseError := parseCommandInvocation(rawArguments)
	if parseError != nil {
		return parseError
	}

	switch invocation.subcommand {
	case "install":
		return coalescecmd.Run(coalescecmd.Options{
			Key:                    coalescepath.E2EInstallCommandKey,
			FailIfRunning:          coalescepath.E2EInstallFailIfRunningKeys,
			StateRootDirectoryPath: coalescepath.StateRootDirectoryPath,
			Func: func() error {
				return runCommand(runCommandInput{
					workingDirectoryPath: e2eWorkingDirectoryPath,
					commandPath:          "pnpm",
					commandArguments:     []string{"i"},
				})
			},
		})
	case "install-browsers":
		return coalescecmd.Run(coalescecmd.Options{
			Key:                    coalescepath.E2EInstallBrowsersChromiumCommandKey,
			FailIfRunning:          coalescepath.E2EInstallBrowsersChromiumFailIfRunningKeys,
			StateRootDirectoryPath: coalescepath.StateRootDirectoryPath,
			Func: func() error {
				return runCommand(runCommandInput{
					workingDirectoryPath: e2eWorkingDirectoryPath,
					commandPath:          "pnpm",
					commandArguments: []string{
						"exec",
						"playwright",
						"install",
						"chromium",
					},
				})
			},
		})
	case "test":
		return runPlaywrightTest(
			runPlaywrightTestInput{
				e2eMode:             "",
				playwrightArguments: invocation.playwrightArguments,
			},
		)
	case "test-dev":
		return runPlaywrightTest(
			runPlaywrightTestInput{
				e2eMode:             "dev",
				playwrightArguments: invocation.playwrightArguments,
			},
		)
	case "test-prod":
		return runPlaywrightTest(
			runPlaywrightTestInput{
				e2eMode:             "prod",
				playwrightArguments: invocation.playwrightArguments,
			},
		)
	default:
		return fmt.Errorf("%w: unsupported subcommand %q", errInvalidArguments, invocation.subcommand)
	}
}

func parseCommandInvocation(rawArguments []string) (commandInvocation, error) {
	if len(rawArguments) == 0 {
		return commandInvocation{}, fmt.Errorf(
			"%w: expected subcommand: install|install-browsers|test|test-dev|test-prod",
			errInvalidArguments,
		)
	}

	subcommand := strings.TrimSpace(rawArguments[0])
	if subcommand == "" {
		return commandInvocation{}, fmt.Errorf("%w: subcommand is empty", errInvalidArguments)
	}

	invocation := commandInvocation{
		subcommand:          subcommand,
		playwrightArguments: append([]string{}, rawArguments[1:]...),
	}
	return invocation, nil
}

type runPlaywrightTestInput struct {
	e2eMode             string
	playwrightArguments []string
}

func runPlaywrightTest(input runPlaywrightTestInput) error {
	key := buildPlaywrightCoalescingKey(input)

	return coalescecmd.Run(coalescecmd.Options{
		Key:                    key,
		FailIfRunning:          coalescepath.E2ETestFailIfRunningKeys,
		StateRootDirectoryPath: coalescepath.StateRootDirectoryPath,
		Func: func() error {
			commandArguments := []string{
				"exec",
				"playwright",
				"test",
				"--config",
				"./playwright.config.ts",
			}
			commandArguments = append(commandArguments, input.playwrightArguments...)

			commandEnvironment := append([]string{}, os.Environ()...)
			if input.e2eMode != "" {
				commandEnvironment = append(
					commandEnvironment,
					"VORMA_E2E_MODE="+input.e2eMode,
				)
			}

			return runCommand(runCommandInput{
				workingDirectoryPath: e2eWorkingDirectoryPath,
				commandPath:          "pnpm",
				commandArguments:     commandArguments,
				environment:          commandEnvironment,
			})
		},
	})
}

func buildPlaywrightCoalescingKey(input runPlaywrightTestInput) string {
	normalizedMode := strings.TrimSpace(input.e2eMode)
	if normalizedMode == "" {
		normalizedMode = "default"
	}

	keyPayloadBytes, _ := json.Marshal(struct {
		E2EMode             string   `json:"e2e_mode"`
		PlaywrightArguments []string `json:"playwright_arguments"`
	}{
		E2EMode:             normalizedMode,
		PlaywrightArguments: input.playwrightArguments,
	})
	hashBytes := sha256.Sum256(keyPayloadBytes)
	return fmt.Sprintf(
		"internal-cmd-e2e-test-%s-%s",
		normalizedMode,
		hex.EncodeToString(hashBytes[:6]),
	)
}

func runCommand(input runCommandInput) error {
	if strings.TrimSpace(input.commandPath) == "" {
		return errors.New("command path is required")
	}

	command := exec.Command(input.commandPath, input.commandArguments...)
	command.Dir = input.workingDirectoryPath
	if len(input.environment) > 0 {
		command.Env = append([]string{}, input.environment...)
	}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	fmt.Fprintf(
		os.Stderr,
		"e2e: running command in %s: %s %s\n",
		input.workingDirectoryPath,
		input.commandPath,
		strings.Join(input.commandArguments, " "),
	)

	if runError := command.Run(); runError != nil {
		return fmt.Errorf(
			"run command %q with args %q: %w",
			input.commandPath,
			input.commandArguments,
			runError,
		)
	}
	return nil
}
