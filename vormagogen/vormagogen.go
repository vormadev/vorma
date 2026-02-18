package vormagogen

import (
	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/mux"
)

func RegisterLoaderDiscoveredByBuild[O any, CtxPtr ~*Ctx, Ctx any](
	app *vorma.Vorma,
	pattern string,
	loaderFunc func(CtxPtr) (O, error),
	decorateLoaderContext func(*vorma.LoaderReqData) CtxPtr,
) *vorma.Loader[O] {
	panicIfNilDiscoveredRegistrationApp(
		"vormagogen.RegisterLoaderDiscoveredByBuild",
		app,
	)
	panicIfNilLoaderRegistrationArguments(
		"vormagogen.RegisterLoaderDiscoveredByBuild",
		loaderFunc,
		decorateLoaderContext,
	)
	loaderTask := vorma.DefineLoaderForRegistration(
		app,
		pattern,
		loaderFunc,
		decorateLoaderContext,
	)
	mux.AddNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		pattern,
		loaderTask,
	)
	return loaderTask
}

func RegisterActionDiscoveredByBuild[I any, O any, CtxPtr ~*Ctx, Ctx any](
	app *vorma.Vorma,
	method string,
	pattern string,
	actionFunc func(CtxPtr) (O, error),
	decorateActionContext func(*vorma.ActionReqData[I]) CtxPtr,
) *vorma.Action[I, O] {
	panicIfNilDiscoveredRegistrationApp(
		"vormagogen.RegisterActionDiscoveredByBuild",
		app,
	)
	panicIfNilActionRegistrationArguments(
		"vormagogen.RegisterActionDiscoveredByBuild",
		actionFunc,
		decorateActionContext,
	)
	actionTask := vorma.DefineActionForRegistration(
		app,
		method,
		pattern,
		actionFunc,
		decorateActionContext,
	)
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		method,
		pattern,
		actionTask,
	)
	return actionTask
}

func panicIfNilDiscoveredRegistrationApp(caller string, app *vorma.Vorma) {
	if app == nil {
		panic(caller + ": app cannot be nil")
	}
}

func panicIfNilLoaderRegistrationArguments[O any, CtxPtr ~*Ctx, Ctx any](
	caller string,
	loaderFunc func(CtxPtr) (O, error),
	decorateLoaderContext func(*vorma.LoaderReqData) CtxPtr,
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
	decorateActionContext func(*vorma.ActionReqData[I]) CtxPtr,
) {
	if actionFunc == nil {
		panic(caller + ": action function cannot be nil")
	}
	if decorateActionContext == nil {
		panic(caller + ": decorateCtx cannot be nil")
	}
}
