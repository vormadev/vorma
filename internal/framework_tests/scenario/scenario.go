package scenario

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/build"
	"github.com/vormadev/vorma/kit/envutil"
)

const route_root_pattern = "/"
const route_counter_pattern = "/counter"
const route_slow_pattern = "/slow"
const route_echo_pattern = "/echo"
const route_item_pattern = "/items/:id"
const route_client_pattern = "/client/:id"
const route_nested_pattern = "/nested"
const route_nested_detail_pattern = "/nested/:id/details"
const route_fail_pattern = "/fail"

const action_echo_pattern = "/echo"
const action_count_pattern = "/count"
const echo_action_fail_message = "__bombadil_fail__"
const variant_env_key = "VORMA_BOMBADIL_VARIANT"
const deployment_env_key = "VORMA_BOMBADIL_DEPLOYMENT"
const mode_env_key = "VORMA_BOMBADIL_MODE"
const panic_route_env_key = "VORMA_BOMBADIL_ENABLE_PANIC_ROUTE"
const exit_route_env_key = "VORMA_BOMBADIL_ENABLE_EXIT_ROUTE"
const deployment_a = "A"
const deployment_b = "B"
const mode_dev = "dev"
const mode_prod = "prod"
const switch_path = "/__bombadil/switch"
const deployment_path = "/__bombadil/deployment"
const panic_path = "/__bombadil/panic"
const exit_path = "/__bombadil/exit"
const server_entry = "cmd/serve"
const render_entry = "vorma.entry.ts"
const route_module_root = "components/routes"

type (
	View[I, O any]     = vorma.View[I, O, *RequestCtx[I], RequestCtx[I]]
	APIRoute[I, O any] = vorma.APIRoute[I, O, *RequestCtx[I], RequestCtx[I]]
	RequestCtx[I any]  struct{ *vorma.RequestCtx[I] }
)

func (RequestCtx[I]) Wrap(c *vorma.RequestCtx[I]) *RequestCtx[I] {
	return &RequestCtx[I]{RequestCtx: c}
}

type Variant struct {
	UIVariant       string
	DistDir         string
	TSGenOutFile    string
	DevTSGenOutFile string
	ViteConfigFile  string
}

type deployment_variant struct {
	name        string
	dist_suffix string
	data_suffix string
}

type deployment struct {
	name    string
	handler http.Handler
}

type switchboard struct {
	mu          sync.RWMutex
	deployments map[string]deployment
	current     string
}

type RootData struct {
	Name       string
	Deployment string
}

type CounterInput struct {
	N int `json:"n"`
}

type CounterData struct {
	Value      int
	Deployment string
}

type SlowInput struct {
	DelayMS int `json:"delay_ms"`
}

type SlowData struct {
	DelayMS    int
	Stamp      string
	Deployment string
}

type EchoData struct {
	Message    string
	Deployment string
}

type ItemData struct {
	ID         string
	Deployment string
}

type ClientData struct {
	ID          string
	ServerStamp string
	Deployment  string
}

type NestedData struct {
	Section    string
	Deployment string
}

type NestedDetailData struct {
	ID         string
	Section    string
	Deployment string
}

type CountActionInput struct {
	Delta int `json:"delta"`
}

type CountActionData struct {
	Next       int
	Deployment string
}

type EchoActionInput struct {
	Message string
}

type EchoActionData struct {
	Message    string
	Deployment string
}

var deployment_a_variant = deployment_variant{
	name:        deployment_a,
	dist_suffix: "a",
	data_suffix: "from-A",
}

var deployment_b_variant = deployment_variant{
	name:        deployment_b,
	dist_suffix: "b",
	data_suffix: "from-B",
}

var React = Variant{
	UIVariant:       "react",
	DistDir:         ".dist.react",
	TSGenOutFile:    "vorma.react.gen.ts",
	DevTSGenOutFile: "vorma.react.dev.gen.ts",
	ViteConfigFile:  "vite.react.config.ts",
}

var Preact = Variant{
	UIVariant:       "preact",
	DistDir:         ".dist.preact",
	TSGenOutFile:    "vorma.preact.gen.ts",
	DevTSGenOutFile: "vorma.preact.dev.gen.ts",
	ViteConfigFile:  "vite.preact.config.ts",
}

var Solid = Variant{
	UIVariant:       "solid",
	DistDir:         ".dist.solid",
	TSGenOutFile:    "vorma.solid.gen.ts",
	DevTSGenOutFile: "vorma.solid.dev.gen.ts",
	ViteConfigFile:  "vite.solid.config.ts",
}

func (v Variant) Build(pc uintptr, file string, line int, ok bool) {
	d := SelectedDeployment()
	build.Run(v.Router(nil, d), build.Caller(pc, file, line, ok))
}

func (v Variant) Serve(static_fs fs.FS) {
	board, err := v.switchboard(map[string]fs.FS{
		deployment_a: static_fs,
		deployment_b: static_fs,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize switchboard: %v", err))
	}
	v.serve_board(board)
}

func (v Variant) switchboard(static_fss map[string]fs.FS) (*switchboard, error) {
	deployments := make(map[string]deployment)
	for _, d := range []deployment_variant{deployment_a_variant, deployment_b_variant} {
		static_fs, ok := static_fss[d.name]
		if !ok {
			return nil, fmt.Errorf("missing static fs for deployment %s", d.name)
		}
		h, err := v.Router(static_fs, d)()
		if err != nil {
			return nil, err
		}
		deployments[d.name] = deployment{
			name:    d.name,
			handler: h,
		}
	}
	return &switchboard{
		deployments: deployments,
		current:     deployment_a,
	}, nil
}

func (v Variant) ServeFromDisk() {
	static_fss := map[string]fs.FS{
		deployment_a: os.DirFS(
			filepath.Join(v.deployment_dist_dir(deployment_a_variant), ".vorma", "static"),
		),
		deployment_b: os.DirFS(
			filepath.Join(v.deployment_dist_dir(deployment_b_variant), ".vorma", "static"),
		),
	}
	board, err := v.switchboard(static_fss)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize switchboard: %v", err))
	}
	v.serve_board(board)
}

func (v Variant) ServeSelectedFromDisk() {
	d := SelectedDeployment()
	h, err := v.Router(
		os.DirFS(filepath.Join(v.deployment_dist_dir(d), ".vorma", "static")),
		d,
	)()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize deployment: %v", err))
	}
	board := &switchboard{
		deployments: map[string]deployment{
			d.name: {
				name:    d.name,
				handler: h,
			},
		},
		current: d.name,
	}
	v.serve_board(board)
}

func (v Variant) serve_board(board *switchboard) {
	port := envutil.GetInt("PORT", 0)
	if port == 0 {
		panic("PORT env var must be set to a valid integer")
	}

	addr := fmt.Sprintf(":%d", port)
	url := "http://localhost" + addr
	server := &http.Server{
		Addr:    addr,
		Handler: board,
	}

	fmt.Printf("Starting %s framework test server at %s\n", v.UIVariant, url)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("Application server failed: %v", err))
	}
}

func SelectedVariant() Variant {
	switch os.Getenv(variant_env_key) {
	case "react":
		return React
	case "preact":
		return Preact
	case "solid":
		return Solid
	default:
		panic(fmt.Sprintf("%s must be one of: react, preact, solid", variant_env_key))
	}
}

func SelectedDeployment() deployment_variant {
	switch os.Getenv(deployment_env_key) {
	case "", deployment_a:
		return deployment_a_variant
	case deployment_b:
		return deployment_b_variant
	default:
		panic(
			fmt.Sprintf(
				"%s must be one of: %s, %s",
				deployment_env_key,
				deployment_a,
				deployment_b,
			),
		)
	}
}

func SelectedMode() string {
	switch os.Getenv(mode_env_key) {
	case "", mode_prod:
		return mode_prod
	case mode_dev:
		return mode_dev
	default:
		panic(
			fmt.Sprintf(
				"%s must be one of: %s, %s",
				mode_env_key,
				mode_prod,
				mode_dev,
			),
		)
	}
}

func (v Variant) Config(static_fs fs.FS, d deployment_variant) *vorma.Config {
	ts_gen_out_file := v.TSGenOutFile
	if SelectedMode() == mode_dev {
		ts_gen_out_file = v.DevTSGenOutFile
	}

	return &vorma.Config{
		ServerEntry: server_entry,

		DistConfig: vorma.DistConfig{
			OutDir:   v.deployment_dist_dir(d),
			StaticFS: static_fs,
		},

		PathConfig: vorma.PathConfig{
			PublicStaticBase: "/",
			APIBase:          "/api/",
		},

		FrontendConfig: vorma.FrontendConfig{
			UIVariant:               v.UIVariant,
			JSPackageManagerBaseCmd: "pnpm",
			JSPackageManagerDir:     ".",
			ViteConfigFile:          v.ViteConfigFile,
			RenderEntry:             render_entry,
			PublicStaticSrcDir:      "public",
			CriticalCSSEntry:        "shared/styles/main.critical.css",
		},

		HTMLConfig: vorma.HTMLConfig{
			DefaultHead: func(_ *http.Request, _ *vorma.Instance, h *vorma.HeadBuilder) error {
				h.MetaCharset("utf-8")
				h.MetaNameContent("viewport", "width=device-width, initial-scale=1")
				h.Title("Vorma Framework Test App")
				h.Description("A small Vorma app for framework runtime testing.")
				return nil
			},
			HeadDedupeKeys: func(*vorma.HeadBuilder) {},
		},

		TSGenConfig: vorma.TSGenConfig{
			OutFile: ts_gen_out_file,
		},

		DevWatchConfig: vorma.DevWatchConfig{
			WatchRoot:                ".",
			GlobalIgnore:             []string{".bombadil/**"},
			OnChangeRecompileGo:      []string{"scenario/**/*.go"},
			OnChangeClientRevalidate: []string{},
		},
	}
}

func (v Variant) Router(static_fs fs.FS, d deployment_variant) func() (*vorma.Router, error) {
	instance := vorma.New(v.Config(static_fs, d))
	return func() (*vorma.Router, error) {
		r, err := instance.Router()
		if err != nil {
			return nil, err
		}
		r.MustAddPublicFileServerMiddleware()
		for _, view := range v.views(d) {
			r.View(view)
		}
		for _, api_route := range v.api_routes(d) {
			r.APIRoute(api_route)
		}
		return r, nil
	}
}

func (v Variant) deployment_dist_dir(d deployment_variant) string {
	if SelectedMode() == mode_dev {
		return v.DistDir + ".dev." + d.dist_suffix
	}
	return v.DistDir + "." + d.dist_suffix
}

func (v Variant) views(d deployment_variant) vorma.Views {
	return vorma.Views{
		View[struct{}, RootData]{
			Pattern:      route_root_pattern,
			ClientModule: v.route_module("root.ts"),
			Loader: func(c *RequestCtx[struct{}]) (RootData, error) {
				c.HeadBuilder().Title("Vorma Framework Test App")
				return RootData{Name: "root", Deployment: d.data_suffix}, nil
			},
		},

		View[CounterInput, CounterData]{
			Pattern:      route_counter_pattern,
			ClientModule: v.route_module("counter.ts"),
			Loader: func(c *RequestCtx[CounterInput]) (CounterData, error) {
				value := min(max(c.Input().N, -5), 5)
				c.HeadBuilder().Title(fmt.Sprintf("Counter %d", value))
				return CounterData{
					Value:      value,
					Deployment: d.data_suffix,
				}, nil
			},
		},

		View[SlowInput, SlowData]{
			Pattern:      route_slow_pattern,
			ClientModule: v.route_module("slow.ts"),
			Loader: func(c *RequestCtx[SlowInput]) (SlowData, error) {
				delay_ms := min(max(c.Input().DelayMS, 0), 250)
				time.Sleep(time.Duration(delay_ms) * time.Millisecond)
				c.HeadBuilder().Title("Slow Route")
				return SlowData{
					DelayMS:    delay_ms,
					Stamp:      strconv.FormatInt(time.Now().UnixNano(), 10),
					Deployment: d.data_suffix,
				}, nil
			},
		},

		View[struct{}, EchoData]{
			Pattern:      route_echo_pattern,
			ClientModule: v.route_module("echo.ts"),
			Loader: func(c *RequestCtx[struct{}]) (EchoData, error) {
				c.HeadBuilder().Title("Echo")
				return EchoData{
					Message:    "ready",
					Deployment: d.data_suffix,
				}, nil
			},
		},

		View[struct{}, ItemData]{
			Pattern:      route_item_pattern,
			ClientModule: v.route_module("item.ts"),
			Loader: func(c *RequestCtx[struct{}]) (ItemData, error) {
				id := c.Param("id")
				c.HeadBuilder().Title("Item " + id)
				return ItemData{ID: id, Deployment: d.data_suffix}, nil
			},
		},

		View[struct{}, ClientData]{
			Pattern:      route_client_pattern,
			ClientModule: v.route_module("client.ts"),
			Loader: func(c *RequestCtx[struct{}]) (ClientData, error) {
				id := c.Param("id")
				c.HeadBuilder().Title("Client " + id)
				return ClientData{
					ID:          id,
					ServerStamp: strconv.FormatInt(time.Now().UnixNano(), 10),
					Deployment:  d.data_suffix,
				}, nil
			},
		},

		View[struct{}, NestedData]{
			Pattern:      route_nested_pattern,
			ClientModule: v.route_module("nested.ts"),
			Loader: func(c *RequestCtx[struct{}]) (NestedData, error) {
				c.HeadBuilder().Title("Nested")
				return NestedData{
					Section:    "nested",
					Deployment: d.data_suffix,
				}, nil
			},
		},

		View[struct{}, NestedDetailData]{
			Pattern:      route_nested_detail_pattern,
			ClientModule: v.route_module("nested_detail.ts"),
			Loader: func(c *RequestCtx[struct{}]) (NestedDetailData, error) {
				id := c.Param("id")
				c.HeadBuilder().Title("Nested " + id)
				return NestedDetailData{
					ID:         id,
					Section:    "nested",
					Deployment: d.data_suffix,
				}, nil
			},
		},

		View[struct{}, struct{}]{
			Pattern:      route_fail_pattern,
			ClientModule: v.route_module("fail.ts"),
			Loader: func(c *RequestCtx[struct{}]) (struct{}, error) {
				c.SetResponseStatus(500)
				return struct{}{}, &vorma.LoaderError{
					ClientMsg: "Fixture loader failed on purpose.",
				}
			},
		},
	}
}

func (v Variant) api_routes(d deployment_variant) vorma.APIRoutes {
	return vorma.APIRoutes{
		APIRoute[CountActionInput, CountActionData]{
			Method:  http.MethodGet,
			Pattern: action_count_pattern,
			Kind:    vorma.APIRouteKindQuery,
			Handler: func(c *RequestCtx[CountActionInput]) (CountActionData, error) {
				next := min(max(c.Input().Delta, -5), 5)
				return CountActionData{Next: next, Deployment: d.data_suffix}, nil
			},
		},

		APIRoute[EchoActionInput, EchoActionData]{
			Method:  http.MethodPost,
			Pattern: action_echo_pattern,
			Kind:    vorma.APIRouteKindMutation,
			Handler: func(c *RequestCtx[EchoActionInput]) (EchoActionData, error) {
				if c.Input().Message == echo_action_fail_message {
					c.SetResponseStatus(http.StatusConflict, "Fixture action failed on purpose.")
					return EchoActionData{
						Message:    "",
						Deployment: d.data_suffix,
					}, nil
				}
				return EchoActionData{
					Message:    c.Input().Message,
					Deployment: d.data_suffix,
				}, nil
			},
		},
	}
}

func (v Variant) route_module(name string) string {
	return filepath.ToSlash(filepath.Join(route_module_root, name))
}

func (s *switchboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/__bombadil/") {
		s.serve_control(w, r)
		return
	}

	s.mu.RLock()
	h := s.deployments[s.current].handler
	s.mu.RUnlock()
	h.ServeHTTP(w, r)
}

func (s *switchboard) serve_control(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case panic_path:
		if envutil.GetBool(panic_route_env_key, false) {
			panic("framework test panic route")
		}
		w.WriteHeader(http.StatusNotFound)
	case exit_path:
		if envutil.GetBool(exit_route_env_key, false) {
			w.WriteHeader(http.StatusNoContent)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			go func() {
				time.Sleep(50 * time.Millisecond)
				os.Exit(86)
			}()
			return
		}
		w.WriteHeader(http.StatusNotFound)
	case deployment_path:
		s.write_deployment(w)
	case switch_path:
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.switch_to(r.URL.Query().Get("to"))
		s.write_deployment(w)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *switchboard) switch_to(to string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.deployments[to]; ok {
		s.current = to
		return
	}
	if len(s.deployments) <= 1 {
		return
	}
	if s.current == deployment_a {
		s.current = deployment_b
		return
	}
	s.current = deployment_a
}

func (s *switchboard) write_deployment(w http.ResponseWriter) {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"deployment":%q}`, current)
}
