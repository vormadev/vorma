package ki

import (
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/netutil"
)

type must_start_dev_opts struct {
	is_rebuild   bool
	recompile_go bool
}

func (c *Config) MustStartDev(_opts ...must_start_dev_opts) {
	var opts must_start_dev_opts
	if len(_opts) > 0 {
		opts = _opts[0]
	}

	if opts.is_rebuild {
		c.send_rebuilding_signal()
		c.kill_running_go_binary()
	}

	c.MainInit(MainInitOptions{IsDev: true, IsRebuild: opts.is_rebuild}, "MustStartDev")

	MustGetAppPort() // Warm port right away, in case default is unavailable. Also, env needs to be set in this scope.

	refresh_server_port, err := netutil.GetFreePort(default_refresh_server_port)
	if err != nil {
		c.panic("failed to get free port", err)
	}
	set_refresh_server_port(refresh_server_port)

	// build without binary
	err = c.BuildWave(BuildOptions{
		IsDev:             true,
		RecompileGoBinary: false,
		is_dev_rebuild:    opts.is_rebuild,
	})
	if err != nil {
		c.panic("failed to build app", err)
	}

	if c.isUsingVite() {
		if c._vite_dev_ctx != nil {
			c._vite_dev_ctx.Cleanup()
		}
		c._vite_dev_ctx, err = c.viteDevBuild()
		if err != nil {
			c.panic("failed to start vite dev server", err)
		}
		go c._vite_dev_ctx.Wait()
	}

	if !opts.is_rebuild || opts.recompile_go {
		// compile go binary now because we didn't above
		if err := c.compile_go_binary(true); err != nil {
			c.panic("failed to compile go binary", err)
		}
	}

	go c.run_go_binary()
	go c.setup_browser_refresh_mux()

	if opts.is_rebuild {
		c.must_reload_broadcast(
			refreshFilePayload{ChangeType: changeTypeOther},
			must_reload_broadcast_opts{
				wait_for_app:  true,
				wait_for_vite: c.isUsingVite(),
				message:       "Hard reloading browser",
			},
		)

		return
	}

	defer c.kill_running_go_binary()

	debouncer := new_debouncer(30*time.Millisecond, func(events []fsnotify.Event) {
		c.process_batched_events(events)
	})

	for {
		select {
		case evt := <-c.watcher.Events:
			debouncer.add_evt(evt)
		case err := <-c.watcher.Errors:
			c.Logger.Error(fmt.Sprintf("watcher error: %v", err))
		}
	}
}
