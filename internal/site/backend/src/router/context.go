package router

import "github.com/vormadev/vorma"

type LoaderCtx struct{ *vorma.LoaderReqData }
type ActionCtx[I any] struct{ *vorma.ActionReqData[I] }

func decorateLoaderCtx(rd *vorma.LoaderReqData) *LoaderCtx {
	return &LoaderCtx{LoaderReqData: rd}
}

func decorateActionCtx[I any](rd *vorma.ActionReqData[I]) *ActionCtx[I] {
	return &ActionCtx[I]{ActionReqData: rd}
}

func DefineLoader[O any](
	pattern string,
	loader vorma.LoaderFunc[LoaderCtx, O],
) *vorma.Loader[O] {
	return vorma.DefineLoaderForRegistration(
		App,
		pattern,
		loader,
		decorateLoaderCtx,
	)
}

func DefineAction[I any, O any](
	method string,
	pattern string,
	action vorma.ActionFunc[ActionCtx[I], O],
) *vorma.Action[I, O] {
	return vorma.DefineActionForRegistration(
		App,
		method,
		pattern,
		action,
		decorateActionCtx,
	)
}
