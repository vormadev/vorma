package vormabuild

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/vormadev/vorma/kit/response"
)

func (rs *run_state) vite_config_handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := rs.get_config()
		if err != nil {
			rs.log.Error("Error getting config for vite config handler",
				"error", err,
			)
			res := response.New(w)
			res.Error(500, "error getting config")
			return
		}

		res := response.New(w)

		rs.mu.Lock()
		modules := rs.ts_modules
		rs.mu.Unlock()

		route_modules := []string{}
		for _, r := range modules {
			abs, err := filepath.Abs(r.ImportPath)
			if err != nil {
				rs.log.Error(
					"Error getting absolute path for TS module import path in vite config handler",
					"error",
					err,
					"import_path",
					r.ImportPath,
				)
				res.Error(500, "error processing TS module import paths")
				return
			}
			route_modules = append(route_modules, abs)
		}

		res.JSON(struct {
			PublicStaticBasePath string
			EntryModule          string
			RouteModules         []string
			IgnoredPatterns      []string
			DedupeList           []string
		}{
			PublicStaticBasePath: cfg.public_static_base_path(),
			EntryModule:          cfg.ts_entry_abs_slash(),
			RouteModules:         route_modules,
			IgnoredPatterns:      cfg.vite_ignored_patterns(),
			DedupeList:           cfg.vite_dedupe_list(),
		})
	}
}

func (rs *run_state) vite_hash_handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)

		src_path := r.URL.Query().Get("src_path")
		if src_path == "" {
			res.Error(400, "missing src_path parameter")
			return
		}

		rs.mu.Lock()
		defer rs.mu.Unlock()

		v, ok := rs.pub_fm[src_path]
		if !ok {
			res.Error(404)
			return
		}

		res.Text(v)
	}
}

func (rs *run_state) vite_set_port_handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)

		if r.Method != http.MethodPost {
			res.Error(405)
			return
		}

		port_str := r.URL.Query().Get("port")
		if port_str == "" {
			res.Error(400, "missing port parameter")
			return
		}

		port_int, err := strconv.Atoi(port_str)
		if err != nil {
			rs.log.Error("Error parsing port parameter in vite set port handler",
				"error", err,
			)
			res.Error(400, "invalid port parameter")
			return
		}

		conn, err := net.Dial(
			"tcp",
			net.JoinHostPort(dev_loopback_host, fmt.Sprintf("%d", port_int)),
		)
		if err != nil {
			rs.log.Error("Error connecting to Vite plugin control port",
				"error", err,
				"port", port_int,
			)
			res.Error(500, "error connecting to vite restart port")
			return
		}
		conn.Close()

		rs.mu.Lock()
		defer rs.mu.Unlock()
		rs.vite_plugin_control_port = port_int

		res.OKText()
	}
}

func (rs *run_state) send_vite_plugin_restart() error {
	rs.mu.Lock()
	ctrl_port := rs.vite_plugin_control_port
	rs.mu.Unlock()

	if ctrl_port == 0 {
		return fmt.Errorf("Vite plugin control port not set; cannot restart Vite server")
	}

	resp, err := http.Post(
		fmt.Sprintf("http://%s:%d/cfg-changed", dev_loopback_host, ctrl_port),
		"",
		nil,
	)
	if err != nil {
		return fmt.Errorf("error sending restart command to Vite plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Vite plugin restart request returned status: %s", resp.Status)
	}

	return nil
}
