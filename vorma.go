// Package vorma exposes the public application-facing API for building Vorma
// apps.
//
// Runtime internals live under internal/vormaruntime and are surfaced here via
// a stable facade (`Vorma`) plus typed loader/action registration helpers.
//
// Build-time orchestration and code generation are intentionally separated into
// package vormabuild.
package vorma

import (
	_ "embed"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
)

type (
	// HeadEls aliases the canonical head element collection type.
	HeadEls = headels.HeadEls
	// AdHocType aliases the TypeScript ad-hoc type declaration helper.
	AdHocType = tsgen.AdHocType
	// LoaderReqData aliases loader request context data.
	LoaderReqData = vormaruntime.LoaderReqData
	// ActionReqData aliases action request context data.
	ActionReqData[I any] = vormaruntime.ActionReqData[I]
	// None aliases the empty task input marker.
	None = mux.None
	// Action aliases a typed action task handler.
	Action[I any, O any] = mux.TaskHandler[I, O]
	// Loader aliases a typed loader task handler.
	Loader[O any] = mux.TaskHandler[None, O]
	// LoaderFunc is the public loader function signature.
	LoaderFunc[Ctx any, O any] = func(*Ctx) (O, error)
	// ActionFunc is the public action function signature.
	ActionFunc[Ctx any, O any] = func(*Ctx) (O, error)
	// LoadersRouterOptions aliases loader router configuration.
	LoadersRouterOptions = vormaruntime.LoadersRouterOptions
	// ActionsRouterOptions aliases action router configuration.
	ActionsRouterOptions = vormaruntime.ActionsRouterOptions
	// FormData aliases normalized action form-data representation.
	FormData = vormaruntime.FormData
	// LoaderError aliases loader error payload shape.
	LoaderError = vormaruntime.LoaderError
	// DefaultHeadElsFunc defines default head element population.
	DefaultHeadElsFunc = func(r *http.Request, app *Vorma, head *HeadEls) error
	// HeadDedupeKeysFunc defines head-element dedupe key registration.
	HeadDedupeKeysFunc = func(head *HeadEls)
	// RootTemplateDataFunc provides template data per request.
	RootTemplateDataFunc = func(r *http.Request) (map[string]any, error)
	// DiscoveredRegisteredAction describes a discovered registered action route.
	DiscoveredRegisteredAction = struct{ Method, Pattern string }
	// DiscoveredLoaderTaskExecutor executes discovered loader tasks for a
	// request.
	DiscoveredLoaderTaskExecutor = func(r *http.Request) (*nestedmux.TasksResults, bool)
)

// Vorma is the public app facade over internal runtime state.
type Vorma struct {
	*wave.Wave
	runtime *vormaruntime.Vorma
}

// VormaAppConfig configures NewVormaApp.
type VormaAppConfig struct {
	Wave                 *wave.Wave
	DefaultHeadElsFunc   DefaultHeadElsFunc
	HeadDedupeKeysFunc   HeadDedupeKeysFunc
	RootTemplateDataFunc RootTemplateDataFunc
	LoadersRouterOptions LoadersRouterOptions
	ActionsRouterOptions ActionsRouterOptions
	AdHocTypes           []*AdHocType
	ExtraTSCode          string
	Logger               *slog.Logger
}

// VormaBuildIDHeaderKey is the response header containing the active build id.
const VormaBuildIDHeaderKey = vormaruntime.VormaBuildIDHeaderKey

// MustGetPort returns the application runtime port.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func MustGetPort() int { return wave.MustGetPort() }

// GetIsDev reports whether the current process is running in dev mode.
func GetIsDev() bool { return wave.GetIsDev() }

// SetModeToDev marks the current process as development mode.
func SetModeToDev() { wave.SetModeToDev() }

// IsJSONRequest reports whether the request is a Vorma JSON route-data request.
func IsJSONRequest(r *http.Request) bool {
	return vormaruntime.IsJSONRequest(r)
}

// EnableThirdPartyRouter injects task context middleware for external routers.
func EnableThirdPartyRouter(next http.Handler) http.Handler {
	return mux.InjectTasksCtxMiddleware(next)
}

// NewVormaApp constructs a Vorma app facade from config.
func NewVormaApp(o VormaAppConfig) *Vorma {
	var defaultHeadElsFunc vormaruntime.GetDefaultHeadElsFunc
	if o.DefaultHeadElsFunc != nil {
		defaultHeadElsFunc = func(
			r *http.Request,
			runtimeApp *vormaruntime.Vorma,
			head *headels.HeadEls,
		) error {
			return o.DefaultHeadElsFunc(
				r,
				newPublicVormaFromRuntime(runtimeApp),
				head,
			)
		}
	}

	runtimeApp := vormaruntime.NewVormaApp(vormaruntime.VormaAppConfig{
		Wave:                 o.Wave,
		DefaultHeadElsFunc:   defaultHeadElsFunc,
		HeadDedupeKeysFunc:   o.HeadDedupeKeysFunc,
		RootTemplateDataFunc: o.RootTemplateDataFunc,
		LoadersRouterOptions: o.LoadersRouterOptions,
		ActionsRouterOptions: o.ActionsRouterOptions,
		AdHocTypes:           o.AdHocTypes,
		ExtraTSCode:          o.ExtraTSCode,
		Logger:               o.Logger,
	})
	return newPublicVormaFromRuntime(runtimeApp)
}

func newPublicVormaFromRuntime(runtimeApp *vormaruntime.Vorma) *Vorma {
	if runtimeApp == nil {
		return &Vorma{}
	}
	return &Vorma{
		Wave:    runtimeApp.Wave,
		runtime: runtimeApp,
	}
}

func (v *Vorma) requireRuntime(caller string) *vormaruntime.Vorma {
	if v == nil || v.runtime == nil {
		panic(caller + ": app cannot be nil")
	}
	return v.runtime
}

// MustInit initializes runtime route/template artifacts and panics on failure.
func (v *Vorma) MustInit() {
	v.requireRuntime("vorma.Vorma.MustInit").MustInit()
}

// MustInitWithDefaultRouter initializes Vorma and returns a default mux.Router.
func (v *Vorma) MustInitWithDefaultRouter() *mux.Router {
	return v.requireRuntime("vorma.Vorma.MustInitWithDefaultRouter").
		MustInitWithDefaultRouter()
}

// MustStaticMiddleware returns static-file middleware and panics on setup
// errors.
func (v *Vorma) MustStaticMiddleware() func(http.Handler) http.Handler {
	return v.requireRuntime("vorma.Vorma.MustStaticMiddleware").
		MustStaticMiddleware()
}

// ServerAddr returns the bound server address once initialized.
func (v *Vorma) ServerAddr() string {
	return v.requireRuntime("vorma.Vorma.ServerAddr").ServerAddr()
}

// BuildID returns the current runtime build identifier.
func (v *Vorma) BuildID() string {
	return v.requireRuntime("vorma.Vorma.BuildID").BuildID()
}

// UnsafeRuntimeForFrameworkInternals exposes the internal runtime object for
// framework integration code.
func (v *Vorma) UnsafeRuntimeForFrameworkInternals() *vormaruntime.Vorma {
	return v.requireRuntime("vorma.Vorma.UnsafeRuntimeForFrameworkInternals")
}

// RegisterDiscoveredLoaderTask registers a discovered loader handler into the
// runtime loader router.
func RegisterDiscoveredLoaderTask[O any](
	app *Vorma,
	pattern string,
	task *Loader[O],
) {
	runtimeApp := app.requireRuntime("vorma.RegisterDiscoveredLoaderTask")
	nestedmux.AddTaskHandler(
		runtimeApp.LoadersRouter().NestedRouter,
		pattern,
		task,
	)
}

// RegisterDiscoveredActionTask registers a discovered action handler into the
// runtime action router.
func RegisterDiscoveredActionTask[I any, O any](
	app *Vorma,
	method string,
	pattern string,
	task *Action[I, O],
) {
	runtimeApp := app.requireRuntime("vorma.RegisterDiscoveredActionTask")
	mux.AddTaskHandler(runtimeApp.ActionsRouter().Router, method, pattern, task)
}

// HasRegisteredLoaderTask reports whether a loader is registered for pattern.
func (v *Vorma) HasRegisteredLoaderTask(pattern string) bool {
	runtimeApp := v.requireRuntime("vorma.Vorma.HasRegisteredLoaderTask")
	return runtimeApp.LoadersRouter().NestedRouter.HasTaskHandler(pattern)
}

// RegisteredActionRoutes returns all currently registered action routes.
func (v *Vorma) RegisteredActionRoutes() []DiscoveredRegisteredAction {
	runtimeApp := v.requireRuntime("vorma.Vorma.RegisteredActionRoutes")
	routes := runtimeApp.ActionsRouter().AllRoutes()
	if len(routes) == 0 {
		return nil
	}

	out := make([]DiscoveredRegisteredAction, 0, len(routes))
	for _, route := range routes {
		out = append(
			out,
			DiscoveredRegisteredAction{
				Method:  route.Method(),
				Pattern: route.OriginalPattern(),
			},
		)
	}
	return out
}

// FindNestedMatchesAndRunLoaderTasks resolves nested loader matches and runs
// their task graph for the request.
func (v *Vorma) FindNestedMatchesAndRunLoaderTasks(
	r *http.Request,
) (*nestedmux.TasksResults, bool) {
	runtimeApp := v.requireRuntime(
		"vorma.Vorma.FindNestedMatchesAndRunLoaderTasks",
	)
	return nestedmux.FindMatchesAndRunTasks(
		runtimeApp.LoadersRouter().NestedRouter,
		r,
	)
}

// DefineLoaderForRegistration marks a loader declaration for build discovery and generated
// auto-registration. This call returns a task handler but does not directly
// mutate runtime router registration state.
func DefineLoaderForRegistration[O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	p string,
	f func(CtxPtr) (O, error),
	decorateCtx func(*LoaderReqData) CtxPtr,
) *Loader[O] {
	panicIfNilLoaderRegistrationArguments(
		"vorma.DefineLoaderForRegistration",
		f,
		decorateCtx,
	)
	_, _ = app, p
	return newLoaderTask(f, decorateCtx)
}

// DefineActionForRegistration marks an action declaration for build discovery and generated
// auto-registration. This call returns a task handler but does not directly
// mutate runtime router registration state.
func DefineActionForRegistration[I any, O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	m string,
	p string,
	f func(CtxPtr) (O, error),
	decorateCtx func(*ActionReqData[I]) CtxPtr,
) *Action[I, O] {
	panicIfNilActionRegistrationArguments(
		"vorma.DefineActionForRegistration",
		f,
		decorateCtx,
	)
	_, _, _ = app, m, p
	return newActionTask(f, decorateCtx)
}

func newLoaderTask[O any, CtxPtr ~*Ctx, Ctx any](
	loaderFunc func(CtxPtr) (O, error),
	decorateLoaderContext func(*LoaderReqData) CtxPtr,
) *Loader[O] {
	return mux.TaskHandlerFromFunc(
		func(loaderReqData *LoaderReqData) (O, error) {
			return loaderFunc(decorateLoaderContext(loaderReqData))
		},
	)
}

func newActionTask[I any, O any, CtxPtr ~*Ctx, Ctx any](
	actionFunc func(CtxPtr) (O, error),
	decorateActionContext func(*ActionReqData[I]) CtxPtr,
) *Action[I, O] {
	return mux.TaskHandlerFromFunc(
		func(actionReqData *ActionReqData[I]) (O, error) {
			return actionFunc(decorateActionContext(actionReqData))
		},
	)
}

func panicIfNilLoaderRegistrationArguments[O any, CtxPtr ~*Ctx, Ctx any](
	caller string,
	loaderFunc func(CtxPtr) (O, error),
	decorateLoaderContext func(*LoaderReqData) CtxPtr,
) {
	if loaderFunc == nil {
		panic(caller + ": loader function cannot be nil")
	}
	if decorateLoaderContext == nil {
		panic(caller + ": decorateCtx cannot be nil")
	}
}

func panicIfNilActionRegistrationArguments[I any, O any, CtxPtr ~*Ctx, Ctx any](
	caller string,
	actionFunc func(CtxPtr) (O, error),
	decorateActionContext func(*ActionReqData[I]) CtxPtr,
) {
	if actionFunc == nil {
		panic(caller + ": action function cannot be nil")
	}
	if decorateActionContext == nil {
		panic(caller + ": decorateCtx cannot be nil")
	}
}

//go:embed internal/__LAST_RELEASE.txt
var canonicalVersion string

// CurrentReleaseVersion returns the version string embedded at build time.
func CurrentReleaseVersion() string {
	return strings.TrimSpace(canonicalVersion)
}
