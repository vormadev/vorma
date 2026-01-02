package viteutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/grace"
)

var Log = colorlog.New("viteutil")

type BuildCtx struct {
	mu   *sync.RWMutex
	cmd  *exec.Cmd
	opts *BuildCtxOptions
	port int
}

func (c *BuildCtx) GetPort() int {
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
	port := opts.DefaultPort
	if port == 0 {
		port = 5173
	}
	return &BuildCtx{
		mu:   &sync.RWMutex{},
		opts: opts,
		port: port,
	}
}

func (c *BuildCtx) prep_cmd() {
	split_cmd := strings.Fields(c.opts.JSPackageManagerBaseCmd)

	c.cmd = exec.Command(split_cmd[0], split_cmd[1:]...)
	c.cmd.Stdout, c.cmd.Stderr = os.Stdout, os.Stderr

	if c.opts.JSPackageManagerCmdDir != "" {
		c.cmd.Dir = c.opts.JSPackageManagerCmdDir
	}
}

func (c *BuildCtx) DevBuild() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil && c.cmd.Process != nil {
		if err := grace.TerminateProcess(c.cmd.Process, 3*time.Second, nil); err != nil {
			Log.Warn(fmt.Sprintf("DevBuild: Error terminating vite process: %s", err))
			return err
		} else {
			Log.Info("DevBuild: Terminated vite process", "pid", c.cmd.Process.Pid)
		}
	}

	c.prep_cmd()

	var err error
	c.port, err = InitPort(c.port)
	if err != nil {
		Log.Error(fmt.Sprintf("Error initializing vite port: %s", err))
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

	Log.Info("Running vite (dev)...",
		"command", fmt.Sprintf(`"%s"`, strings.Join(c.cmd.Args, " ")),
	)

	if err := c.cmd.Start(); err != nil {
		Log.Error(fmt.Sprintf("Error running vite (dev): %s", err))
	}

	return nil
}

func (c *BuildCtx) Wait() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cmd != nil && c.cmd.Process != nil && c.cmd.ProcessState == nil {
		if err := c.cmd.Wait(); err != nil {
			Log.Info(fmt.Sprintf("viteutil: BuildCtx: Wait: %s", err))
		}
	}
}

func (c *BuildCtx) Cleanup() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cmd != nil && c.cmd.Process != nil && c.cmd.ProcessState == nil {
		if err := grace.TerminateProcess(c.cmd.Process, 3*time.Second, nil); err != nil {
			Log.Info(fmt.Sprintf("viteutil: BuildCtx: Cleanup: %s", err))
		} else {
			Log.Info("Cleanup: Terminated vite process", "pid", c.cmd.Process.Pid)
		}
	}
}

func (c *BuildCtx) ProdBuild() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.prep_cmd()

	c.cmd.Args = append(c.cmd.Args, "vite", "build",
		"--outDir", filepath.Join(".", c.opts.OutDir),
		"--assetsDir", filepath.Join("."),
		"--manifest", "__temp_viteutil_manifest__.json",
	)

	if c.opts.ViteConfigFile != "" {
		c.cmd.Args = append(c.cmd.Args, "--config", c.opts.ViteConfigFile)
	}

	os.Setenv("ROLLDOWN_OPTIONS_VALIDATION", "loose")

	Log.Info("Running vite build (prod)...",
		"command", fmt.Sprintf(`"%s"`, strings.Join(c.cmd.Args, " ")),
	)

	if err := c.cmd.Run(); err != nil {
		Log.Error(fmt.Sprintf("Error running vite build (prod): %s", err))
		return err
	}

	// Move __temp_viteutil_manifest__.json to the specified location
	manifestPath := filepath.Join(".", c.opts.OutDir, "__temp_viteutil_manifest__.json")
	if err := os.Rename(manifestPath, c.opts.ManifestOut); err != nil {
		Log.Error(fmt.Sprintf("Error moving vite manifest: %s", err))
		return err
	}

	Log.Info("DONE running vite build (prod)",
		"manifest", c.opts.ManifestOut,
		"outDir", c.opts.OutDir,
	)

	return nil
}
