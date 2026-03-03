package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/coalescepath"
	"github.com/vormadev/vorma/lab/coalescecmd"
)

var errInvalidArguments = errors.New("invalid arguments")

type commandInvocation struct {
	subcommand string
}

type runCommandInput struct {
	workingDirectoryPath string
	commandPath          string
	commandArguments     []string
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
	fmt.Fprintf(os.Stderr, "tsstate command error: %v\n", runError)
	os.Exit(exitCode)
}

func run(rawArguments []string) error {
	invocation, parseError := parseCommandInvocation(rawArguments)
	if parseError != nil {
		return parseError
	}

	repositoryRootPath, resolveRepositoryRootError := filepath.Abs(".")
	if resolveRepositoryRootError != nil {
		return fmt.Errorf(
			"resolve repository root path: %w",
			resolveRepositoryRootError,
		)
	}

	switch invocation.subcommand {
	case "nuke-node-modules":
		return coalescecmd.Run(coalescecmd.Options{
			Key:                    coalescepath.TSStateNukeNodeModulesCommandKey,
			FailIfRunning:          coalescepath.TSStateNukeNodeModulesFailIfRunningKeys,
			StateRootDirectoryPath: coalescepath.StateRootDirectoryPath,
			Func: func() error {
				return nukeNodeModulesDirectories(repositoryRootPath)
			},
		})
	case "install":
		return coalescecmd.Run(coalescecmd.Options{
			Key:                    coalescepath.TSStateInstallCommandKey,
			FailIfRunning:          coalescepath.TSStateInstallFailIfRunningKeys,
			StateRootDirectoryPath: coalescepath.StateRootDirectoryPath,
			Func: func() error {
				return runInstallSequence(repositoryRootPath)
			},
		})
	case "reset":
		return coalescecmd.Run(coalescecmd.Options{
			Key:                    coalescepath.TSStateResetCommandKey,
			FailIfRunning:          coalescepath.TSStateResetFailIfRunningKeys,
			StateRootDirectoryPath: coalescepath.StateRootDirectoryPath,
			Func: func() error {
				if nukeError := nukeNodeModulesDirectories(repositoryRootPath); nukeError != nil {
					return nukeError
				}
				return runInstallSequence(repositoryRootPath)
			},
		})
	default:
		return fmt.Errorf(
			"%w: unsupported subcommand %q",
			errInvalidArguments,
			invocation.subcommand,
		)
	}
}

func parseCommandInvocation(rawArguments []string) (commandInvocation, error) {
	if len(rawArguments) == 0 {
		return commandInvocation{}, fmt.Errorf(
			"%w: expected subcommand: nuke-node-modules|install|reset",
			errInvalidArguments,
		)
	}
	if len(rawArguments) > 1 {
		return commandInvocation{}, fmt.Errorf(
			"%w: unexpected extra arguments: %s",
			errInvalidArguments,
			strings.Join(rawArguments[1:], ", "),
		)
	}

	subcommand := strings.TrimSpace(rawArguments[0])
	if subcommand == "" {
		return commandInvocation{}, fmt.Errorf("%w: subcommand is empty", errInvalidArguments)
	}

	return commandInvocation{subcommand: subcommand}, nil
}

func nukeNodeModulesDirectories(repositoryRootPath string) error {
	removedDirectoryCount := 0
	walkError := filepath.WalkDir(
		repositoryRootPath,
		func(path string, directoryEntry fs.DirEntry, walkError error) error {
			if walkError != nil {
				return walkError
			}
			if !directoryEntry.IsDir() {
				return nil
			}
			if directoryEntry.Name() != "node_modules" {
				return nil
			}

			if removeError := os.RemoveAll(path); removeError != nil {
				return fmt.Errorf(
					"remove node_modules directory %q: %w",
					path,
					removeError,
				)
			}
			removedDirectoryCount += 1
			return filepath.SkipDir
		},
	)
	if walkError != nil {
		return fmt.Errorf("walk repository for node_modules removal: %w", walkError)
	}

	fmt.Fprintf(
		os.Stderr,
		"tsstate: removed %d node_modules director%s\n",
		removedDirectoryCount,
		pluralSuffix(removedDirectoryCount, "y", "ies"),
	)
	return nil
}

func runInstallSequence(repositoryRootPath string) error {
	if rootInstallError := runCommand(runCommandInput{
		workingDirectoryPath: repositoryRootPath,
		commandPath:          "pnpm",
		commandArguments:     []string{"i"},
	}); rootInstallError != nil {
		return rootInstallError
	}

	createDirectoryPath := filepath.Join(repositoryRootPath, "typescript/vorma/create")
	if createInstallError := runCommand(runCommandInput{
		workingDirectoryPath: createDirectoryPath,
		commandPath:          "pnpm",
		commandArguments:     []string{"i"},
	}); createInstallError != nil {
		return createInstallError
	}

	e2eDirectoryPath := filepath.Join(repositoryRootPath, "internal/e2e")
	if e2eInstallError := runCommand(runCommandInput{
		workingDirectoryPath: e2eDirectoryPath,
		commandPath:          "pnpm",
		commandArguments:     []string{"i"},
	}); e2eInstallError != nil {
		return e2eInstallError
	}

	return nil
}

func runCommand(input runCommandInput) error {
	if strings.TrimSpace(input.commandPath) == "" {
		return errors.New("command path is required")
	}

	command := exec.Command(input.commandPath, input.commandArguments...)
	command.Dir = input.workingDirectoryPath
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	fmt.Fprintf(
		os.Stderr,
		"tsstate: running command in %s: %s %s\n",
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

func pluralSuffix(count int, singularSuffix string, pluralSuffix string) string {
	if count == 1 {
		return singularSuffix
	}
	return pluralSuffix
}
