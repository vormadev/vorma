package vormabuild

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/vormadev/vorma/internal/pkg/cssbundle"
	"github.com/vormadev/vorma/internal/pkg/fswatcher"
	"github.com/vormadev/vorma/internal/pkg/lockfile"
	"github.com/vormadev/vorma/internal/pkg/staticproc"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/set"
)

const public_url_prefix = "@public/"

func (rs *run_state) log_build_start(is_initial bool) {
	if is_initial {
		rs.log.Info("Running initial build...")
	} else {
		rs.log.Info("Running incremental build...")
	}
}

/////////////////////////////////////////////////////////////////////
/////// REFRESH GO
/////////////////////////////////////////////////////////////////////

// Acquires lock.
// Stops running app server.
// Rebuilds app server.
// Reads live gen state in gen mode (and updates rs.cfg and rs.type_gen_result).
// Restarts fswatcher with new watch root and ignore patterns.
// Ensures static output dirs exist.
// Writes boilerplate files.
// Writes vorma config.
// Runs static build (in case config changed src/out dirs).
// Restarts app server.
func (rs *run_state) refresh_go() error {
	is_initial := false

	rs.client_manager.broadcast(refresh_payload{
		ChangeType: show_rebuilding_overlay,
	})

	__cfg, err := rs.get_config()
	if err != nil {
		is_initial = true
		__cfg, err = to_cfg(rs.__c)
		if err != nil {
			return fmt.Errorf("error converting config: %w", err)
		}
	}

	err = fsutil.EnsureDirs(
		__cfg.pub_out(),
		filepath.Dir(__cfg.ts_gen_out_file()),
	)
	if err != nil {
		return fmt.Errorf("error ensuring output dirs: %w", err)
	}

	if rs.is_dev && rs.dev_lock == nil {
		rs.mu.Lock()
		rs.dev_lock = lockfile.NewPIDLock(filepath.Join(__cfg.vorma_out(), "dev.lock"))
		if err := rs.dev_lock.Acquire(); err != nil {
			rs.mu.Unlock()
			rs.log.Error(
				"Another instance of the dev server is already running for this project. Exiting.",
			)
			return fmt.Errorf("error acquiring dev lock: %w", err)
		}
		rs.mu.Unlock()
	}

	rs.log_build_start(is_initial)
	if !is_initial {
		rs.panic_if_test_stage(__test_panic_stage_during_go_refresh)
	}

	wg := sync.WaitGroup{}
	app_server_build_err_ch := make(chan error, 1)

	rs.mu.Lock()
	app_server_sv := rs.app_server_sv
	build_ctx := rs.build_ctx
	rs.mu.Unlock()

	if rs.is_dev {
		wg.Go(func() { app_server_sv.stop(false) })
		wg.Go(func() {
			if err := __cfg.compile_app_server(build_ctx); err != nil {
				app_server_build_err_ch <- fmt.Errorf("error compiling server: %w", err)
				return
			}
			app_server_build_err_ch <- nil
		})
	}
	defer wg.Wait()

	live_state, err := __cfg.read_live_state_from_subprocess(build_ctx, rs.build_entry)
	if err != nil {
		return fmt.Errorf("error reading live state: %w", err)
	}

	// From here on out, use rs.cfg, not __cfg

	rs.mu.Lock()

	rs.last_cfg = rs.cfg
	rs.cfg, err = to_cfg(live_state.VormaConfig)
	if err != nil {
		rs.mu.Unlock()
		return fmt.Errorf("error converting config: %w", err)
	}
	rs.live_ts_result = live_state.TSResult
	rs.ts_modules = live_state.TSModules
	rs.search_schemas = live_state.SearchSchemas

	if rs.is_dev && rs.last_cfg != nil {
		last_cfg_json, err := jsonutil.Serialize(rs.last_cfg)
		if err != nil {
			rs.mu.Unlock()
			return fmt.Errorf("error serializing last cfg: %w", err)
		}

		new_cfg_json, err := jsonutil.Serialize(rs.cfg)
		if err != nil {
			rs.mu.Unlock()
			return fmt.Errorf("error serializing new cfg: %w", err)
		}

		if string(last_cfg_json) != string(new_cfg_json) {
			if rs.last_cfg.dist_dir() != rs.cfg.dist_dir() {
				if err := os.RemoveAll(rs.last_cfg.vorma_out()); err != nil {
					rs.log.Warn("Failed to clean up old .vorma directory",
						"path", rs.last_cfg.vorma_out(),
						"error", err,
					)
				}
			}

			rs.__c = rs.cfg.C
			rs.cfg = nil

			rs.dev_lock.Release()
			rs.dev_lock = nil

			rs.vite_server_sv.stop(false)

			rs.mu.Unlock()
			wg.Wait()
			return rs.refresh_go()
		}
	}

	if rs.is_dev {
		if rs.watcher != nil {
			rs.watcher_ctx_cancel()
		}
		ignore_patterns := append(
			base_watch_ignore_patterns,
			rs.cfg.global_watch_exclude_patterns()...,
		)
		ignore_patterns = append(
			ignore_patterns,
			rs.cfg.vorma_out_watch_ignore_pattern(),
		)
		rs.watcher = fswatcher.NewWatcher(fswatcher.WatcherOptions{
			WatchRoot:      rs.cfg.watch_root(),
			IgnorePatterns: ignore_patterns,
		})
		rs.watcher_ctx, rs.watcher_ctx_cancel = context.WithCancel(rs.root_ctx)
		rs.go_safely(func() {
			rs.watcher.Watch(rs.watcher_ctx, rs.on_evt_batch)
		})
	}

	if err := rs.cfg.write_gitignore(); err != nil {
		return fmt.Errorf("error writing .gitignore: %w", err)
	}

	if rs.build_ctx.Err() != nil {
		rs.mu.Unlock()
		return rs.build_cancelled_error("before static build")
	}
	rs.mu.Unlock()

	if err := rs.run_static_build(static_build_opts{
		should_log_start: false,
		is_initial:       is_initial,
	}); err != nil {
		return fmt.Errorf("error building static files: %w", err)
	}

	if build_ctx.Err() != nil {
		return rs.build_cancelled_error("after static build")
	}

	wg.Wait()
	if rs.is_dev {
		if app_err := <-app_server_build_err_ch; app_err != nil {
			return app_err
		}
	}
	if build_ctx.Err() != nil {
		return rs.build_cancelled_error("after app server compile")
	}

	if rs.is_dev {
		// Needed before app start so runtime init has APIMountRoot.
		// Will be re-written later to populate the vite server port,
		// which is not available at this point.
		if err := rs.write_manifest(); err != nil {
			return fmt.Errorf("error writing manifest: %w", err)
		}
		if err := rs.start_app_server(); err != nil {
			return fmt.Errorf("error starting server: %w", err)
		}
	}

	if !is_initial {
		rs.panic_if_test_stage(__test_panic_stage_before_vite_restart)
		if err := rs.send_vite_plugin_restart(); err != nil {
			return fmt.Errorf("error sending restart command to Vite plugin: %w", err)
		}
		rs.panic_if_test_stage(__test_panic_stage_after_vite_restart)
	}

	if build_ctx.Err() != nil {
		return rs.build_cancelled_error("before Vite server start")
	}

	if rs.dev_mux_port != 0 && !rs.vite_server_sv.is_running() {
		if err := rs.start_vite_server(); err != nil {
			// Panic here -- not really recoverable given we don't have plugin comms
			panic(fmt.Sprintf("Error starting Vite server: %v", err))
		}
	}

	return nil
}

/////////////////////////////////////////////////////////////////////
/////// RUN STATIC BUILD
/////////////////////////////////////////////////////////////////////

type static_build_opts struct {
	should_log_start           bool
	is_initial                 bool
	includes_client_revalidate bool
}

// Acquires lock.
// Reads live pub src files.
// Bundles critical CSS.
// Updates watched CSS files.
// Sets rs.pub_fm, rs.critical_css, and rs.css_files_to_watch.
// Outputs pub static files to disk.
// Writes vorma public file map to disk.
// Writes gen TS file to disk.
func (rs *run_state) run_static_build(opts static_build_opts) error {
	if opts.should_log_start {
		rs.log_build_start(opts.is_initial)
	}

	cfg, err := rs.get_config()
	if err != nil {
		return fmt.Errorf("error getting config: %w", err)
	}

	pub_files, err := cfg.collect_physical_pub_files()
	if err != nil {
		return fmt.Errorf("error collecting public static files: %w", err)
	}
	pub_fm := cfg.to_pub_fm(pub_files)

	var critical_css_result cssbundle.BundleOutput
	if cfg.critical_css_entry() != "." {
		critical_css_result, err = cssbundle.Bundle(cssbundle.BundleArgs{
			EntryPath: cfg.critical_css_entry(),
			ResolveURL: func(raw string, parsed *url.URL) (string, bool, error) {
				if strings.HasPrefix(parsed.Path, "/") {
					return raw, true, nil
				}
				if !strings.HasPrefix(parsed.Path, public_url_prefix) {
					return "", false, fmt.Errorf(
						"CSS URL paths must be absolute, external, or start with %q",
						public_url_prefix,
					)
				}

				lookup := strings.TrimPrefix(parsed.Path, public_url_prefix)
				suffix := ""
				if parsed.RawQuery != "" {
					suffix += "?" + parsed.RawQuery
				}
				if parsed.Fragment != "" {
					suffix += "#" + parsed.Fragment
				}
				if public_url, ok := pub_fm[lookup]; ok {
					return public_url + suffix, true, nil
				}

				return "", false, fmt.Errorf(
					"unresolved static public asset %q",
					parsed.Path,
				)
			},
		})
		if err != nil {
			return fmt.Errorf("error bundling critical CSS: %w", err)
		}
	}

	css_files_to_watch := set.New([]string{
		cfg.critical_css_entry(),
	})
	for _, f := range critical_css_result.Imports {
		css_files_to_watch.Add(f)
	}

	rs.mu.Lock()

	if rs.build_ctx.Err() != nil {
		rs.mu.Unlock()
		return rs.build_cancelled_error("before writing static build state")
	}

	critical_css_changed := critical_css_result.CSS != rs.critical_css
	pub_fm_changed := hash_pub_fm(rs.pub_fm) != hash_pub_fm(pub_fm)

	rs.pub_fm = pub_fm
	rs.critical_css = critical_css_result.CSS
	rs.css_files_to_watch = css_files_to_watch
	rs.static_ts_result = cfg.to_static_ts_result(pub_fm)
	rs.mu.Unlock()

	if err := staticproc.Reconcile(cfg.pub_out(), pub_files); err != nil {
		return err
	}

	if err = rs.write_ts_gen_out_file(); err != nil {
		return fmt.Errorf("error making gen TS file: %w", err)
	}

	if opts.is_initial {
		rs.log.Info("Initial build complete")
	} else {
		if err := rs.write_manifest(); err != nil {
			return fmt.Errorf("error writing manifest: %w", err)
		}

		rs.log.Info("Incremental build complete")
	}

	if pub_fm_changed {
		rs.client_manager.broadcast(refresh_payload{
			ChangeType: hard_reload,
		})
		return nil
	}

	if critical_css_changed {
		rs.client_manager.broadcast(refresh_payload{
			ChangeType:  update_critical_css,
			CriticalCSS: critical_css_result.CSS,
		})
	}
	if opts.includes_client_revalidate {
		rs.client_manager.broadcast(refresh_payload{
			ChangeType: client_revalidate,
		})
		return nil
	}

	if !opts.is_initial {
		rs.client_manager.broadcast(refresh_payload{
			ChangeType: hide_rebuilding_overlay,
		})
	}

	return nil
}

func hash_pub_fm(pub_fm map[string]string) string {
	keys := make([]string, 0, len(pub_fm))
	for k := range pub_fm {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	hash := sha256.New()
	for _, k := range keys {
		hash.Write([]byte(k))
		hash.Write([]byte(pub_fm[k]))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
