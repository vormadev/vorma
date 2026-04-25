package scenario

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
const variant_env_key = "VORMA_BOMBADIL_VARIANT"
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

type RootData struct {
	Name string
}

type CounterInput struct {
	N int `json:"n"`
}

type CounterData struct {
	Value int
}

type SlowInput struct {
	DelayMS int `json:"delay_ms"`
}

type SlowData struct {
	DelayMS int
	Stamp   string
}

type EchoData struct {
	Message string
}

type ItemData struct {
	ID string
}

type ClientData struct {
	ID          string
	ServerStamp string
}

type NestedData struct {
	Section string
}

type NestedDetailData struct {
	ID      string
	Section string
}

type CountActionInput struct {
	Delta int `json:"delta"`
}

type CountActionData struct {
	Next int
}

type EchoActionInput struct {
	Message string
}

type EchoActionData struct {
	Message string
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
	build.Run(build.RunArgs{
		App:     v.App(),
		Loaders: v.Loaders(),
		Actions: v.Actions(),
		Caller:  build.CaptureCaller(pc, file, line, ok),
	})
}

func (v Variant) Serve(static_fs fs.FS) {
	port := envutil.GetInt("PORT", 0)
	if port == 0 {
		panic("PORT env var must be set to a valid integer")
	}

	app := v.App()
	r, err := vorma.InitRouter(app, v.Loaders(), v.Actions(), static_fs)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize router: %v", err))
	}

	static_mw, err := app.PublicFileServerMiddleware()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize public file server middleware: %v", err))
	}
	r.AddGlobalHTTPMiddleware(static_mw)

	addr := fmt.Sprintf(":%d", port)
	url := "http://localhost" + addr
	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	fmt.Printf("Starting %s Bombadil fixture server at %s\n", v.UIVariant, url)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("Application server failed: %v", err))
	}
}

func (v Variant) ServeFromDisk() {
	v.Serve(os.DirFS(filepath.Join(v.DistDir, ".vorma", "static")))
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

func (v Variant) App() *vorma.Vorma {
	return &vorma.Vorma{
		DistDir:     v.DistDir,
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

func (v Variant) Loaders() vorma.Loaders {
	return vorma.Loaders{
		Loader[struct{}, RootData]{
			Pattern:  route_root_pattern,
			TSModule: v.route_module("root.ts"),
			Handler: func(c *RequestCtx[struct{}]) (RootData, error) {
				c.HeadBuilder().Title("Vorma Bombadil Fixture")
				return RootData{Name: "root"}, nil
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
				return CounterData{Value: value}, nil
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
					DelayMS: delay_ms,
					Stamp:   strconv.FormatInt(time.Now().UnixNano(), 10),
				}, nil
			},
		},

		Loader[struct{}, EchoData]{
			Pattern:  route_echo_pattern,
			TSModule: v.route_module("echo.ts"),
			Handler: func(c *RequestCtx[struct{}]) (EchoData, error) {
				c.HeadBuilder().Title("Echo")
				return EchoData{Message: "ready"}, nil
			},
		},

		Loader[struct{}, ItemData]{
			Pattern:  route_item_pattern,
			TSModule: v.route_module("item.ts"),
			Handler: func(c *RequestCtx[struct{}]) (ItemData, error) {
				id := c.Param("id")
				c.HeadBuilder().Title("Item " + id)
				return ItemData{ID: id}, nil
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
				}, nil
			},
		},

		Loader[struct{}, NestedData]{
			Pattern:  route_nested_pattern,
			TSModule: v.route_module("nested.ts"),
			Handler: func(c *RequestCtx[struct{}]) (NestedData, error) {
				c.HeadBuilder().Title("Nested")
				return NestedData{Section: "nested"}, nil
			},
		},

		Loader[struct{}, NestedDetailData]{
			Pattern:  route_nested_detail_pattern,
			TSModule: v.route_module("nested_detail.ts"),
			Handler: func(c *RequestCtx[struct{}]) (NestedDetailData, error) {
				id := c.Param("id")
				c.HeadBuilder().Title("Nested " + id)
				return NestedDetailData{
					ID:      id,
					Section: "nested",
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

func (v Variant) Actions() vorma.Actions {
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
				return CountActionData{Next: next}, nil
			},
		},

		Action[EchoActionInput, EchoActionData]{
			Method:  http.MethodPost,
			Pattern: action_echo_pattern,
			Kind:    vorma.ActionKindMutation,
			Handler: func(c *RequestCtx[EchoActionInput]) (EchoActionData, error) {
				return EchoActionData{Message: c.Input().Message}, nil
			},
		},
	}
}

func (v Variant) route_module(name string) string {
	return filepath.ToSlash(filepath.Join(route_module_root, name))
}
