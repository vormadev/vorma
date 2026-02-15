// Package vitecmd provides build/dev command orchestration for Vite.
// Keep this package out of runtime imports.
package vitecmd

import (
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
	"github.com/vormadev/vorma/kit/grace"
	"github.com/vormadev/vorma/lab/viteutil"
)

var log = colorlog.New("vitecmd")
var initPort = viteutil.InitPort

type BuildCtx struct {
	mu             *sync.Mutex
	cmd            *exec.Cmd
	opts           *BuildCtxOptions
	port           int
	waitInProgress bool
	waitDone       chan struct{}
}

func (c *BuildCtx) GetPort() int {
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
	// optional -- default is 5173
	DefaultPort int
	// optional
	ViteConfigFile string
}

func NewBuildCtx(opts *BuildCtxOptions) *BuildCtx {
	if opts == nil {
		opts = &BuildCtxOptions{}
	}

	port := opts.DefaultPort
	if port == 0 {
		port = 5173
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
		"--clearScreen", "false",
		"--strictPort", "true",
	)

	if c.opts.ViteConfigFile != "" {
		c.cmd.Args = append(c.cmd.Args, "--config", c.opts.ViteConfigFile)
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
		log.Info(fmt.Sprintf("vitecmd: BuildCtx: Wait: %s", err))
	}
}

func (c *BuildCtx) Cleanup() {
	c.mu.Lock()
	err := c.terminateProcessLockedAndWait()
	cmd := c.cmd
	c.mu.Unlock()

	if err != nil {
		log.Info(fmt.Sprintf("vitecmd: BuildCtx: Cleanup: %s", err))
		return
	}

	if cmd != nil && cmd.Process != nil {
		log.Info("Cleanup: Terminated vite process", "pid", cmd.Process.Pid)
	}
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
		"--outDir", filepath.Join(".", c.opts.OutDir),
		"--assetsDir", filepath.Join("."),
		"--manifest", "__temp_viteutil_manifest__.json",
	)

	if c.opts.ViteConfigFile != "" {
		c.cmd.Args = append(c.cmd.Args, "--config", c.opts.ViteConfigFile)
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
	manifestPath := filepath.Join(".", c.opts.OutDir, "__temp_viteutil_manifest__.json")
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

func signalProcessTerminationWithoutWaiting(process *os.Process) error {
	if process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return process.Kill()
	}
	return process.Signal(syscall.SIGTERM)
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
	err := grace.TerminateProcess(process, 3*time.Second, nil)
	c.mu.Lock()

	c.waitInProgress = false
	close(waitDone)
	c.waitDone = nil

	return err
}
