package vorma

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/vormadev/vorma/vorma2"
)

const BuildIDHeaderKey = vorma2.BuildIDHeaderKey

type (
	HeadEls                           = vorma2.HeadEls
	None                              = vorma2.None
	Action[I any, O any]              = vorma2.Action[I, O]
	Loader[O any]                     = vorma2.Loader[O]
	LoaderReqData                     = vorma2.LoaderReqData
	ActionReqData[I any]              = vorma2.ActionReqData[I]
	AdHocType                         = vorma2.AdHocType
	FormData                          = vorma2.FormData
	DefaultHeadElsFunc                = vorma2.DefaultHeadElsFunc
	HeadDedupeKeysFunc                = vorma2.HeadDedupeKeysFunc
	RootTemplateDataFunc              = vorma2.RootTemplateDataFunc
	LoadersRouterOptions              = vorma2.LoadersRouterOptions
	ActionsRouterOptions              = vorma2.ActionsRouterOptions
	Vorma                             = vorma2.Vorma
	VormaAppConfig                    = vorma2.VormaAppConfig
	Loaders                           = vorma2.Loaders
	Actions                           = vorma2.Actions
	LoaderFunc[Ctx any, O any]        = vorma2.LoaderFunc[Ctx, O]
	ActionFunc[Ctx any, I any, O any] = vorma2.ActionFunc[Ctx, I, O]
)

func IsJSONRequest(r *http.Request) bool {
	return vorma2.IsJSONRequest(r)
}

func EnableThirdPartyRouter(next http.Handler) http.Handler {
	return vorma2.EnableThirdPartyRouter(next)
}

func NewVormaApp(o VormaAppConfig) *Vorma {
	return vorma2.NewVormaApp(o)
}

func RegisterLoader[O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	pattern string,
	fn func(CtxPtr) (O, error),
	decorate_ctx func(*LoaderReqData) CtxPtr,
) *Loader[O] {
	return vorma2.RegisterLoader(app, pattern, fn, decorate_ctx)
}

func RegisterAction[I any, O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	method string,
	pattern string,
	fn func(CtxPtr) (O, error),
	decorate_ctx func(*ActionReqData[I]) CtxPtr,
) *Action[I, O] {
	return vorma2.RegisterAction(app, method, pattern, fn, decorate_ctx)
}

//go:embed internal/__LAST_RELEASE.txt
var canonicalVersion string

// CurrentReleaseVersion returns the version string embedded at build time.
func CurrentReleaseVersion() string {
	return strings.TrimSpace(canonicalVersion)
}
