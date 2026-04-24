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
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/kit/netutil"
	"github.com/vormadev/vorma/kit/searchparams"
	"github.com/vormadev/vorma/kit/set"
)

type run_state struct {
	log    *slog.Logger
	__v    *vorma.Vorma
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

	root_ctx context.Context
	mu       sync.Mutex

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

	main_css           string
	critical_css       string
	css_files_to_watch *set.Set[string]

	vite_plugin_control_port int
}

type mail struct {
	time                       time.Time
	includes_client_revalidate bool
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

func Run(
	v *vorma.Vorma,
	loaders vorma.Loaders,
	actions vorma.Actions,
	is_dev bool,
	caller_file string,
) {
	if envutil.GetBool(live_state_mode_env_key, false) {
		print_live_state_and_exit(v, loaders, actions)
	}

	root_ctx, root_ctx_cancel := context.WithCancel(context.Background())
	build_ctx, build_ctx_cancel := context.WithCancel(root_ctx)
	watcher_ctx, watcher_ctx_cancel := context.WithCancel(root_ctx)

	rs := &run_state{
		log:                colorlog.New("vorma"),
		__v:                v,
		is_dev:             is_dev,
		build_entry:        filepath.Dir(caller_file),
		root_ctx:           root_ctx,
		build_ctx:          build_ctx,
		build_ctx_cancel:   build_ctx_cancel,
		watcher_ctx:        watcher_ctx,
		watcher_ctx_cancel: watcher_ctx_cancel,
		app_server_sv:      &supervisor{},
		vite_server_sv:     &supervisor{},
		mailbox:            mailbox.NewMailbox[string, mail](),
		css_files_to_watch: set.New[string](),
		client_manager:     new_client_manager(),
	}

	defer func() {
		if rs.dev_lock != nil {
			_ = rs.dev_lock.Release()
		}
		root_ctx_cancel()
		rs.stop_child_processes(false)
	}()

	/////// LISTEN FOR KILL SIGNALS
	sig_ch := make(chan os.Signal, 2)
	signal.Notify(sig_ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig_ch

		// Listen for second signal to force kill
		go func() {
			<-sig_ch
			rs.log.Info("Force killing...")
			rs.stop_child_processes(true)
		}()

		rs.log.Info("Shutting down...")
		rs.stop_child_processes(false)

		root_ctx_cancel()
	}()

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
	go http.Serve(listen, dev_mux)

	if rs.is_dev {
		if err := rs.start_vite_server(); err != nil {
			if root_ctx.Err() != nil {
				return
			}
			panic(fmt.Sprintf("Error starting Vite server: %v", err))
		}
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
			select {
			case <-root_ctx.Done():
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

	static_implicated := false
	go_implicated := false
	client_revalidate_implicated := false

	for _, evt := range evts {
		p := fsutil.SysNorm(evt.Path)
		if !go_implicated {
			if cfg.matches_server_watch(p) {
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
			if cfg.matches_client_revalidate_on_change(p) {
				client_revalidate_implicated = true
			}
		}
	}

	if go_implicated {
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
