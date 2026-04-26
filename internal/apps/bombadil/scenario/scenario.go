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
const deployment_a = "A"
const deployment_b = "B"
const switch_path = "/__bombadil/switch"
const deployment_path = "/__bombadil/deployment"
const server_entry = "cmd/serve"
const vite_config_file = "vite.config.ts"
const render_entry = "vorma.entry.ts"
const route_module_root = "components/routes"

type (
	Loader[I any, O any] = vorma.Loader[I, O, *RequestCtx[I], RequestCtx[I]]
	Action[I any, O any] = vorma.Action[I, O, *RequestCtx[I], RequestCtx[I]]
	RequestCtx[I any]    struct{ *vorma.RequestCtx[I] }
)

type Variant struct {
	UIVariant    string
	DistDir      string
	TSGenOutFile string
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
	UIVariant:    "react",
	DistDir:      ".dist.react",
	TSGenOutFile: "vorma.react.gen.ts",
}

var Preact = Variant{
	UIVariant:    "preact",
	DistDir:      ".dist.preact",
	TSGenOutFile: "vorma.preact.gen.ts",
}

var Solid = Variant{
	UIVariant:    "solid",
	DistDir:      ".dist.solid",
	TSGenOutFile: "vorma.solid.gen.ts",
}

func (RequestCtx[I]) Wrap(c *vorma.RequestCtx[I]) *RequestCtx[I] {
	return &RequestCtx[I]{RequestCtx: c}
}

func (v Variant) Build(pc uintptr, file string, line int, ok bool) {
	d := SelectedDeployment()
	build.Run(build.RunArgs{
		App:     v.App(d),
		Loaders: v.loaders(d),
		Actions: v.actions(d),
		Caller:  build.CaptureCaller(pc, file, line, ok),
	})
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
		h, err := v.deployment_handler(static_fs, d)
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

func (v Variant) deployment_handler(static_fs fs.FS, d deployment_variant) (http.Handler, error) {
	app := v.App(d)
	r, err := vorma.InitRouter(
		app,
		v.loaders(d),
		v.actions(d),
		static_fs,
	)
	if err != nil {
		return nil, err
	}

	static_mw, err := app.PublicFileServerMiddleware()
	if err != nil {
		return nil, err
	}
	r.AddGlobalHTTPMiddleware(static_mw)
	return r, nil
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

	fmt.Printf("Starting %s Bombadil fixture server at %s\n", v.UIVariant, url)
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

func (v Variant) App(d deployment_variant) *vorma.Vorma {
	return &vorma.Vorma{
		DistDir:     v.deployment_dist_dir(d),
		ServerEntry: server_entry,

		PathConfig: vorma.PathConfig{
			PublicStaticBase: "/",
			APIBase:          "/api/",
		},

		FrontendConfig: vorma.FrontendConfig{
			UIVariant:               v.UIVariant,
			JSPackageManagerBaseCmd: "pnpm",
			JSPackageManagerDir:     ".",
			ViteConfigFile:          vite_config_file,
			RenderEntry:             render_entry,
			PublicStaticSrcDir:      "public",
			MainCSSEntry:            "shared/styles/main.css",
			CriticalCSSEntry:        "shared/styles/main.critical.css",
		},

		HTMLConfig: vorma.HTMLConfig{
			DefaultHead: func(_ *http.Request, _ *vorma.Vorma, h *vorma.HeadBuilder) error {
				h.MetaCharset("utf-8")
				h.MetaNameContent("viewport", "width=device-width, initial-scale=1")
				h.Title("Vorma Bombadil Fixture")
				h.Description("A small Vorma app for Bombadil browser fuzzing.")
				return nil
			},
			HeadDedupeKeys: func(*vorma.HeadBuilder) {},
		},

		TSGenConfig: vorma.TSGenConfig{
			OutFile: v.TSGenOutFile,
		},

		DevWatchConfig: vorma.DevWatchConfig{
			Root:                     ".",
			GlobalIgnore:             []string{".bombadil/**"},
			OnChangeRecompileGo:      []string{"scenario/**/*.go"},
			OnChangeClientRevalidate: []string{},
		},
	}
}

func (v Variant) deployment_dist_dir(d deployment_variant) string {
	return v.DistDir + "." + d.dist_suffix
}

func (v Variant) loaders(d deployment_variant) vorma.Loaders {
	return vorma.Loaders{
		Loader[struct{}, RootData]{
			Pattern:  route_root_pattern,
			TSModule: v.route_module("root.ts"),
			Handler: func(c *RequestCtx[struct{}]) (RootData, error) {
				c.HeadBuilder().Title("Vorma Bombadil Fixture")
				return RootData{Name: "root", Deployment: d.data_suffix}, nil
			},
		},

		Loader[CounterInput, CounterData]{
			Pattern:  route_counter_pattern,
			TSModule: v.route_module("counter.ts"),
			Handler: func(c *RequestCtx[CounterInput]) (CounterData, error) {
				value := c.Input().N
				if value < -5 {
					value = -5
				}
				if value > 5 {
					value = 5
				}
				c.HeadBuilder().Title(fmt.Sprintf("Counter %d", value))
				return CounterData{
					Value:      value,
					Deployment: d.data_suffix,
				}, nil
			},
		},

		Loader[SlowInput, SlowData]{
			Pattern:  route_slow_pattern,
			TSModule: v.route_module("slow.ts"),
			Handler: func(c *RequestCtx[SlowInput]) (SlowData, error) {
				delay_ms := c.Input().DelayMS
				if delay_ms < 0 {
					delay_ms = 0
				}
				if delay_ms > 250 {
					delay_ms = 250
				}
				time.Sleep(time.Duration(delay_ms) * time.Millisecond)
				c.HeadBuilder().Title("Slow Route")
				return SlowData{
					DelayMS:    delay_ms,
					Stamp:      strconv.FormatInt(time.Now().UnixNano(), 10),
					Deployment: d.data_suffix,
				}, nil
			},
		},

		Loader[struct{}, EchoData]{
			Pattern:  route_echo_pattern,
			TSModule: v.route_module("echo.ts"),
			Handler: func(c *RequestCtx[struct{}]) (EchoData, error) {
				c.HeadBuilder().Title("Echo")
				return EchoData{
					Message:    "ready",
					Deployment: d.data_suffix,
				}, nil
			},
		},

		Loader[struct{}, ItemData]{
			Pattern:  route_item_pattern,
			TSModule: v.route_module("item.ts"),
			Handler: func(c *RequestCtx[struct{}]) (ItemData, error) {
				id := c.Param("id")
				c.HeadBuilder().Title("Item " + id)
				return ItemData{ID: id, Deployment: d.data_suffix}, nil
			},
		},

		Loader[struct{}, ClientData]{
			Pattern:  route_client_pattern,
			TSModule: v.route_module("client.ts"),
			Handler: func(c *RequestCtx[struct{}]) (ClientData, error) {
				id := c.Param("id")
				c.HeadBuilder().Title("Client " + id)
				return ClientData{
					ID:          id,
					ServerStamp: strconv.FormatInt(time.Now().UnixNano(), 10),
					Deployment:  d.data_suffix,
				}, nil
			},
		},

		Loader[struct{}, NestedData]{
			Pattern:  route_nested_pattern,
			TSModule: v.route_module("nested.ts"),
			Handler: func(c *RequestCtx[struct{}]) (NestedData, error) {
				c.HeadBuilder().Title("Nested")
				return NestedData{
					Section:    "nested",
					Deployment: d.data_suffix,
				}, nil
			},
		},

		Loader[struct{}, NestedDetailData]{
			Pattern:  route_nested_detail_pattern,
			TSModule: v.route_module("nested_detail.ts"),
			Handler: func(c *RequestCtx[struct{}]) (NestedDetailData, error) {
				id := c.Param("id")
				c.HeadBuilder().Title("Nested " + id)
				return NestedDetailData{
					ID:         id,
					Section:    "nested",
					Deployment: d.data_suffix,
				}, nil
			},
		},

		Loader[struct{}, struct{}]{
			Pattern:  route_fail_pattern,
			TSModule: v.route_module("fail.ts"),
			Handler: func(c *RequestCtx[struct{}]) (struct{}, error) {
				c.SetResponseStatus(500)
				return struct{}{}, &vorma.LoaderError{
					ClientMsg: "Fixture loader failed on purpose.",
				}
			},
		},
	}
}

func (v Variant) actions(d deployment_variant) vorma.Actions {
	return vorma.Actions{
		Action[CountActionInput, CountActionData]{
			Method:  http.MethodGet,
			Pattern: action_count_pattern,
			Kind:    vorma.ActionKindQuery,
			Handler: func(c *RequestCtx[CountActionInput]) (CountActionData, error) {
				next := c.Input().Delta
				if next < -5 {
					next = -5
				}
				if next > 5 {
					next = 5
				}
				return CountActionData{Next: next, Deployment: d.data_suffix}, nil
			},
		},

		Action[EchoActionInput, EchoActionData]{
			Method:  http.MethodPost,
			Pattern: action_echo_pattern,
			Kind:    vorma.ActionKindMutation,
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
