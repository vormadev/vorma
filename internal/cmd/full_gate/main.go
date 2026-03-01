package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vormadev/vorma/internal/coalescepath"
	"github.com/vormadev/vorma/lab/coalescecmd"
)

type releaseGateStage struct {
	stageName             string
	targetNames           []string
	dependencyTargetNames []string
}

type gateTargetResult struct {
	targetName    string
	output        string
	executionErr  error
	exitCode      int
	duration      time.Duration
	skippedReason string
}

var releaseGateStages = []releaseGateStage{
	{
		stageName: "base-checks",
		targetNames: []string{
			"gotest",
			"staticcheck",
		},
	},
	{
		stageName: "typescript-reset",
		targetNames: []string{
			"tsreset",
		},
	},
	{
		stageName: "typescript",
		targetNames: []string{
			"tstest-source",
			"tslint",
			"tscheck",
			"npmbuild",
		},
		dependencyTargetNames: []string{"tsreset"},
	},
	{
		stageName: "distribution-and-e2e",
		targetNames: []string{
			"tstest-dist",
			"e2e-test",
		},
		dependencyTargetNames: []string{"npmbuild"},
	},
}

func main() {
	os.Exit(runMain())
}

func runMain() int {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "release gate command does not take arguments")
		return 2
	}

	commandExitCode := 0
	coalesceError := coalescecmd.Run(coalescecmd.Options{
		Key:                    coalescepath.FullGateCommandKey,
		StateRootDirectoryPath: coalescepath.StateRootDirectoryPath,
		Func: func() error {
			commandExitCode = runReleaseGateProcess()
			return nil
		},
	})
	if coalesceError != nil {
		fmt.Fprintf(os.Stderr, "full gate coalesce failed: %v\n", coalesceError)
		return 1
	}
	return commandExitCode
}

func runReleaseGateProcess() int {
	maxParallelTargets := resolveMaxParallelTargets()
	resultsByTargetName := make(map[string]gateTargetResult)

	for _, stage := range releaseGateStages {
		dependencyFailureReason := findDependencyFailureReason(stage, resultsByTargetName)
		if dependencyFailureReason != "" {
			skippedResults := runSkippedStage(stage, dependencyFailureReason)
			printStageResults(stage, skippedResults)
			for targetName, targetResult := range skippedResults {
				resultsByTargetName[targetName] = targetResult
			}
			continue
		}

		stageResults := runStageInParallel(stage, maxParallelTargets)
		printStageResults(stage, stageResults)
		for targetName, targetResult := range stageResults {
			resultsByTargetName[targetName] = targetResult
		}
	}

	failedTargetResults := collectFailedTargetResultsInOrder(resultsByTargetName)
	if len(failedTargetResults) > 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "RELEASE GATE: FAIL")
		for _, failedTargetResult := range failedTargetResults {
			fmt.Fprintf(
				os.Stderr,
				"failed target: %s (exit=%d)\n",
				failedTargetResult.targetName,
				failedTargetResult.exitCode,
			)
		}
		return failedTargetResults[0].exitCode
	}

	skippedTargetResults := collectSkippedTargetResultsInOrder(resultsByTargetName)
	if len(skippedTargetResults) > 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "RELEASE GATE: FAIL")
		for _, skippedTargetResult := range skippedTargetResults {
			fmt.Fprintf(
				os.Stderr,
				"skipped target: %s (%s)\n",
				skippedTargetResult.targetName,
				skippedTargetResult.skippedReason,
			)
		}
		return 1
	}

	fmt.Println()
	fmt.Println("RELEASE GATE: PASS")
	return 0
}

func resolveMaxParallelTargets() int {
	const defaultMaxParallelTargets = 3
	const minimumMaxParallelTargets = 1
	const maximumMaxParallelTargets = 4
	parallelTargetsSetting := strings.TrimSpace(os.Getenv("VORMA_FULL_GATE_PARALLEL"))
	if parallelTargetsSetting != "" {
		parsedParallelTargets, parseErr := strconv.Atoi(parallelTargetsSetting)
		if parseErr != nil || parsedParallelTargets < minimumMaxParallelTargets {
			fmt.Fprintf(
				os.Stderr,
				"invalid VORMA_FULL_GATE_PARALLEL=%q; using default=%d\n",
				parallelTargetsSetting,
				defaultMaxParallelTargets,
			)
			return defaultMaxParallelTargets
		}
		if parsedParallelTargets > maximumMaxParallelTargets {
			return maximumMaxParallelTargets
		}
		return parsedParallelTargets
	}

	cpuCount := runtime.NumCPU()
	if cpuCount < minimumMaxParallelTargets {
		return minimumMaxParallelTargets
	}
	if cpuCount < defaultMaxParallelTargets {
		return cpuCount
	}
	return defaultMaxParallelTargets
}

func runStageInParallel(
	stage releaseGateStage,
	maxParallelTargets int,
) map[string]gateTargetResult {
	resultsByTargetName := make(map[string]gateTargetResult, len(stage.targetNames))
	var resultsLock sync.Mutex
	var waitGroup sync.WaitGroup
	parallelismLimiter := make(chan struct{}, maxParallelTargets)
	targetResultsChannel := make(chan gateTargetResult, len(stage.targetNames))

	fmt.Printf(
		">>> running stage %q with %d target(s), parallelism=%d\n",
		stage.stageName,
		len(stage.targetNames),
		maxParallelTargets,
	)

	runningTargetsStartedAt := make(map[string]time.Time)
	var runningTargetsLock sync.Mutex
	addRunningTarget := func(targetName string) {
		runningTargetsLock.Lock()
		runningTargetsStartedAt[targetName] = time.Now()
		runningTargetsLock.Unlock()
		fmt.Printf("[start] %s\n", targetName)
	}
	removeRunningTarget := func(targetResult gateTargetResult) {
		runningTargetsLock.Lock()
		delete(runningTargetsStartedAt, targetResult.targetName)
		runningTargetsLock.Unlock()
		if targetResult.executionErr != nil {
			fmt.Printf(
				"[done] %s in %s (failed, exit=%d)\n",
				targetResult.targetName,
				targetResult.duration.Round(time.Millisecond),
				targetResult.exitCode,
			)
			return
		}
		fmt.Printf(
			"[done] %s in %s\n",
			targetResult.targetName,
			targetResult.duration.Round(time.Millisecond),
		)
	}

	heartbeatDoneChannel := make(chan struct{})
	heartbeatTicker := time.NewTicker(15 * time.Second)
	go func() {
		defer heartbeatTicker.Stop()
		for {
			select {
			case <-heartbeatDoneChannel:
				return
			case <-heartbeatTicker.C:
				runningTargetsLock.Lock()
				if len(runningTargetsStartedAt) == 0 {
					runningTargetsLock.Unlock()
					continue
				}
				runningTargetDescriptions := make([]string, 0, len(runningTargetsStartedAt))
				for targetName, startedAt := range runningTargetsStartedAt {
					runningTargetDescriptions = append(
						runningTargetDescriptions,
						fmt.Sprintf(
							"%s(%s)",
							targetName,
							time.Since(startedAt).Round(time.Second),
						),
					)
				}
				runningTargetsLock.Unlock()
				sort.Strings(runningTargetDescriptions)
				fmt.Printf(
					"[progress] stage %q still running: %s\n",
					stage.stageName,
					strings.Join(runningTargetDescriptions, ", "),
				)
			}
		}
	}()

	for _, targetName := range stage.targetNames {
		targetName := targetName
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			parallelismLimiter <- struct{}{}
			addRunningTarget(targetName)
			targetResult := runMakeTarget(targetName)
			<-parallelismLimiter
			targetResultsChannel <- targetResult
		}()
	}

	for range stage.targetNames {
		targetResult := <-targetResultsChannel
		removeRunningTarget(targetResult)
		resultsLock.Lock()
		resultsByTargetName[targetResult.targetName] = targetResult
		resultsLock.Unlock()
	}

	waitGroup.Wait()
	close(heartbeatDoneChannel)
	return resultsByTargetName
}

func runSkippedStage(
	stage releaseGateStage,
	dependencyFailureReason string,
) map[string]gateTargetResult {
	resultsByTargetName := make(map[string]gateTargetResult, len(stage.targetNames))
	for _, targetName := range stage.targetNames {
		resultsByTargetName[targetName] = gateTargetResult{
			targetName:    targetName,
			skippedReason: dependencyFailureReason,
		}
	}
	return resultsByTargetName
}

func runMakeTarget(targetName string) gateTargetResult {
	startedAt := time.Now()
	command := exec.Command("make", targetName)
	command.Stdin = os.Stdin
	var combinedOutput bytes.Buffer
	command.Stdout = &combinedOutput
	command.Stderr = &combinedOutput
	commandErr := command.Run()
	return gateTargetResult{
		targetName:   targetName,
		output:       combinedOutput.String(),
		executionErr: commandErr,
		exitCode:     exitCodeFromCommandError(commandErr),
		duration:     time.Since(startedAt),
	}
}

func findDependencyFailureReason(
	stage releaseGateStage,
	resultsByTargetName map[string]gateTargetResult,
) string {
	for _, dependencyTargetName := range stage.dependencyTargetNames {
		dependencyResult, foundDependencyResult := resultsByTargetName[dependencyTargetName]
		if !foundDependencyResult {
			return fmt.Sprintf(
				"dependency %s did not run",
				dependencyTargetName,
			)
		}
		if dependencyResult.executionErr != nil {
			return fmt.Sprintf(
				"dependency %s failed",
				dependencyTargetName,
			)
		}
		if dependencyResult.skippedReason != "" {
			return fmt.Sprintf(
				"dependency %s was skipped: %s",
				dependencyTargetName,
				dependencyResult.skippedReason,
			)
		}
	}
	return ""
}

func printStageResults(
	stage releaseGateStage,
	resultsByTargetName map[string]gateTargetResult,
) {
	fmt.Printf("\n=== stage: %s ===\n", stage.stageName)
	for _, targetName := range stage.targetNames {
		targetResult := resultsByTargetName[targetName]
		fmt.Printf("\n--- target: %s (%s) ---\n", targetResult.targetName, targetResult.duration.Round(time.Millisecond))
		if targetResult.skippedReason != "" {
			fmt.Printf("skipped: %s\n", targetResult.skippedReason)
			continue
		}
		trimmedOutput := strings.TrimSpace(targetResult.output)
		if trimmedOutput == "" {
			fmt.Println("(no output)")
		} else {
			fmt.Println(trimmedOutput)
		}
		if targetResult.executionErr != nil {
			fmt.Printf("target failed: %s (exit=%d)\n", targetResult.targetName, targetResult.exitCode)
		}
	}
}

func collectFailedTargetResultsInOrder(
	resultsByTargetName map[string]gateTargetResult,
) []gateTargetResult {
	failedTargetResults := make([]gateTargetResult, 0)
	for _, stage := range releaseGateStages {
		for _, targetName := range stage.targetNames {
			targetResult, found := resultsByTargetName[targetName]
			if !found {
				continue
			}
			if targetResult.executionErr != nil {
				failedTargetResults = append(failedTargetResults, targetResult)
			}
		}
	}
	return failedTargetResults
}

func collectSkippedTargetResultsInOrder(
	resultsByTargetName map[string]gateTargetResult,
) []gateTargetResult {
	skippedTargetResults := make([]gateTargetResult, 0)
	for _, stage := range releaseGateStages {
		for _, targetName := range stage.targetNames {
			targetResult, found := resultsByTargetName[targetName]
			if !found {
				continue
			}
			if targetResult.skippedReason != "" {
				skippedTargetResults = append(skippedTargetResults, targetResult)
			}
		}
	}
	return skippedTargetResults
}

func exitCodeFromCommandError(commandErr error) int {
	if commandErr == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(commandErr, &exitErr) {
		commandExitCode := exitErr.ExitCode()
		if commandExitCode != 0 {
			return commandExitCode
		}
	}
	return 1
}
