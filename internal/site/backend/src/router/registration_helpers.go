package router

import (
	"github.com/vormadev/vorma"
)

type LoaderCtx struct{ *vorma.LoaderReqData }
type ActionCtx[I any] struct{ *vorma.ActionReqData[I] }

func decorateLoaderCtx(rd *vorma.LoaderReqData) *LoaderCtx {
	return &LoaderCtx{LoaderReqData: rd}
}
func decorateActionCtx[I any](rd *vorma.ActionReqData[I]) *ActionCtx[I] {
	return &ActionCtx[I]{ActionReqData: rd}
}

func NewLoader[O any](
	pattern string, loader vorma.LoaderFunc[LoaderCtx, O],
) *vorma.Loader[O] {
	return vorma.NewLoader(App, pattern, loader, decorateLoaderCtx)
}
func NewAction[I any, O any](
	method string, pattern string, action vorma.ActionFunc[ActionCtx[I], I, O],
) *vorma.Action[I, O] {
	return vorma.NewAction(App, method, pattern, action, decorateActionCtx)
}
