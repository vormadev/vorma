package vorma

import (
	"net/http"
	"path/filepath"

	"github.com/vormadev/vorma/internal/pkg/vormarun"
	"github.com/vormadev/vorma/kit/tsgen"
)

/////// CONSTANTS

const (
	ClientBuildIDHeaderKey       = vormarun.X_Vorma_Client_Build_Id
	PublicStaticOutNamePrefix    = vormarun.Public_Static_Out_Name_Prefix
	PublicStaticPrehashedDirname = vormarun.Prehashed_Dirname
)

/////// CONFIG TYPES

type (
	Instance       = vormarun.Instance
	Router         = vormarun.Router
	Config         = vormarun.Config
	DistConfig     = vormarun.DistConfig
	DevWatchConfig = vormarun.DevWatchConfig
	FrontendConfig = vormarun.FrontendConfig
	TSGenConfig    = vormarun.TSGenConfig
	HTMLConfig     = vormarun.HTMLConfig
	PathConfig     = vormarun.PathConfig
)

/////// RUNTIME TYPES

type (
	RequestCtxWrapper[I, RP any] = vormarun.RequestCtxWrapper[I, RP]

	View[I, O any, RP ~*R, R RequestCtxWrapper[I, RP]]     = vormarun.View[I, O, RP, R]
	APIRoute[I, O any, RP ~*R, R RequestCtxWrapper[I, RP]] = vormarun.APIRoute[I, O, RP, R]

	RequestCtx[I any] = vormarun.RequestCtx[I]

	AnyView     = vormarun.AnyView
	AnyAPIRoute = vormarun.AnyAPIRoute

	Views     = vormarun.Views
	APIRoutes = vormarun.APIRoutes

	LoaderError = vormarun.LoaderError
	FormData    = vormarun.FormData

	APIRouteKind = vormarun.APIRouteKind
)

const (
	APIRouteKindQuery    = vormarun.APIRouteKindQuery
	APIRouteKindMutation = vormarun.APIRouteKindMutation
)

/////// CORE FUNCTIONS

func New(config *Config) *Instance {
	instance, err := vormarun.New(config)
	if err != nil {
		panic(err)
	}
	return instance
}

func IsDev() bool { return vormarun.IsDev() }

func IsJSONRequest(r *http.Request) bool {
	return vormarun.IsJSONRequest(r)
}

/////// WRAPPER TYPES / HELPERS

type HeadBuilder = vormarun.HeadBuilder

type GoTypeSrc = vormarun.GoTypeSrc

func GoType[T any](requestedName ...string) *GoTypeSrc {
	return tsgen.GoType[T](requestedName...)
}

type TSDrafter = vormarun.TSDrafter

/////// PURE CONVENIENCE HELPERS

func MakeRootFunc(cwdBase string) func(subPath string) string {
	return func(p string) string {
		return filepath.Join(filepath.FromSlash(cwdBase), p)
	}
}
