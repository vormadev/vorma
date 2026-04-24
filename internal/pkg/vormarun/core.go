package vormarun

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/head"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/tsgen"
)

func IsDev() bool {
	return envutil.GetBool(Env_Key_Is_Dev, false)
}

type HeadBuilder = head.Builder
type GoTypeSrc = tsgen.GoTypeSrc

type DevWatchConfig struct {
	// Optional.
	//
	// The outermost directory to watch for changes in dev mode.
	//
	// Defaults to the current working directory.
	Root string

	// Optional.
	//
	// Glob patterns you want the dev watcher to completely ignore.
	//
	// Always ignored: "**/.git" and "**/node_modules".
	GlobalIgnore []string

	// Optional.
	//
	// Glob patterns pointing to Go files (or files implicating Go files, such
	// as embedded templates) that should trigger a Go refresh when changed.
	//
	// If empty, defaults to watching all .go files in DevWatchConfig.Root.
	OnChangeRecompileGo []string

	// Optional.
	//
	// Glob patterns pointing to files that, when changed, should trigger a
	// client-side data revalidation. Useful when editing loader-served
	// content in dev mode, such as markdown files.
	OnChangeClientRevalidate []string
}

type FrontendConfig struct {
	// Required.
	//
	// Must be one of: "react", "preact", "solid"
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
	// Point this to the TypeScript file where you mechanically render your application root.
	RenderEntry string

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
	// Point this to the file where you define your non-critical global application stylesheet.
	// That file's CSS bundle will be imported and applied via a traditional stylesheet link element.
	MainCSSEntry string

	// Optional.
	//
	// Point this to the file where you define your critical global application stylesheet.
	// That file's CSS bundle will be inlined into the HTML document head inside style tags.
	CriticalCSSEntry string
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
	DefaultHead func(*http.Request, *Vorma, *HeadBuilder) error `json:"-"`

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
	// The mount root for your actions router.
	//
	// Default: "/api/"
	APIBase string
}

type Vorma struct {
	ServerEntry    string
	DistDir        string
	PathConfig     PathConfig
	FrontendConfig FrontendConfig
	HTMLConfig     HTMLConfig
	TSGenConfig    TSGenConfig
	DevWatchConfig DevWatchConfig

	init_once                   sync.Once
	init_err                    error
	log                         *slog.Logger
	root_mux                    *mux.Router
	loaders_mux                 *mux.NestedRouter
	actions_mux                 *mux.Router
	static_fs                   fs.FS
	_manifest                   *Manifest
	client_build_id             string
	parsed_tmpl                 *template.Template
	head_renderer               *head.Renderer
	_final_public_filepaths     *set.Set[string]
	supported_methods           *set.Set[string]
	supported_methods_allow_val string
}

type RequestCtx[I any] = mux.RequestCtx[I]

type AnyLoader interface {
	IType() *tsgen.GoTypeSrc
	OType() *tsgen.GoTypeSrc
	GetPattern() string
	GetTSModule() string
	register_to_mux(*mux.NestedRouter)
}
type AnyAction interface {
	IType() *tsgen.GoTypeSrc
	OType() *tsgen.GoTypeSrc
	GetMethod() string
	GetPattern() string
	GetKind() ActionKind
	register_to_mux(*mux.Router)
}

type Loaders []AnyLoader
type Actions []AnyAction

type RequestCtxWrapper[I any, CtxPtr any] interface {
	Wrap(*RequestCtx[I]) CtxPtr
}

type Loader[
	I any,
	O any,
	CtxPtr ~*Ctx,
	Ctx RequestCtxWrapper[I, CtxPtr],
] struct {
	Pattern  string
	Handler  func(CtxPtr) (O, error)
	TSModule string
}

func (l Loader[I, O, CtxPtr, Ctx]) IType() *tsgen.GoTypeSrc {
	return tsgen.GoType[I]()
}

func (l Loader[I, O, CtxPtr, Ctx]) OType() *tsgen.GoTypeSrc {
	return tsgen.GoType[O]()
}

func (l Loader[I, O, CtxPtr, Ctx]) GetPattern() string { return l.Pattern }

func (l Loader[I, O, CtxPtr, Ctx]) GetTSModule() string { return l.TSModule }

func (l Loader[I, O, CtxPtr, Ctx]) register_to_mux(r *mux.NestedRouter) {
	if l.Handler == nil {
		mux.AddNestedTaskHandler(r, l.Pattern, mux.TaskHandlerFromFunc(
			func(ctx *RequestCtx[I]) (mux.None, error) { return mux.None{}, nil },
		))
		return
	}
	mux.AddNestedTaskHandler(r, l.Pattern, mux.TaskHandlerFromFunc(
		func(ctx *RequestCtx[I]) (O, error) {
			var zero Ctx
			return l.Handler(zero.Wrap(ctx))
		},
	))
}

type Action[
	I any,
	O any,
	CtxPtr ~*Ctx,
	Ctx RequestCtxWrapper[I, CtxPtr],
] struct {
	Method  string
	Pattern string
	Kind    ActionKind
	Handler func(CtxPtr) (O, error)
}

type ActionKind string

const (
	ActionKindQuery    ActionKind = "query"
	ActionKindMutation ActionKind = "mutation"
)

func (a Action[I, O, CtxPtr, Ctx]) IType() *tsgen.GoTypeSrc { return tsgen.GoType[I]() }

func (a Action[I, O, CtxPtr, Ctx]) OType() *tsgen.GoTypeSrc { return tsgen.GoType[O]() }

func (a Action[I, O, CtxPtr, Ctx]) GetMethod() string { return a.Method }

func (a Action[I, O, CtxPtr, Ctx]) GetPattern() string { return a.Pattern }

func (a Action[I, O, CtxPtr, Ctx]) GetKind() ActionKind { return a.Kind }

func (a Action[I, O, CtxPtr, Ctx]) register_to_mux(r *mux.Router) {
	if a.Handler == nil {
		mux.AddTaskHandler(r, a.Method, a.Pattern, mux.TaskHandlerFromFunc(
			func(ctx *RequestCtx[mux.None]) (mux.None, error) {
				return mux.None{}, nil
			},
		))
		return
	}
	mux.AddTaskHandler(r, a.Method, a.Pattern, mux.TaskHandlerFromFunc(
		func(ctx *RequestCtx[I]) (O, error) {
			var zero Ctx
			var zero_output O
			data, err := a.Handler(zero.Wrap(ctx))
			if err != nil {
				return zero_output, err
			}
			return data, nil
		},
	))
}
