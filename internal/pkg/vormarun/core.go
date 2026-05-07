package vormarun

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/head"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/tsgen"
)

func IsDev() bool {
	return envutil.GetBool(Env_Key_Is_Dev, false)
}

func IsBuild() bool {
	return envutil.GetBool(Env_Key_Is_Build, false)
}

type HeadBuilder = head.Builder
type GoTypeSrc = tsgen.GoTypeSrc

type DevWatchConfig struct {
	// Optional.
	//
	// Glob patterns defining the filesystem surface to watch in dev mode.
	// Supports ordered globset semantics, including `!` exclusions.
	//
	// If empty, Vorma watches "." after applying built-in exclusions.
	//
	// Always ignored: "**/.git", "**/node_modules", and Vorma's output directory.
	WatchPatterns []string

	// Optional.
	//
	// Glob patterns pointing to Go files (or files implicating Go files, such
	// as embedded templates) that should trigger a Go refresh when changed.
	// Supports ordered globset semantics, including `!` exceptions.
	//
	// If empty, defaults to watching all .go files in the watched surface.
	OnChangeRecompileGo []string

	// Optional.
	//
	// Glob patterns pointing to files that, when changed, should trigger a
	// client-side data revalidation. Useful when editing loader-served
	// content in dev mode, such as markdown files.
	// Supports ordered globset semantics, including `!` exceptions.
	OnChangeClientRevalidate []string
}

type FrontendConfig struct {
	// Required.
	//
	// Must be one of: "react", "preact", "remix", "solid"
	UIVariant string

	// Required.
	//
	// Examples: "pnpm", "yarn", "npx", "bunx"
	JSPackageManagerBaseCmd string

	// Optional.
	//
	// Set this manually if your package.json is not inside your effective current working directory.
	//
	// Default: "."
	JSPackageManagerDir string

	// Optional.
	//
	// If set, this gets passed into the Vite `--config` CLI option.
	ViteConfigFile string

	// Required.
	//
	// Point this to the file where you call `await vorma.boot()`.
	EntryFile string

	// Required.
	//
	// Point this to the directory containing any static files you want to serve from
	// your public static base path.
	// Included files will be hashed and served with immutable cache headers.
	//
	// If you have static files that are already content-addressed and safe to
	// serve immutably, you can place them in `<PublicStaticSrcDir>/__prehashed`
	// to bypass hashing.
	PublicStaticSrcDir string

	// Optional.
	//
	// Point this to the file where you define your critical global application stylesheet.
	// That file's CSS bundle will be inlined into the HTML document head inside style tags.
	CriticalCSSFile string
}

type TSDrafter = tsgen.TSDrafter

type TSGenConfig struct {
	OutFile    string
	ExtraTypes []*GoTypeSrc                  `json:"-"`
	ExtraRawTS func(d *TSDrafter) *TSDrafter `json:"-"`
}

type HTMLConfig struct {
	// Optional.
	//
	// Be sure to render `{{.VormaHead}}` in `{{.VormaBody}}` in the appropriate places
	// in the document.
	//
	// Default:
	//		<!doctype html>
	//		<html lang="en">
	//		<head>
	//		{{.VormaHead}}
	//		</head>
	//		<body>
	//		{{.VormaBody}}
	//		</body>
	//		</html>
	Template string

	// Optional. Gets fed into your HTML template.
	TemplateData func(*http.Request) (map[string]any, error) `json:"-"`

	// Optional, but highly recommended.
	// Determines the default head elements rendered by your website
	// (unless trumped by child loaders).
	DefaultHead func(*http.Request, *Instance, *HeadBuilder) error `json:"-"`

	// Elements specified here will be deduplicated across
	// nested loaders, with the "deepest" (most specific)
	// conflicting route pattern as the ultimate winner.
	HeadDedupeKeys func(*HeadBuilder) `json:"-"`
}

type PathConfig struct {
	// Optional.
	//
	// The base path from which your static public assets will be served.
	//
	// Example: "/public/" (e.g., `<domain>/public/vorma_out_<normalized_file_name><hash>.png`)
	//
	// Default: "/"
	PublicStaticBase string

	// Optional.
	//
	// The mount root for your API router.
	//
	// Default: "/api/"
	APIBase string
}

type Config struct {
	// Required.
	//
	// Your application server entry point (e.g., "./cmd/serve/main.go").
	ServerEntry string
	// Required.
	//
	// The directory where the Vorma-owned `.vorma/` sub-directory will be emitted.
	DistDir        string
	PathConfig     PathConfig
	FrontendConfig FrontendConfig
	HTMLConfig     HTMLConfig
	TSGenConfig    TSGenConfig
	DevWatchConfig DevWatchConfig
}

type Instance struct {
	init_once                    sync.Once
	init_err                     error
	router                       *Router
	router_once                  sync.Once
	cfg                          *Config
	log                          *slog.Logger
	static_fs                    fs.FS
	manifest_cache               *Manifest
	client_build_id_cache        string
	root_template                *template.Template
	head_renderer                *head.Renderer
	final_public_filepaths_cache *set.Set[string]
}

func (inst *Instance) Config() *Config { return inst.cfg }

func (inst *Instance) MustSetStaticFS(fsys fs.FS, dirElems ...string) any {
	if len(dirElems) > 0 {
		inst.static_fs = fsutil.MustSub(fsys, dirElems...)
	} else {
		inst.static_fs = fsys
	}
	return nil
}

type RequestCtx[I any] = mux.RequestCtx[I]

type AnyView interface {
	IType() *tsgen.GoTypeSrc
	OType() *tsgen.GoTypeSrc
	GetPattern() string
	GetClientFile() string
	register_to_mux(*mux.NestedRouter)
}
type AnyAPIRoute interface {
	IType() *tsgen.GoTypeSrc
	OType() *tsgen.GoTypeSrc
	GetMethod() string
	GetPattern() string
	GetKind() APIRouteKind
	register_to_mux(*mux.Router)
}

type Views []AnyView
type APIRoutes []AnyAPIRoute

type RequestCtxWrapper[I, RP any] interface{ Wrap(*RequestCtx[I]) RP }

type View[I, O any, RP ~*R, R RequestCtxWrapper[I, RP]] struct {
	Pattern    string
	Loader     func(RP) (O, error)
	ClientFile string
}

func (view View[I, O, RP, R]) IType() *tsgen.GoTypeSrc {
	return tsgen.GoType[I]()
}

func (view View[I, O, RP, R]) OType() *tsgen.GoTypeSrc {
	return tsgen.GoType[O]()
}

func (view View[I, O, RP, R]) GetPattern() string { return view.Pattern }

func (view View[I, O, RP, R]) GetClientFile() string { return view.ClientFile }

func (view View[I, O, RP, R]) register_to_mux(r *mux.NestedRouter) {
	if view.Loader == nil {
		mux.AddNestedTaskHandler(r, view.Pattern, mux.TaskHandlerFromFunc(
			func(*RequestCtx[I]) (mux.None, error) { return mux.None{}, nil },
		))
		return
	}
	mux.AddNestedTaskHandler(r, view.Pattern, mux.TaskHandlerFromFunc(
		func(c *RequestCtx[I]) (O, error) {
			var zero R
			return view.Loader(zero.Wrap(c))
		},
	))
}

type APIRoute[I, O any, RP ~*R, R RequestCtxWrapper[I, RP]] struct {
	Method  string
	Pattern string
	Kind    APIRouteKind
	Handler func(RP) (O, error)
}

type APIRouteKind string

const (
	APIRouteKindQuery    APIRouteKind = "query"
	APIRouteKindMutation APIRouteKind = "mutation"
)

func (a APIRoute[I, O, RP, R]) IType() *tsgen.GoTypeSrc { return tsgen.GoType[I]() }

func (a APIRoute[I, O, RP, R]) OType() *tsgen.GoTypeSrc { return tsgen.GoType[O]() }

func (a APIRoute[I, O, RP, R]) GetMethod() string { return a.Method }

func (a APIRoute[I, O, RP, R]) GetPattern() string { return a.Pattern }

func (a APIRoute[I, O, RP, R]) GetKind() APIRouteKind { return a.Kind }

func (a APIRoute[I, O, RP, R]) register_to_mux(r *mux.Router) {
	if a.Handler == nil {
		mux.AddTaskHandler(r, a.Method, a.Pattern, mux.TaskHandlerFromFunc(
			func(*RequestCtx[mux.None]) (mux.None, error) {
				return mux.None{}, nil
			},
		))
		return
	}
	mux.AddTaskHandler(r, a.Method, a.Pattern, mux.TaskHandlerFromFunc(
		func(c *RequestCtx[I]) (O, error) {
			var zero R
			var zero_output O
			data, err := a.Handler(zero.Wrap(c))
			if err != nil {
				return zero_output, err
			}
			return data, nil
		},
	))
}
