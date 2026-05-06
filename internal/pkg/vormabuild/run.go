package vormabuild

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/internal/pkg/fswatcher"
	"github.com/vormadev/vorma/internal/pkg/lockfile"
	"github.com/vormadev/vorma/internal/pkg/mailbox"
	"github.com/vormadev/vorma/internal/pkg/vormarun"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/globset"
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/kit/netutil"
	"github.com/vormadev/vorma/kit/searchparams"
	"github.com/vormadev/vorma/kit/set"
)

type run_state struct {
	log    *slog.Logger
	__c    *vorma.Config
	is_dev bool

	build_entry string

	dev_lock *lockfile.PIDLock

	last_cfg         *vorma_cfg
	cfg              *vorma_cfg
	live_ts_result   live_ts_result
	static_ts_result static_ts_result
	ts_modules       map[string]ts_route
	search_schemas   map[string]searchparams.Schema
	manifest         *vormarun.Manifest

	pub_fm map[string]string

	root_ctx        context.Context
	root_ctx_cancel context.CancelFunc
	mu              sync.Mutex

	build_ctx        context.Context
	build_ctx_cancel context.CancelFunc

	watcher            *fswatcher.Watcher
	watcher_ctx        context.Context
	watcher_ctx_cancel context.CancelFunc

	dev_mux_port      int
	dev_refresh_token string
	client_manager    *client_manager

	app_server_sv  *supervisor
	vite_server_sv *supervisor

	mailbox *mailbox.Mailbox[string, mail]

	critical_css       string
	css_files_to_watch *set.Set[string]

	vite_plugin_control_port int

	child_exit_ch chan child_process_exit
	panic_ch      chan any
}

type mail struct {
	time                       time.Time
	includes_client_revalidate bool
}

type child_process_exit struct {
	name string
	err  error
}

func (rs *run_state) stop_child_processes(should_force bool) {
	rs.mu.Lock()
	asv := rs.app_server_sv
	vsv := rs.vite_server_sv
	rs.mu.Unlock()

	wg := sync.WaitGroup{}
	wg.Go(func() {
		if asv != nil {
			asv.stop(should_force)
		}
	})
	if vsv != nil {
		vsv.stop(should_force)
	}
	wg.Wait()
}

func (rs *run_state) on_child_process_exit(name string, err error) {
	if rs.root_ctx.Err() != nil {
		return
	}
	select {
	case rs.child_exit_ch <- child_process_exit{name: name, err: err}:
	default:
	}
	rs.root_ctx_cancel()
}

func (rs *run_state) go_safely(fn func()) {
	go func() {
		defer func() {
			if p := recover(); p != nil {
				rs.on_background_panic(p)
			}
		}()
		fn()
	}()
}

func (rs *run_state) on_background_panic(p any) {
	select {
	case rs.panic_ch <- p:
	default:
	}
	rs.root_ctx_cancel()
}

func (rs *run_state) panic_for_child_process_exit(child_exit child_process_exit) {
	panic(fmt.Sprintf(
		"%s process exited unexpectedly: %v",
		child_exit.name,
		child_exit.err,
	))
}

func (rs *run_state) panic_if_fatal_event_pending() {
	select {
	case p := <-rs.panic_ch:
		panic(p)
	case child_exit := <-rs.child_exit_ch:
		rs.panic_for_child_process_exit(child_exit)
	default:
	}
}

func (rs *run_state) build_cancelled_error(stage string) error {
	return fmt.Errorf(
		"build cancelled during %s (build context: %v; root context: %v)",
		stage,
		rs.build_ctx.Err(),
		rs.root_ctx.Err(),
	)
}

func Run(
	_v *vorma.Router,
	caller_file string,
	is_dev bool,
) {
	if envutil.GetBool(live_state_mode_env_key, false) {
		print_live_state_and_exit(_v)
	}

	root_ctx, root_ctx_cancel := context.WithCancel(context.Background())
	build_ctx, build_ctx_cancel := context.WithCancel(root_ctx)
	watcher_ctx, watcher_ctx_cancel := context.WithCancel(root_ctx)

	rs := &run_state{
		log:    colorlog.New("vorma"),
		__c:    _v.Instance().Config(),
		is_dev: is_dev,
		// The purpose of doing this rather than having it as a config item
		// is so that we aren't pretending that it can be updated live (as
		// changing the build entry requires a manual restart). If we had it
		// as a config item, users would reasonably expect that changing it
		// would trigger a rebuild with the new entry, which is not the case.
		build_entry:        filepath.Dir(caller_file),
		root_ctx:           root_ctx,
		root_ctx_cancel:    root_ctx_cancel,
		build_ctx:          build_ctx,
		build_ctx_cancel:   build_ctx_cancel,
		watcher_ctx:        watcher_ctx,
		watcher_ctx_cancel: watcher_ctx_cancel,
		app_server_sv:      &supervisor{},
		vite_server_sv:     &supervisor{},
		mailbox:            mailbox.NewMailbox[string, mail](),
		css_files_to_watch: set.New[string](),
		client_manager:     new_client_manager(),
		child_exit_ch:      make(chan child_process_exit, 1),
		panic_ch:           make(chan any, 1),
	}

	defer func() {
		if rs.dev_lock != nil {
			_ = rs.dev_lock.Release()
		}
		rs.stop_child_processes(false)
		root_ctx_cancel()
	}()

	/////// LISTEN FOR KILL SIGNALS
	sig_ch := make(chan os.Signal, 2)
	signal.Notify(sig_ch, syscall.SIGINT, syscall.SIGTERM)
	rs.go_safely(func() {
		<-sig_ch

		rs.go_safely(func() {
			<-sig_ch
			rs.log.Info("Force killing...")
			rs.stop_child_processes(true)
		})

		rs.log.Info("Shutting down...")
		rs.stop_child_processes(false)

		root_ctx_cancel()
	})

	if err := rs.refresh_go(); err != nil {
		panic(fmt.Sprintf("Initialization error: %v", err))
	}

	/////// DEV MUX
	dev_mux_port, err := netutil.GetRandomFreePort()
	if err != nil {
		panic(fmt.Sprintf("Error getting free port for internal dev server: %v", err))
	}
	dev_refresh_token, err := id.New(16)
	if err != nil {
		panic(fmt.Sprintf("Error generating dev refresh token: %v", err))
	}
	rs.mu.Lock()
	rs.dev_mux_port = dev_mux_port
	rs.dev_refresh_token = dev_refresh_token
	rs.mu.Unlock()

	dev_mux := http.NewServeMux()
	vite_root := func(p string) string { return path.Join("/vite-plugin", p) }
	dev_mux.HandleFunc(vite_root("/cfg"), rs.vite_config_handler())
	dev_mux.HandleFunc(vite_root("/hash"), rs.vite_hash_handler())
	dev_mux.HandleFunc(vite_root("/set-port"), rs.vite_set_port_handler())
	dev_mux.HandleFunc(rs.dev_refresh_endpoint(), rs.dev_refresh_handler())
	listen, err := net.Listen("tcp", fmt.Sprintf("%s:%d", dev_loopback_host, dev_mux_port))
	if err != nil {
		panic(fmt.Sprintf("Error starting internal dev server: %v", err))
	}
	rs.go_safely(func() {
		if err := http.Serve(listen, dev_mux); err != nil &&
			root_ctx.Err() == nil {
			panic(fmt.Sprintf("Internal dev server failed: %v", err))
		}
	})

	if rs.is_dev {
		if err := rs.start_vite_server(); err != nil {
			if root_ctx.Err() != nil {
				return
			}
			panic(fmt.Sprintf("Error starting Vite server: %v", err))
		}
		rs.panic_if_test_stage(__test_panic_stage_after_initial_vite_start)
		rs.panic_async_if_test_stage(__test_panic_stage_async_after_initial_vite_start)
	}

	if !rs.is_dev {
		cfg, err := rs.get_config()
		if err != nil {
			if root_ctx.Err() != nil {
				return
			}
			panic(fmt.Sprintf("Error getting config: %v", err))
		}
		cmd, err := cfg.run_vite_prod_build_cmd(build_ctx, dev_mux_port)
		if err != nil {
			if root_ctx.Err() != nil {
				return
			}
			panic(fmt.Sprintf("Error preparing Vite production build command: %v", err))
		}
		if err := cmd.Run(); err != nil {
			if root_ctx.Err() != nil {
				return
			}
			panic(fmt.Sprintf("Error running Vite production build: %v", err))
		}
		if err := rs.write_manifest(); err != nil {
			if root_ctx.Err() != nil {
				return
			}
			panic(fmt.Sprintf("Error writing manifest: %v", err))
		}
	}

	/////// BLOCK AND WATCH FOR FILE CHANGES
	if rs.is_dev {
		for {
			rs.panic_if_fatal_event_pending()

			select {
			case p := <-rs.panic_ch:
				panic(p)
			case child_exit := <-rs.child_exit_ch:
				rs.panic_for_child_process_exit(child_exit)
			case <-root_ctx.Done():
				rs.panic_if_fatal_event_pending()
				return
			case <-rs.mailbox.C():
				data := rs.mailbox.Claim()

				rs.mu.Lock()
				rs.build_ctx, rs.build_ctx_cancel = context.WithCancel(rs.root_ctx)
				rs.mu.Unlock()

				_, go_implicated := data[mailbox_key_go]
				if go_implicated {
					if err := rs.refresh_go(); err != nil {
						rs.log.Error("Error during refresh", "error", err)
						rs.broadcast_build_error(err)
					}
					continue
				}

				static_mail, static_implicated := data[mailbox_key_static]
				if static_implicated {
					if err := rs.run_static_build(static_build_opts{
						should_log_start:           true,
						is_initial:                 false,
						includes_client_revalidate: static_mail.includes_client_revalidate,
					}); err != nil {
						rs.log.Error("Error during static build", "error", err)
						rs.broadcast_build_error(err)
					}
				}
			}
		}
	}
}

func (rs *run_state) on_evt_batch(evts []fswatcher.Evt) error {
	cfg, err := rs.get_config()
	if err != nil {
		// This means we are in the middle of refreshing and should just ignore the event
		return nil
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	server_watch_set, err := globset.Compile(cfg.server_watch_patterns())
	if err != nil {
		return fmt.Errorf("compile server watch patterns: %w", err)
	}
	client_revalidate_watch_set, err := globset.Compile(
		cfg.client_revalidate_on_change_patterns(),
	)
	if err != nil {
		return fmt.Errorf(
			"compile client revalidate watch patterns: %w",
			err,
		)
	}

	static_implicated := false
	go_implicated := false
	client_revalidate_implicated := false

	for _, evt := range evts {
		p := fsutil.SysNorm(evt.Path)
		rel_path := cfg.watch_relative_path(p)
		if !go_implicated {
			if server_watch_set.Match(rel_path) {
				go_implicated = true
				break
			}
		}
		if !static_implicated {
			if cfg.matches_pub_src(p) || rs.css_files_to_watch.Has(p) {
				static_implicated = true
			}
		}
		if !client_revalidate_implicated {
			if client_revalidate_watch_set.Match(rel_path) {
				client_revalidate_implicated = true
			}
		}
	}

	if go_implicated {
		paths := make([]string, 0, len(evts))
		for _, evt := range evts {
			paths = append(paths, cfg.watch_relative_path(fsutil.SysNorm(evt.Path)))
		}
		rs.log.Info(
			"Detected Go-related file change; cancelling current build and queueing rebuild",
			"paths",
			paths,
		)
		if rs.build_ctx_cancel != nil {
			rs.build_ctx_cancel()
		}
		rs.mailbox.Deliver(mailbox_key_go, mail{time: time.Now()})
		return nil
	}
	if static_implicated || client_revalidate_implicated {
		rs.mailbox.Deliver(
			mailbox_key_static,
			mail{time: time.Now(), includes_client_revalidate: client_revalidate_implicated},
		)
	}

	return nil
}
