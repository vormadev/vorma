package vorma2

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vormadev/vorma/internal/pkg/prodcache"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/validate"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/vorma2/internal/constants"
	"github.com/vormadev/vorma/vorma2/internal/types"
	"github.com/vormadev/vorma/wave"
)

/////////////////////////////////////////////////////////////////////
/////// Public type aliases
/////////////////////////////////////////////////////////////////////

type (
	HeadEls              = headels.HeadEls
	None                 = mux.None
	Action[I any, O any] = mux.TaskHandler[I, O]
	Loader[O any]        = mux.TaskHandler[None, O]
	LoaderReqData        = mux.ReqData[mux.None]
	ActionReqData[I any] = mux.ReqData[I]
	AdHocType            = tsgen.AdHocType

	LoaderFunc[Ctx any, O any]        = func(*Ctx) (O, error)
	ActionFunc[Ctx any, I any, O any] = func(*Ctx) (O, error)
)

const ClientBuildIDHeaderKey = "X-Vorma-Client-Build-Id"

type FormData struct{}

func (m FormData) TSTypeRaw() string { return "FormData" }

func IsJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(json_query_key) != ""
}

func EnableThirdPartyRouter(next http.Handler) http.Handler {
	return mux.InjectTasksCtxMiddleware(next)
}

/////////////////////////////////////////////////////////////////////
/////// Callback types
/////////////////////////////////////////////////////////////////////

type (
	DefaultHeadElsFunc   = func(r *http.Request, app *Vorma, head *HeadEls) error
	HeadDedupeKeysFunc   = func(head *HeadEls)
	RootTemplateDataFunc = func(r *http.Request) (map[string]any, error)
)

/////////////////////////////////////////////////////////////////////
/////// Router options
/////////////////////////////////////////////////////////////////////

type LoadersRouterOptions struct {
	DynamicParamPrefix             rune
	SplatSegmentIdentifier         rune
	ExplicitIndexSegmentIdentifier string
}

type ActionsRouterOptions struct {
	DynamicParamPrefix     rune
	SplatSegmentIdentifier rune
	MountRoot              string
	SupportedMethods       []string
}

/////////////////////////////////////////////////////////////////////
/////// Vorma
/////////////////////////////////////////////////////////////////////

// Do not instantiate directly. Use `vorma2.NewVormaApp()` instead.
type Vorma struct {
	*wave.Wave                  // set via Init() at runtime, not needed at buildtime
	get_wave                    func() *wave.Wave
	logger                      *slog.Logger
	loaders_router              *mux.NestedRouter
	actions_router              *mux.Router
	supported_methods           map[string]bool
	supported_methods_allow     string
	get_default_head_els        DefaultHeadElsFunc
	get_head_dedupe_keys        HeadDedupeKeysFunc
	get_root_tmpl_data          RootTemplateDataFunc
	ad_hoc_types                []*AdHocType
	extra_ts_code               string
	head_els_inst               *headels.Instance
	runtime_snapshot            *prodcache.Cache[*types.RuntimeSnapshot]
	root_template               *prodcache.Cache[*template.Template]
	route_data_cache            sync.Map
	route_data_cache_client_bid atomic.Value // last client build ID seen by the cache
	static_prod_head_cache      sync.Map     // static_head_cache_key → template.HTML
	patterns_bid                atomic.Value // server build ID for which patterns are registered
	server_addr                 string

	loaders_handler_once sync.Once
	loaders_handler      mux.TasksCtxRequirerFunc
	actions_handler_once sync.Once
	actions_handler      mux.TasksCtxRequirerFunc
}

type VormaAppConfig struct {
	Wave                 func() *wave.Wave
	DefaultHeadElsFunc   DefaultHeadElsFunc
	HeadDedupeKeysFunc   HeadDedupeKeysFunc
	RootTemplateDataFunc RootTemplateDataFunc
	LoadersRouterOptions LoadersRouterOptions
	ActionsRouterOptions ActionsRouterOptions
	AdHocTypes           []*AdHocType
	ExtraTSCode          string
	Logger               *slog.Logger
}

func NewVormaApp(o VormaAppConfig) *Vorma {
	if o.Wave == nil {
		panic("[vorma2]: Wave is required")
	}

	v := &Vorma{
		get_wave:             o.Wave,
		logger:               o.Logger,
		get_default_head_els: o.DefaultHeadElsFunc,
		get_head_dedupe_keys: o.HeadDedupeKeysFunc,
		get_root_tmpl_data:   o.RootTemplateDataFunc,
		ad_hoc_types:         o.AdHocTypes,
		extra_ts_code:        o.ExtraTSCode,
	}
	if v.logger == nil {
		v.logger = colorlog.New("vorma2")
	}

	// head elements instance (reused across requests)
	v.head_els_inst = headels.NewInstance("vorma")
	if o.HeadDedupeKeysFunc != nil {
		dedupe_head := headels.New()
		o.HeadDedupeKeysFunc(dedupe_head)
		v.head_els_inst.InitUniqueRules(dedupe_head)
	}

	// loaders router
	explicit_index := o.LoadersRouterOptions.ExplicitIndexSegmentIdentifier
	if explicit_index == "" {
		explicit_index = "_index"
	}
	v.loaders_router = mux.NewNestedRouter(&mux.NestedOptions{
		DynamicParamPrefix:             o.LoadersRouterOptions.DynamicParamPrefix,
		SplatSegmentIdentifier:         o.LoadersRouterOptions.SplatSegmentIdentifier,
		ExplicitIndexSegmentIdentifier: explicit_index,
	})

	// actions router
	mount_root := o.ActionsRouterOptions.MountRoot
	if mount_root == "" {
		mount_root = "/api/"
	}
	v.supported_methods = build_supported_methods(
		o.ActionsRouterOptions.SupportedMethods,
	)
	v.supported_methods_allow = build_supported_methods_allow(
		v.supported_methods,
	)
	v.actions_router = mux.NewRouter(&mux.Options{
		DynamicParamPrefix:     o.ActionsRouterOptions.DynamicParamPrefix,
		SplatSegmentIdentifier: o.ActionsRouterOptions.SplatSegmentIdentifier,
		MountRoot:              mount_root,
		ParseInput:             parse_action_input,
	})

	v.runtime_snapshot = prodcache.New(
		wave.IsDev,
		v.__UNCACHED__runtime_snapshot,
	)
	v.root_template = prodcache.New(wave.IsDev, v.__UNCACHED__root_template)

	return v
}

/////////////////////////////////////////////////////////////////////
/////// Initialization
/////////////////////////////////////////////////////////////////////

// MustInit eagerly loads the runtime snapshot and root template,
// registers route patterns on the nested router, and computes the
// server address. Panics on any failure so broken deploys crash at
// startup instead of serving 500s on first request.
func (v *Vorma) MustInit() {
	v.Wave = v.get_wave()
	snapshot, err := v.runtime_snapshot.Get()
	if err != nil {
		panic(fmt.Sprintf("[vorma2]: failed to load runtime snapshot: %v", err))
	}
	if _, err := v.root_template.Get(); err != nil {
		panic(fmt.Sprintf("[vorma2]: failed to load root template: %v", err))
	}
	for pattern := range snapshot.Paths {
		v.loaders_router.AddPatternWithoutHandlerIfMissing(pattern.Str())
	}
	v.patterns_bid.Store(snapshot.ServerBuildID)
	v.server_addr = fmt.Sprintf(":%d", wave.Port())
	v.logger.Info(
		"vorma2 initialized",
		"server_build_id",
		snapshot.ServerBuildID,
	)
}

func (v *Vorma) MustInitWithDefaultRouter() *mux.Router {
	v.MustInit()
	r := mux.NewRouter()
	loaders, actions := v.Loaders(), v.Actions()
	r.AddHTTPHandler("GET", loaders.HandlerMountPattern(), loaders.Handler())
	for m := range actions.SupportedMethods() {
		r.AddHTTPHandler(m, actions.HandlerMountPattern(), actions.Handler())
	}
	return r
}

func (v *Vorma) MustStaticMiddleware() func(http.Handler) http.Handler {
	return v.Wave.MustStaticFileServerMiddleware(true)
}

func (v *Vorma) ServerAddr() string { return v.server_addr }

// maybe_reregister_patterns adds any new route patterns to the
// nested router when the server build ID changes during dev. This
// handles routes added after the initial MustInit registration.
func (v *Vorma) maybe_reregister_patterns(
	snapshot *types.RuntimeSnapshot,
) {
	if !wave.IsDev() {
		return
	}
	last_bid, _ := v.patterns_bid.Load().(string)
	if last_bid == snapshot.ServerBuildID {
		return
	}
	for pattern := range snapshot.Paths {
		v.loaders_router.AddPatternWithoutHandlerIfMissing(pattern.Str())
	}
	v.patterns_bid.Store(snapshot.ServerBuildID)
}

/////////////////////////////////////////////////////////////////////
/////// Build-internal accessors
/////////////////////////////////////////////////////////////////////

// BuildInternals provides build-time access to router and
// codegen state. Not part of the public API contract.
type BuildInternals struct{ v *Vorma }

func (v *Vorma) ForBuild() BuildInternals { return BuildInternals{v: v} }

func (bi BuildInternals) LoadersRouter() *mux.NestedRouter {
	return bi.v.loaders_router
}
func (bi BuildInternals) ActionsRouter() *mux.Router {
	return bi.v.actions_router
}
func (bi BuildInternals) AdHocTypes() []*AdHocType {
	return bi.v.ad_hoc_types
}
func (bi BuildInternals) ExtraTSCode() string {
	return bi.v.extra_ts_code
}

/////////////////////////////////////////////////////////////////////
/////// Loaders / Actions mount helpers
/////////////////////////////////////////////////////////////////////

type Loaders struct{ v *Vorma }
type Actions struct{ v *Vorma }

func (v *Vorma) Loaders() *Loaders { return &Loaders{v: v} }
func (v *Vorma) Actions() *Actions { return &Actions{v: v} }

func (l *Loaders) HandlerMountPattern() string { return "/*" }
func (l *Loaders) Handler() http.Handler       { return l.v.LoadersHandler() }

func (a *Actions) HandlerMountPattern() string {
	return a.v.actions_router.MountRoot("*")
}
func (a *Actions) Handler() http.Handler { return a.v.ActionsHandler() }
func (a *Actions) SupportedMethods() map[string]bool {
	out := make(map[string]bool, len(a.v.supported_methods))
	for k, val := range a.v.supported_methods {
		out[k] = val
	}
	return out
}

/////////////////////////////////////////////////////////////////////
/////// Caches
/////////////////////////////////////////////////////////////////////

func (v *Vorma) __UNCACHED__runtime_snapshot() (*types.RuntimeSnapshot, error) {
	snapshot_filename := constants.RUNTIME_SNAPSHOT_PROD_FILENAME
	if wave.IsDev() {
		snapshot_filename = constants.RUNTIME_SNAPSHOT_DEV_FILENAME
	}

	snapshot_bytes, err := fs.ReadFile(
		v.Wave.StaticRootFS(),
		path.Join(constants.RUNTIME_DIRNAME, snapshot_filename),
	)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", snapshot_filename, err)
	}

	snapshot, err := jsonutil.Parse[types.RuntimeSnapshot](snapshot_bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", snapshot_filename, err)
	}

	return &snapshot, nil
}

func (v *Vorma) __UNCACHED__root_template() (*template.Template, error) {
	snapshot, err := v.runtime_snapshot.Get()
	if err != nil {
		return nil, err
	}
	private_fs, err := v.Wave.PrivateFS()
	if err != nil {
		return nil, err
	}
	return template.ParseFS(private_fs, snapshot.RootTemplatePath.Str())
}

/////////////////////////////////////////////////////////////////////
/////// Registration helpers
/////////////////////////////////////////////////////////////////////

func RegisterLoader[O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	pattern string,
	fn func(CtxPtr) (O, error),
	decorate_ctx func(*LoaderReqData) CtxPtr,
) *Loader[O] {
	if fn == nil {
		panic("[vorma2]: RegisterLoader: fn cannot be nil")
	}
	if decorate_ctx == nil {
		panic("[vorma2]: RegisterLoader: decorate_ctx cannot be nil")
	}
	task := mux.TaskHandlerFromFunc(
		func(req_data *LoaderReqData) (O, error) {
			return fn(decorate_ctx(req_data))
		},
	)
	mux.AddNestedTaskHandler(app.loaders_router, pattern, task)
	return task
}

func RegisterAction[I any, O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	method string,
	pattern string,
	fn func(CtxPtr) (O, error),
	decorate_ctx func(*ActionReqData[I]) CtxPtr,
) *Action[I, O] {
	if fn == nil {
		panic("[vorma2]: RegisterAction: fn cannot be nil")
	}
	if decorate_ctx == nil {
		panic("[vorma2]: RegisterAction: decorate_ctx cannot be nil")
	}
	task := mux.TaskHandlerFromFunc(
		func(req_data *ActionReqData[I]) (O, error) {
			return fn(decorate_ctx(req_data))
		},
	)
	mux.AddTaskHandler(app.actions_router, method, pattern, task)
	return task
}

/////////////////////////////////////////////////////////////////////
/////// Action input parsing
/////////////////////////////////////////////////////////////////////

func build_supported_methods(configured []string) map[string]bool {
	out := make(map[string]bool)
	if len(configured) == 0 {
		for _, m := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
			out[m] = true
		}
		return out
	}
	for _, m := range configured {
		upper := strings.ToUpper(strings.TrimSpace(m))
		if upper != "" {
			out[upper] = true
		}
	}
	return out
}

func build_supported_methods_allow(methods map[string]bool) string {
	sorted := make([]string, 0, len(methods))
	for m := range methods {
		sorted = append(sorted, m)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

func parse_action_input(
	r *http.Request,
	input_ptr any,
) error {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return validate.URLSearchParamsInto(r, input_ptr)
	}
	content_type, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if content_type == "application/x-www-form-urlencoded" ||
		content_type == "multipart/form-data" {
		if _, ok := input_ptr.(*FormData); ok {
			return nil
		}
		return &validate.ValidationError{
			Err: fmt.Errorf("form content type requires FormData input"),
		}
	}
	return validate.JSONBodyInto(r, input_ptr)
}
