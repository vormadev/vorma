package vorma

import (
	"io/fs"
	"path/filepath"

	"github.com/vormadev/vorma/internal/pkg/vormarun"
	"github.com/vormadev/vorma/kit/mux"
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
	Vorma          = vormarun.Vorma
	DevWatchConfig = vormarun.DevWatchConfig
	FrontendConfig = vormarun.FrontendConfig
	TSGenConfig    = vormarun.TSGenConfig
	HTMLConfig     = vormarun.HTMLConfig
	PathConfig     = vormarun.PathConfig
)

/////// RUNTIME TYPES

type (
	Loader[
		O any,
		CtxPtr ~*Ctx,
		Ctx vormarun.LoaderCtxWrapper[CtxPtr],
	] = vormarun.Loader[O, CtxPtr, Ctx]

	Action[
		I any,
		O any,
		CtxPtr ~*Ctx,
		Ctx vormarun.ActionCtxWrapper[I, CtxPtr],
	] = vormarun.Action[I, O, CtxPtr, Ctx]

	LoaderCtx        = vormarun.LoaderCtx
	ActionCtx[I any] = vormarun.ActionCtx[I]

	AnyLoader = vormarun.AnyLoader
	AnyAction = vormarun.AnyAction

	Loaders = []AnyLoader
	Actions = []AnyAction

	LoaderError = vormarun.LoaderError
	FormData    = vormarun.FormData
)

/////// CORE FUNCTIONS

func IsDev() bool { return vormarun.IsDev() }

func InitRouter(
	v *Vorma,
	loaders []AnyLoader,
	actions []AnyAction,
	staticFS fs.FS,
) (*mux.Router, error) {
	return vormarun.InitRouter(v, loaders, actions, staticFS)
}

/////// WRAPPER TYPES / HELPERS

type HeadEls = vormarun.HeadEls

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
