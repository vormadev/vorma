package vorma

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
)

// Type aliases for public API
type (
	Vorma                             = vormaruntime.Vorma
	HeadEls                           = headels.HeadEls
	AdHocType                         = tsgen.AdHocType
	VormaAppConfig                    = vormaruntime.VormaAppConfig
	LoadersRouter                     = vormaruntime.LoadersRouter
	LoaderReqData                     = vormaruntime.LoaderReqData
	ActionsRouter                     = vormaruntime.ActionsRouter
	ActionReqData[I any]              = vormaruntime.ActionReqData[I]
	None                              = mux.None
	Action[I any, O any]              = mux.TaskHandler[I, O]
	Loader[O any]                     = mux.TaskHandler[None, O]
	LoaderFunc[Ctx any, O any]        = func(*Ctx) (O, error)
	ActionFunc[Ctx any, I any, O any] = func(*Ctx) (O, error)
	LoadersRouterOptions              = vormaruntime.LoadersRouterOptions
	ActionsRouterOptions              = vormaruntime.ActionsRouterOptions
	FormData                          = vormaruntime.FormData
	LoaderError                       = vormaruntime.LoaderError
)

const VormaBuildIDHeaderKey = vormaruntime.VormaBuildIDHeaderKey

// MustGetPort returns the application runtime port.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func MustGetPort() int { return wave.MustGetPort() }

func GetIsDev() bool { return wave.GetIsDev() }
func SetModeToDev()  { wave.SetModeToDev() }

func IsJSONRequest(r *http.Request) bool {
	return vormaruntime.IsJSONRequest(r)
}
func EnableThirdPartyRouter(next http.Handler) http.Handler {
	return mux.InjectTasksCtxMiddleware(next)
}

func NewVormaApp(o VormaAppConfig) *Vorma {
	return vormaruntime.NewVormaApp(o)
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

func CurrentReleaseVersion() string {
	return strings.TrimSpace(canonicalVersion)
}
