package vormabuild

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/vormadev/vorma/internal/pkg/cssbundle"
	"github.com/vormadev/vorma/internal/pkg/fswatcher"
	"github.com/vormadev/vorma/internal/pkg/lockfile"
	"github.com/vormadev/vorma/internal/pkg/staticproc"
	"github.com/vormadev/vorma/internal/pkg/vormarun"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/set"
)

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
		__cfg, err = to_cfg(rs.__v)
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

			rs.__v = rs.cfg.V
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
		rs.watcher = fswatcher.NewWatcher(fswatcher.WatcherOptions{
			WatchRoot: rs.cfg.watch_root(),
			IgnorePatterns: append(
				base_watch_ignore_patterns,
				rs.cfg.global_watch_exclude_patterns()...,
			),
		})
		rs.watcher_ctx, rs.watcher_ctx_cancel = context.WithCancel(rs.root_ctx)
		go rs.watcher.Watch(rs.watcher_ctx, rs.on_evt_batch)
	}

	if err := rs.cfg.write_gitignore(); err != nil {
		return fmt.Errorf("error writing .gitignore: %w", err)
	}

	if rs.build_ctx.Err() != nil {
		rs.mu.Unlock()
		return fmt.Errorf("build cancelled")
	}
	rs.mu.Unlock()

	if err := rs.run_static_build(static_build_opts{
		should_log_start: false,
		is_initial:       is_initial,
	}); err != nil {
		return fmt.Errorf("error building static files: %w", err)
	}

	if build_ctx.Err() != nil {
		return fmt.Errorf("build cancelled")
	}

	wg.Wait()
	if rs.is_dev {
		if app_err := <-app_server_build_err_ch; app_err != nil {
			return app_err
		}
	}
	if build_ctx.Err() != nil {
		return fmt.Errorf("build cancelled")
	}

	if rs.is_dev {
		// Needed before app start so runtime init has ActionsMountRoot.
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
		if err := rs.send_vite_plugin_restart(); err != nil {
			return fmt.Errorf("error sending restart command to Vite plugin: %w", err)
		}
	}

	if build_ctx.Err() != nil {
		return fmt.Errorf("build cancelled")
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
// Bundles CSS.
// Updates watched CSS files.
// Sets rs.pub_fm, rs.main_css, rs.critical_css, and rs.css_files_to_watch.
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

	var main_css_result cssbundle.BundleOutput
	var critical_css_result cssbundle.BundleOutput
	if cfg.main_css_entry() != "." {
		main_css_result, err = cssbundle.Bundle(cfg.main_css_entry(), pub_fm)
		if err != nil {
			return fmt.Errorf("error bundling main CSS: %w", err)
		}
	}
	if cfg.critical_css_entry() != "." {
		critical_css_result, err = cssbundle.Bundle(cfg.critical_css_entry(), pub_fm)
		if err != nil {
			return fmt.Errorf("error bundling critical CSS: %w", err)
		}
	}

	css_files_to_watch := set.New([]string{
		cfg.main_css_entry(),
		cfg.critical_css_entry(),
	})
	for _, f := range main_css_result.Imports {
		css_files_to_watch.Add(f)
	}
	for _, f := range critical_css_result.Imports {
		css_files_to_watch.Add(f)
	}

	pub_files.Inject(&staticproc.File{
		SrcPathRel:    vormarun.Main_CSS_Filename,
		Bytes:         []byte(main_css_result.CSS),
		OutNamePrefix: vormarun.Public_Static_Out_Name_Prefix,
	})
	pub_fm = cfg.to_pub_fm(pub_files)

	rs.mu.Lock()

	if rs.build_ctx.Err() != nil {
		rs.mu.Unlock()
		return fmt.Errorf("build cancelled")
	}

	main_css_changed := main_css_result.CSS != rs.main_css
	critical_css_changed := critical_css_result.CSS != rs.critical_css
	pub_fm_changed := hash_pub_fm(rs.pub_fm) != hash_pub_fm(pub_fm)

	rs.pub_fm = pub_fm
	rs.main_css = main_css_result.CSS
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
	if main_css_changed {
		rs.client_manager.broadcast(refresh_payload{
			ChangeType: update_main_css,
			MainCSSURL: pub_fm[vormarun.Main_CSS_Filename],
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
		if k == vormarun.Main_CSS_Filename {
			continue
		}
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
