// Package vitecmd provides build/dev command orchestration for Vite.
// Keep this package out of runtime imports.
package vitecmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/lab/viteutil"
)

var log = colorlog.New("viteutil")
var initPort = viteutil.InitPort

type BuildCtx struct {
	mu             *sync.Mutex
	cmd            *exec.Cmd
	opts           *BuildCtxOptions
	port           int
	waitInProgress bool
	waitDone       chan struct{}
}

func (c *BuildCtx) Port() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.port
}

type BuildCtxOptions struct {
	// required -- e.g., "npx", "pnpm", "yarn", "bunx", etc.
	JSPackageManagerBaseCmd string
	// optional -- used for monorepos that need to run commands from ancestor directories
	JSPackageManagerCmdDir string
	// required
	OutDir string
	// required
	ManifestOut string
	// optional -- default is 5199
	DefaultPort int
	// optional
	ViteConfigFile string
}

func viteConfigFileArgumentPath(
	viteConfigFile string,
	commandWorkingDirectory string,
) string {
	trimmedViteConfigFile := strings.TrimSpace(viteConfigFile)
	if trimmedViteConfigFile == "" {
		return ""
	}
	if !filepath.IsAbs(trimmedViteConfigFile) {
		return trimmedViteConfigFile
	}
	trimmedCommandWorkingDirectory := strings.TrimSpace(commandWorkingDirectory)
	if trimmedCommandWorkingDirectory == "" {
		return filepath.Clean(trimmedViteConfigFile)
	}
	relativeViteConfigFile, relativeViteConfigFileError := filepath.Rel(
		trimmedCommandWorkingDirectory,
		trimmedViteConfigFile,
	)
	if relativeViteConfigFileError != nil {
		return filepath.Clean(trimmedViteConfigFile)
	}
	normalizedRelativeViteConfigFile := filepath.ToSlash(
		filepath.Clean(relativeViteConfigFile),
	)
	if normalizedRelativeViteConfigFile == "." || normalizedRelativeViteConfigFile == "" {
		return filepath.Clean(trimmedViteConfigFile)
	}
	if strings.HasPrefix(normalizedRelativeViteConfigFile, ".") {
		return normalizedRelativeViteConfigFile
	}
	return "./" + normalizedRelativeViteConfigFile
}

func NewBuildCtx(opts *BuildCtxOptions) *BuildCtx {
	if opts == nil {
		opts = &BuildCtxOptions{}
	}

	port := opts.DefaultPort
	if port == 0 {
		port = 5199
	}
	return &BuildCtx{
		mu:   &sync.Mutex{},
		opts: opts,
		port: port,
	}
}

func (c *BuildCtx) prep_cmd() error {
	split_cmd := strings.Fields(c.opts.JSPackageManagerBaseCmd)
	if len(split_cmd) == 0 {
		return fmt.Errorf("JSPackageManagerBaseCmd is required")
	}

	c.cmd = exec.Command(split_cmd[0], split_cmd[1:]...)
	c.cmd.Stdout, c.cmd.Stderr = os.Stdout, os.Stderr
	if runtime.GOOS != "windows" {
		// Launch Vite wrapper command in its own process group so stop/restart can
		// terminate the whole wrapper-child tree atomically.
		c.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	if c.opts.JSPackageManagerCmdDir != "" {
		c.cmd.Dir = c.opts.JSPackageManagerCmdDir
	}

	return nil
}

func (c *BuildCtx) DevBuild() error {
	c.mu.Lock()
	err := c.terminateProcessLockedAndWait()
	if err != nil {
		c.mu.Unlock()
		log.Warn(fmt.Sprintf("DevBuild: Error terminating vite process: %s", err))
		return err
	}

	if err := c.prep_cmd(); err != nil {
		c.mu.Unlock()
		return err
	}

	c.port, err = initPort(c.port)
	if err != nil {
		c.mu.Unlock()
		log.Error(fmt.Sprintf("Error initializing vite port: %s", err))
		return err
	}

	c.cmd.Args = append(c.cmd.Args, "vite",
		"--port", fmt.Sprintf("%d", c.port),
		"--host", "127.0.0.1",
		"--clearScreen", "false",
		"--strictPort", "true",
	)

	if c.opts.ViteConfigFile != "" {
		c.cmd.Args = append(
			c.cmd.Args,
			"--config",
			viteConfigFileArgumentPath(
				c.opts.ViteConfigFile,
				c.opts.JSPackageManagerCmdDir,
			),
		)
	}

	log.Info("Running vite (dev)...",
		"command", fmt.Sprintf(`"%s"`, strings.Join(c.cmd.Args, " ")),
	)

	err = c.cmd.Start()
	if err != nil {
		c.mu.Unlock()
		log.Error(fmt.Sprintf("Error running vite (dev): %s", err))
		return err
	}

	c.mu.Unlock()
	return nil
}

func (c *BuildCtx) Wait() {
	c.mu.Lock()
	if c.cmd == nil || c.cmd.Process == nil {
		c.mu.Unlock()
		return
	}
	if c.waitInProgress {
		done := c.waitDone
		c.mu.Unlock()
		if done != nil {
			<-done
		}
		return
	}
	c.waitInProgress = true
	c.waitDone = make(chan struct{})
	cmd := c.cmd
	done := c.waitDone
	c.mu.Unlock()

	err := cmd.Wait()

	c.mu.Lock()
	c.waitInProgress = false
	close(done)
	c.waitDone = nil
	c.mu.Unlock()

	if err != nil {
		log.Info(fmt.Sprintf("viteutil: BuildCtx: Wait: %s", err))
	}
}

func (c *BuildCtx) Cleanup() {
	cleanupError := c.CleanupWithError()
	if cleanupError != nil {
		log.Info(fmt.Sprintf("viteutil: BuildCtx: Cleanup: %s", cleanupError))
	}
}

func (c *BuildCtx) CleanupWithError() error {
	c.mu.Lock()
	terminateError := c.terminateProcessLockedAndWait()
	cmd := c.cmd
	c.mu.Unlock()

	if terminateError != nil {
		return terminateError
	}

	if cmd != nil && cmd.Process != nil {
		log.Info("Cleanup: Terminated vite process", "pid", cmd.Process.Pid)
	}
	return nil
}

func (c *BuildCtx) ProdBuild() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if strings.TrimSpace(c.opts.OutDir) == "" {
		return fmt.Errorf("OutDir is required")
	}
	if strings.TrimSpace(c.opts.ManifestOut) == "" {
		return fmt.Errorf("ManifestOut is required")
	}

	if err := c.prep_cmd(); err != nil {
		return err
	}

	c.cmd.Args = append(c.cmd.Args, "vite", "build",
		"--outDir", c.opts.OutDir,
		"--assetsDir", filepath.Join("."),
		"--manifest", "__temp_viteutil_manifest__.json",
	)

	if c.opts.ViteConfigFile != "" {
		c.cmd.Args = append(
			c.cmd.Args,
			"--config",
			viteConfigFileArgumentPath(
				c.opts.ViteConfigFile,
				c.opts.JSPackageManagerCmdDir,
			),
		)
	}

	c.cmd.Env = append(os.Environ(), "ROLLDOWN_OPTIONS_VALIDATION=loose")

	log.Info("Running vite build (prod)...",
		"command", fmt.Sprintf(`"%s"`, strings.Join(c.cmd.Args, " ")),
	)

	if err := c.cmd.Run(); err != nil {
		log.Error(fmt.Sprintf("Error running vite build (prod): %s", err))
		return err
	}

	// Move __temp_viteutil_manifest__.json to the specified location
	manifestPath := filepath.Join(c.opts.OutDir, "__temp_viteutil_manifest__.json")
	manifestOutputDirectory := filepath.Dir(c.opts.ManifestOut)
	if err := os.MkdirAll(manifestOutputDirectory, 0o755); err != nil {
		log.Error(fmt.Sprintf("Error creating manifest output directory: %s", err))
		return fmt.Errorf("create manifest output directory: %w", err)
	}
	if err := os.Rename(manifestPath, c.opts.ManifestOut); err != nil {
		log.Error(fmt.Sprintf("Error moving vite manifest: %s", err))
		return err
	}

	log.Info("DONE running vite build (prod)",
		"manifest", c.opts.ManifestOut,
		"outDir", c.opts.OutDir,
	)

	return nil
}

func shouldIgnoreProcessTerminationError(
	processTerminationError error,
) bool {
	if processTerminationError == nil {
		return true
	}
	if errors.Is(processTerminationError, os.ErrProcessDone) {
		return true
	}
	if errors.Is(processTerminationError, syscall.ESRCH) {
		return true
	}
	errorString := strings.ToLower(processTerminationError.Error())
	if strings.Contains(errorString, "process already finished") {
		return true
	}
	return false
}

func shouldIgnoreProcessWaitError(
	processWaitError error,
) bool {
	if processWaitError == nil {
		return true
	}
	var exitError *exec.ExitError
	if errors.As(processWaitError, &exitError) {
		errorString := strings.ToLower(processWaitError.Error())
		if strings.Contains(errorString, "signal: terminated") ||
			strings.Contains(errorString, "signal: killed") {
			return true
		}
	}
	errorString := strings.ToLower(processWaitError.Error())
	if strings.Contains(errorString, "waitid: no child processes") {
		return true
	}
	return false
}

func signalProcessTerminationWithoutWaiting(
	process *os.Process,
) error {
	if process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return process.Kill()
	}

	processGroupID, getProcessGroupIDError := syscall.Getpgid(process.Pid)
	if getProcessGroupIDError == nil && processGroupID > 0 {
		currentProcessGroupID := syscall.Getpgrp()
		if currentProcessGroupID != processGroupID {
			return syscall.Kill(-processGroupID, syscall.SIGTERM)
		}
	}
	return process.Signal(syscall.SIGTERM)
}

func signalProcessKillWithoutWaiting(
	process *os.Process,
) error {
	if process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return process.Kill()
	}

	processGroupID, getProcessGroupIDError := syscall.Getpgid(process.Pid)
	if getProcessGroupIDError == nil && processGroupID > 0 {
		currentProcessGroupID := syscall.Getpgrp()
		if currentProcessGroupID != processGroupID {
			return syscall.Kill(-processGroupID, syscall.SIGKILL)
		}
	}
	return process.Kill()
}

func waitForViteProcessExitWithKillFallback(
	waitResultChannel chan error,
	process *os.Process,
	gracefulStopTimeout time.Duration,
	signalTerminationError error,
) error {
	if gracefulStopTimeout <= 0 {
		gracefulStopTimeout = 3 * time.Second
	}
	if shouldIgnoreProcessTerminationError(signalTerminationError) {
		signalTerminationError = nil
	}

	select {
	case waitError := <-waitResultChannel:
		if !shouldIgnoreProcessWaitError(waitError) {
			return waitError
		}
		return signalTerminationError
	case <-time.After(gracefulStopTimeout):
	}

	killError := signalProcessKillWithoutWaiting(process)
	if shouldIgnoreProcessTerminationError(killError) {
		killError = nil
	}

	select {
	case waitError := <-waitResultChannel:
		if !shouldIgnoreProcessWaitError(waitError) {
			return waitError
		}
		if signalTerminationError != nil {
			return signalTerminationError
		}
		return killError
	case <-time.After(gracefulStopTimeout):
		if signalTerminationError != nil {
			return signalTerminationError
		}
		if killError != nil {
			return killError
		}
		return fmt.Errorf("timed out waiting for vite process to exit")
	}
}

// terminateProcessLockedAndWait assumes c.mu is locked on entry.
func (c *BuildCtx) terminateProcessLockedAndWait() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	if c.waitInProgress {
		waitDone := c.waitDone
		process := c.cmd.Process

		c.mu.Unlock()
		err := signalProcessTerminationWithoutWaiting(process)
		if err == nil && waitDone != nil {
			<-waitDone
		}
		c.mu.Lock()

		return err
	}

	if c.cmd.ProcessState != nil {
		return nil
	}

	c.waitInProgress = true
	c.waitDone = make(chan struct{})
	waitDone := c.waitDone
	process := c.cmd.Process

	c.mu.Unlock()
	waitResultChannel := make(chan error, 1)
	go func() {
		waitResultChannel <- c.cmd.Wait()
	}()

	err := waitForViteProcessExitWithKillFallback(
		waitResultChannel,
		process,
		3*time.Second,
		signalProcessTerminationWithoutWaiting(process),
	)
	c.mu.Lock()

	c.waitInProgress = false
	close(waitDone)
	c.waitDone = nil

	return err
}
