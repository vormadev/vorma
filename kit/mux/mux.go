// Package mux provides typed HTTP task routing with pattern matching,
// middleware, and nested route execution.
//
// It is optimized for Vorma/Wave runtime use-cases where handlers are modeled
// as typed tasks instead of untyped `http.HandlerFunc` chains. The package
// supports:
// - request input decoding into typed structs
// - nested pattern routing with params/splats
// - task middleware and HTTP middleware composition
// - deterministic task execution and result aggregation
//
// Package mux is framework-agnostic and can be used directly outside Vorma.
package mux

import (
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/genericsutil"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/internal/muxcore"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/tasks"
	"github.com/vormadev/vorma/kit/validate"
)

var (
	muxLog       = colorlog.New("mux")
	emptyHTTPMws = []httpMiddlewareWithOptions{}
	emptyTaskMws = []taskMiddlewareWithOptions{}
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC API
/////////////////////////////////////////////////////////////////////

// NOTES:
// Order of registration of handlers does not matter. Order of middleware
// registration DOES matter. For traditional middleware, it will run sequentially,
// first to last. For task middleware, they will run with maximum parallelism based
// on the dependency graph of the associated underlying tasks, but their response
// proxies will be merged according to the rules of response.Proxy (which means
// that ordering sometimes matters in tie-breaking scenarios). See response.Proxy
// and tasks.Task documentation and source code for more details.

type (
	// None is the no-input marker for route and middleware generics.
	None = genericsutil.None
	// TaskHandler is a task-backed route handler.
	TaskHandler[I any, O any] = tasks.Task[*ReqData[I], O]
	// Params contains route params extracted by matcher.
	Params = matcher.Params
)

// ReqData is the request-scoped data passed to task handlers and task middleware.
type ReqData[I any] struct {
	params        Params
	splatVals     []string
	tasksCtx      *tasks.Ctx
	input         I
	req           *http.Request
	responseProxy *response.Proxy
}

// MiddlewareOptions configures conditional middleware execution.
type MiddlewareOptions struct {
	// Return true if the middleware should be run for this request.
	// If nil, the middleware will always run.
	If func(r *http.Request) bool
}

type (
	// HTTPMiddleware is standard net/http middleware.
	HTTPMiddleware = func(http.Handler) http.Handler
	// TaskMiddlewareFunc creates task middleware from a function.
	TaskMiddlewareFunc[O any] func(*ReqData[None]) (O, error)
	// TaskMiddleware is task-based middleware run in a tasks.Ctx.
	TaskMiddleware[O any] = tasks.Task[*ReqData[None], O]
	// TaskHandlerFunc creates a task route handler from a function.
	TaskHandlerFunc[I any, O any] func(*ReqData[I]) (O, error)
)

// Router matches routes by method and path and executes HTTP/task middleware layers.
type Router struct {
	parseInput            func(r *http.Request, iPtr any) error
	httpMws               []httpMiddlewareWithOptions
	taskMws               []taskMiddlewareWithOptions
	httpMiddlewareVersion atomic.Uint64
	methodToMatcherMap    map[string]*methodMatcher
	matcherOpts           *matcher.Options
	notFoundHandler       http.Handler
	mountRoot             string
	allRoutes             []AnyRoute
}

// AllRoutes returns a snapshot copy of every registered route.
func (rt *Router) AllRoutes() []AnyRoute {
	allRoutesCopy := make([]AnyRoute, len(rt.allRoutes))
	copy(allRoutesCopy, rt.allRoutes)
	return allRoutesCopy
}

// DynamicParamPrefix returns the matcher's dynamic param prefix rune.
func (rt *Router) DynamicParamPrefix() rune {
	return rt.matcherOpts.DynamicParamPrefix
}

// SplatSegmentIdentifier returns the matcher's splat segment identifier rune.
func (rt *Router) SplatSegmentIdentifier() rune {
	return rt.matcherOpts.SplatSegmentIdentifier
}

// Takes zero or one pattern strings. If no arguments are provided, returns
// the mount root, otherwise returns the mount root joined with the
// provided pattern. For example, if mux.MountRoot() were to return "/api/",
// then mux.MountRoot("foo") would return "/api/foo". Panics if you pass more
// than one argument.
func (rt *Router) MountRoot(optionalPatternToAppend ...string) string {
	if len(optionalPatternToAppend) == 0 {
		return rt.mountRoot
	}
	if len(optionalPatternToAppend) > 1 {
		panic("MountRoot accepts zero or one optional pattern")
	}
	return path.Join(rt.mountRoot, optionalPatternToAppend[0])
}

// TasksCtxRequirer marks HTTP handlers that require tasks context injection.
type TasksCtxRequirer interface {
	http.Handler
	NeedsTasksCtx()
}

var handlerNeedsTasksCtxImplReflectType = reflect.TypeFor[TasksCtxRequirer]()

// TasksCtxRequirerFunc adapts a function to a TasksCtxRequirer handler.
type TasksCtxRequirerFunc func(http.ResponseWriter, *http.Request)

func (h TasksCtxRequirerFunc) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	h(w, r)
}
func (h TasksCtxRequirerFunc) NeedsTasksCtx() {}

// Options configures router matching behavior and task input parsing.
type Options struct {
	// Used for mounting a router at a specific path, e.g., "/api/". If set,
	// the router will strip the provided mount root from the beginning of
	// incoming url paths before matching them against registered patterns.
	MountRoot              string
	DynamicParamPrefix     rune // Optional. Defaults to ':'.
	SplatSegmentIdentifier rune // Optional. Defaults to '*'.
	// Required if using task handlers. Do validation or whatever you want here,
	// and mutate the input ptr to the desired value (this is what will ultimately
	// be returned by c.Input()).
	ParseInput func(r *http.Request, inputPtr any) error
}

// NewRouter creates a router with optional matcher and mount configuration.
func NewRouter(options ...*Options) *Router {
	var opts *Options
	if len(options) > 0 {
		opts = options[0]
	}

	if opts == nil {
		opts = new(Options)
	}
	matcherOpts := muxcore.BuildMatcherOptions(muxcore.MatcherOptionsInput{
		DynamicParamPrefix:     opts.DynamicParamPrefix,
		SplatSegmentIdentifier: opts.SplatSegmentIdentifier,
	})
	mountRootToUse := muxcore.NormalizeMountRoot(opts.MountRoot)
	return &Router{
		parseInput:         opts.ParseInput,
		methodToMatcherMap: make(map[string]*methodMatcher),
		matcherOpts:        matcherOpts,
		mountRoot:          mountRootToUse,
		httpMws:            emptyHTTPMws,
		taskMws:            emptyTaskMws,
	}
}

// TaskHandlers are used for JSON responses only, and they are intended to
// be particularly convenient for sending JSON. If you need to send a different
// content type, use a traditional http.Handler instead.
func TaskHandlerFromFunc[I any, O any](
	taskHandlerFunc TaskHandlerFunc[I, O],
) *TaskHandler[I, O] {
	return tasks.NewTask(func(c *tasks.Ctx, rd *ReqData[I]) (O, error) {
		return taskHandlerFunc(rd)
	})
}

// TaskMiddlewareFromFunc converts a function into task middleware.
func TaskMiddlewareFromFunc[O any](
	userFunc TaskMiddlewareFunc[O],
) *TaskMiddleware[O] {
	return tasks.NewTask(func(c *tasks.Ctx, rd *ReqData[None]) (O, error) {
		return userFunc(rd)
	})
}

// AddGlobalTaskMiddleware registers task middleware for all routes.
func AddGlobalTaskMiddleware[O any](
	router *Router,
	taskMw *TaskMiddleware[O],
	opts ...*MiddlewareOptions,
) {
	router.taskMws = append(router.taskMws, taskMiddlewareWithOptions{
		mw:   taskMw,
		opts: getFirstOpt(opts),
	})
}

// AddGlobalHTTPMiddleware registers HTTP middleware for all routes.
func AddGlobalHTTPMiddleware(
	router *Router,
	httpMw HTTPMiddleware,
	opts ...*MiddlewareOptions,
) {
	router.httpMws = append(router.httpMws, httpMiddlewareWithOptions{
		mw:   httpMw,
		opts: getFirstOpt(opts),
	})
	router.incrementHTTPMiddlewareVersion()
}

// AddGlobalHTTPMiddleware registers HTTP middleware on the receiver router.
func (rt *Router) AddGlobalHTTPMiddleware(
	httpMw HTTPMiddleware,
	opts ...*MiddlewareOptions,
) {
	AddGlobalHTTPMiddleware(rt, httpMw, opts...)
}

// AddMethodLevelTaskMiddleware registers task middleware for one method.
func AddMethodLevelTaskMiddleware[O any](
	router *Router,
	method string,
	taskMw *TaskMiddleware[O],
	opts ...*MiddlewareOptions,
) {
	mm := router.getOrCreateMethodMatcher(method)
	mm.taskMws = append(mm.taskMws, taskMiddlewareWithOptions{
		mw:   taskMw,
		opts: getFirstOpt(opts),
	})
}

// AddMethodLevelHTTPMiddleware registers HTTP middleware for one method.
func AddMethodLevelHTTPMiddleware(
	router *Router,
	method string,
	httpMw HTTPMiddleware,
	opts ...*MiddlewareOptions,
) {
	mm := router.getOrCreateMethodMatcher(method)
	mm.httpMws = append(mm.httpMws, httpMiddlewareWithOptions{
		mw:   httpMw,
		opts: getFirstOpt(opts),
	})
	mm.incrementHTTPMiddlewareVersion()
}

// AddMethodLevelHTTPMiddleware registers HTTP middleware for one method on the receiver router.
func (rt *Router) AddMethodLevelHTTPMiddleware(
	method string,
	httpMw HTTPMiddleware,
	opts ...*MiddlewareOptions,
) {
	AddMethodLevelHTTPMiddleware(rt, method, httpMw, opts...)
}

// AddPatternLevelTaskMiddleware registers task middleware for one route pattern.
func AddPatternLevelTaskMiddleware[PI any, PO any, MWO any](
	route *Route[PI, PO],
	taskMw *TaskMiddleware[MWO],
	opts ...*MiddlewareOptions,
) {
	route.taskMws = append(route.taskMws, taskMiddlewareWithOptions{
		mw:   taskMw,
		opts: getFirstOpt(opts),
	})
}

// AddPatternLevelHTTPMiddleware registers HTTP middleware for one route pattern.
func AddPatternLevelHTTPMiddleware[I any, O any](
	route *Route[I, O],
	httpMw HTTPMiddleware,
	opts ...*MiddlewareOptions,
) {
	route.httpMws = append(route.httpMws, httpMiddlewareWithOptions{
		mw:   httpMw,
		opts: getFirstOpt(opts),
	})
	route.incrementHTTPMiddlewareVersion()
}

// AddPatternLevelHTTPMiddleware registers HTTP middleware on the receiver route.
func (route *Route[I, O]) AddPatternLevelHTTPMiddleware(
	httpMw HTTPMiddleware,
	opts ...*MiddlewareOptions,
) {
	AddPatternLevelHTTPMiddleware(route, httpMw, opts...)
}

// SetGlobalNotFoundHTTPHandler sets the fallback handler used when no route matches.
func SetGlobalNotFoundHTTPHandler(router *Router, httpHandler http.Handler) {
	router.notFoundHandler = httpHandler
}

// SetGlobalNotFoundHTTPHandler sets the receiver router's not-found handler.
func (rt *Router) SetGlobalNotFoundHTTPHandler(httpHandler http.Handler) {
	SetGlobalNotFoundHTTPHandler(rt, httpHandler)
}

// Route stores registration metadata and handlers for a method+pattern pair.
type Route[I, O any] struct {
	genericsutil.ZeroHelper[I, O]
	router                *Router
	method                string
	originalPattern       string
	httpMws               []httpMiddlewareWithOptions
	taskMws               []taskMiddlewareWithOptions
	httpMiddlewareVersion atomic.Uint64
	handlerType           string
	userHTTPHandler       http.Handler
	taskHandler           tasks.AnyTask
	needsTasksCtx         bool
	compiledHTTP          atomic.Value
}

// AnyRoute is the internal polymorphic route contract used by Router.
type AnyRoute interface {
	OriginalPattern() string
	Method() string
	genericsutil.AnyZeroHelper
	getHandlerType() string
	getHTTPHandler() http.Handler
	getTaskHandler() tasks.AnyTask
	getHTTPMws() []httpMiddlewareWithOptions
	getTaskMws() []taskMiddlewareWithOptions
	getNeedsTasksCtx() bool
	httpChain(rt *Router, mm *methodMatcher) http.Handler
}

// OriginalPattern returns the route pattern as registered.
func (route *Route[I, O]) OriginalPattern() string {
	return route.originalPattern
}

// Method returns the registered HTTP method.
func (route *Route[I, O]) Method() string {
	return route.method
}

// TaskHandlers are used for JSON responses only, and they are intended to
// be particularly convenient for sending JSON. If you need to send a different
// content type, use a traditional http.Handler instead.
func AddTaskHandler[I any, O any](
	router *Router, method, pattern string, taskHandler *TaskHandler[I, O],
) *Route[I, O] {
	route := newRouteStruct[I, O](router, method, pattern)
	route.handlerType = "task"
	route.taskHandler = taskHandler
	mm := router.getOrCreateMethodMatcher(method)
	mm.reqDataGetters[pattern] = createReqDataGetter(route)
	router.registerRoute(route)
	return route
}

// AddHTTPHandlerFunc registers an http.HandlerFunc route.
func AddHTTPHandlerFunc(
	router *Router, method, pattern string, httpHandlerFunc http.HandlerFunc,
) *Route[any, any] {
	return AddHTTPHandler(router, method, pattern, httpHandlerFunc)
}

// AddHTTPHandlerFunc registers an http.HandlerFunc route on the receiver router.
func (rt *Router) AddHTTPHandlerFunc(
	method, pattern string,
	httpHandlerFunc http.HandlerFunc,
) *Route[any, any] {
	return AddHTTPHandlerFunc(rt, method, pattern, httpHandlerFunc)
}

// AddHTTPHandler registers an http.Handler route.
func AddHTTPHandler(
	router *Router, method, pattern string, httpHandler http.Handler,
) *Route[any, any] {
	route := newRouteStruct[any, any](router, method, pattern)
	route.handlerType = "http"
	route.userHTTPHandler = httpHandler
	route.needsTasksCtx = reflectutil.DoesTypeImplementInterface(
		reflect.TypeOf(httpHandler), handlerNeedsTasksCtxImplReflectType,
	)
	mm := router.getOrCreateMethodMatcher(method)
	mm.reqDataGetters[pattern] = createReqDataGetter(route)
	router.registerRoute(route)
	return route
}

// AddHTTPHandler registers an http.Handler route on the receiver router.
func (rt *Router) AddHTTPHandler(
	method, pattern string,
	httpHandler http.Handler,
) *Route[any, any] {
	return AddHTTPHandler(rt, method, pattern, httpHandler)
}

// Params returns matched route params.
func (rd *ReqData[I]) Params() Params { return rd.params }

// Param returns a single matched route param.
func (rd *ReqData[I]) Param(key string) string { return rd.params[key] }

// SplatValues returns matched splat segments.
func (rd *ReqData[I]) SplatValues() []string { return rd.splatVals }

// TasksCtx returns the request's task context.
func (rd *ReqData[I]) TasksCtx() *tasks.Ctx { return rd.tasksCtx }

// Request returns the underlying HTTP request.
func (rd *ReqData[I]) Request() *http.Request { return rd.req }

// ResponseProxy returns the response proxy associated with the request.
func (rd *ReqData[I]) ResponseProxy() *response.Proxy { return rd.responseProxy }

// Input returns parsed input for the route handler.
func (rd *ReqData[I]) Input() I { return rd.input }

// SetTasksCtx sets the task context used by the handler execution.
func (rd *ReqData[I]) SetTasksCtx(tasksCtx *tasks.Ctx) {
	rd.tasksCtx = tasksCtx
}

// ResetForReuse resets request data fields for pooled reuse.
func (rd *ReqData[I]) ResetForReuse(
	params Params,
	splatValues []string,
	input I,
	req *http.Request,
	responseProxy *response.Proxy,
) {
	rd.params = params
	rd.splatVals = splatValues
	rd.input = input
	rd.req = req
	rd.responseProxy = responseProxy
	rd.tasksCtx = nil
}

// ClearForPool clears request data fields before returning to a pool.
func (rd *ReqData[I]) ClearForPool() {
	var zeroInput I
	rd.params = nil
	rd.splatVals = nil
	rd.tasksCtx = nil
	rd.input = zeroInput
	rd.req = nil
	rd.responseProxy = nil
}

// GetTasksCtx returns the request's tasks context when present.
func GetTasksCtx(r *http.Request) *tasks.Ctx {
	return muxcore.GetTasksCtx(r)
}

// RequestWithTasksCtx returns a request carrying the provided tasks context.
func RequestWithTasksCtx(
	request *http.Request,
	tasksCtx *tasks.Ctx,
) *http.Request {
	return muxcore.RequestWithTasksCtx(request, tasksCtx)
}

// GetParam returns one route param by key.
func GetParam(r *http.Request, key string) string {
	return muxcore.GetParam(r, key)
}

// GetParams returns all route params for the request.
func GetParams(r *http.Request) Params {
	return muxcore.GetParams(r)
}

// GetSplatValues returns all matched splat values for the request.
func GetSplatValues(r *http.Request) []string {
	return muxcore.GetSplatValues(r)
}

// ServeHTTP matches the request and executes the resolved route pipeline.
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pathToUse := r.URL.Path
	if rt.mountRoot != "" && strings.HasPrefix(pathToUse, rt.mountRoot) {
		pathToUse = "/" + pathToUse[len(rt.mountRoot):]
	}
	best := rt.findBestMatcherAndMatch(r.Method, pathToUse)
	if !best.didMatch {
		if rt.notFoundHandler != nil {
			rt.notFoundHandler.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}
	match := best.match
	mm := best.methodMatcher
	route := mm.routes[match.OriginalPattern()]
	// Fast path for pure HTTP handlers without task middleware
	if route.getHandlerType() == "http" &&
		!rt.hasAnyTaskMiddleware(mm, route) &&
		!route.getNeedsTasksCtx() {
		if len(match.Params) > 0 || len(match.SplatValues) > 0 {
			r = muxcore.RequestWithData(
				r,
				muxcore.NewRequestData(
					match.Params,
					match.SplatValues,
					nil,
					r,
					nil,
				),
			)
		}
		handler := route.httpChain(rt, mm)
		if best.headFellBackToGet {
			treatGetAsHead(handler, w, r)
		} else {
			handler.ServeHTTP(w, r)
		}
		return
	}
	// Slow path: create TasksCtx and full request data
	tasksCtx := tasks.NewCtx(r.Context())
	r = muxcore.RequestWithData(
		r,
		muxcore.NewRequestData(
			match.Params,
			match.SplatValues,
			tasksCtx,
			r,
			response.NewProxy(),
		),
	)
	reqGetter := mm.reqDataGetters[match.OriginalPattern()]
	reqData, err := reqGetter.getReqData(r, tasksCtx, match)
	if err != nil {
		if validate.IsValidationError(err) {
			muxLog.Error(
				"Validation error",
				"error",
				err,
				"pattern",
				match.OriginalPattern(),
			)
			writeErrorResponseWithHeadFallbackSupport(
				best.headFellBackToGet,
				w,
				r,
				http.StatusBadRequest,
				err.Error(),
			)
		} else {
			muxLog.Error("Internal server error", "error", err, "pattern", match.OriginalPattern())
			writeErrorResponseWithHeadFallbackSupport(
				best.headFellBackToGet,
				w,
				r,
				http.StatusInternalServerError,
				"Internal Server Error",
			)
		}
		return
	}
	var finalHandler http.Handler
	if route.getHandlerType() == "http" {
		finalHandler = route.httpChain(rt, mm)
	} else {
		finalHandler = rt.createTaskFinalHandler(route, reqData)
	}
	handlerWithMW := rt.runAppropriateMws(
		tasksCtx,
		reqData,
		mm,
		route,
		finalHandler,
	)
	if best.headFellBackToGet {
		treatGetAsHead(handlerWithMW, w, r)
	} else {
		handlerWithMW.ServeHTTP(w, r)
	}
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE API
/////////////////////////////////////////////////////////////////////

func applyHTTPMiddlewareWithOptions(
	mwWithOpts httpMiddlewareWithOptions,
	handler http.Handler,
) http.Handler {
	if mwWithOpts.opts != nil && mwWithOpts.opts.If != nil {
		originalHandler := handler
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !mwWithOpts.opts.If(r) {
				originalHandler.ServeHTTP(w, r)
			} else {
				mwWithOpts.mw(originalHandler).ServeHTTP(w, r)
			}
		})
	}
	return mwWithOpts.mw(handler)
}

func applyHTTPMiddlewares(
	handler http.Handler,
	routeMws []httpMiddlewareWithOptions,
	methodMws []httpMiddlewareWithOptions,
	globalMws []httpMiddlewareWithOptions,
) http.Handler { // Apply in reverse order for proper nesting
	for i := len(routeMws) - 1; i >= 0; i-- { // Pattern-level middlewares (innermost)
		handler = applyHTTPMiddlewareWithOptions(routeMws[i], handler)
	}
	for i := len(methodMws) - 1; i >= 0; i-- { // Method-level middlewares
		handler = applyHTTPMiddlewareWithOptions(methodMws[i], handler)
	}
	for i := len(globalMws) - 1; i >= 0; i-- { // Global middlewares (outermost)
		handler = applyHTTPMiddlewareWithOptions(globalMws[i], handler)
	}
	return handler
}

type middlewareBoundTask struct {
	taskToRun tasks.AnyTask
	input     *ReqData[None]
}

func (m *middlewareBoundTask) Run(ctx *tasks.Ctx) error {
	_, err := m.taskToRun.RunWithAnyInput(ctx, m.input)
	return err
}

func (rt *Router) gatherAllTaskMiddlewares(
	methodMatcher *methodMatcher, routeMarker AnyRoute,
) []taskMiddlewareWithOptions {
	taskMwsRoute := routeMarker.getTaskMws()
	if len(rt.taskMws) == 0 && len(methodMatcher.taskMws) == 0 &&
		len(taskMwsRoute) == 0 {
		return nil
	}
	cap := len(taskMwsRoute) + len(methodMatcher.taskMws) + len(rt.taskMws)
	allTaskMws := make([]taskMiddlewareWithOptions, 0, cap)
	allTaskMws = append(allTaskMws, rt.taskMws...)
	allTaskMws = append(allTaskMws, methodMatcher.taskMws...)
	allTaskMws = append(allTaskMws, taskMwsRoute...)
	return allTaskMws
}

func (rt *Router) createTaskFinalHandler(
	route AnyRoute,
	reqDataMarker reqDataMarker,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)
		taskHandler := route.getTaskHandler()
		inputData := reqDataMarker.getUnderlyingReqDataInstance()
		data, err := taskHandler.RunWithAnyInput(
			reqDataMarker.TasksCtx(),
			inputData,
		)
		if err != nil {
			muxLog.Error(
				"Error executing task handler",
				"error",
				err,
				"pattern",
				route.OriginalPattern(),
			)
			res.InternalServerError()
			return
		}
		responseProxy := reqDataMarker.ResponseProxy()
		responseProxy.ApplyToResponseWriter(w, r)
		if responseProxy.IsError() || responseProxy.IsRedirect() {
			return // Don't write JSON after error/redirect
		}
		if reflectutil.ExcludingNoneGetIsNilOrUltimatelyPointsToNil(data) {
			muxLog.Warn(
				"Do not return nil values from task handlers unless: (i) the underlying type is an empty struct or pointer to an empty struct; or (ii) you are returning an error.",
				"pattern",
				route.OriginalPattern(),
			)
		}
		res.JSON(data)
	})
}

func (rt *Router) runAppropriateMws(
	tasksCtx *tasks.Ctx,
	reqDataMarker reqDataMarker,
	methodMatcher *methodMatcher,
	routeMarker AnyRoute,
	finalHandler http.Handler,
) http.Handler {
	var handlerWithHTTPMws http.Handler
	if routeMarker.getHandlerType() == "http" {
		handlerWithHTTPMws = finalHandler
	} else {
		handlerWithHTTPMws = applyHTTPMiddlewares(finalHandler, routeMarker.getHTTPMws(), methodMatcher.httpMws, rt.httpMws)
	}
	collected := rt.gatherAllTaskMiddlewares(methodMatcher, routeMarker)
	if len(collected) == 0 {
		return handlerWithHTTPMws
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		boundTasks := make([]tasks.BoundTask, 0, len(collected))
		reqDataInstances := make([]*ReqData[None], 0, len(collected))
		for _, taskWithOpts := range collected {
			if taskWithOpts.opts != nil && taskWithOpts.opts.If != nil &&
				!taskWithOpts.opts.If(r) {
				continue
			}
			rdForMw := &ReqData[None]{
				params:        reqDataMarker.Params(),
				splatVals:     reqDataMarker.SplatValues(),
				tasksCtx:      tasksCtx,
				input:         None{},
				req:           r,
				responseProxy: response.NewProxy(),
			}
			reqDataInstances = append(reqDataInstances, rdForMw)
			boundTasks = append(boundTasks, &middlewareBoundTask{
				taskToRun: taskWithOpts.mw,
				input:     rdForMw,
			})
		}
		if err := tasksCtx.RunParallel(boundTasks...); err != nil {
			muxLog.Error(
				"Error during parallel middleware execution",
				"error",
				err,
			)
			http.Error(
				w,
				"Internal Server Error",
				http.StatusInternalServerError,
			)
			return
		}
		proxies := make([]*response.Proxy, len(reqDataInstances))
		for i, rdInst := range reqDataInstances {
			proxies[i] = rdInst.ResponseProxy()
		}
		merged := response.MergeProxyResponses(proxies...)
		merged.ApplyToResponseWriter(w, r)
		if merged.IsError() || merged.IsRedirect() {
			return
		}
		handlerWithHTTPMws.ServeHTTP(w, r)
	})
}

func newRouteStruct[I any, O any](
	router *Router,
	method, originalPattern string,
) *Route[I, O] {
	return &Route[I, O]{
		router: router, method: method, originalPattern: originalPattern,
		httpMws: emptyHTTPMws, taskMws: emptyTaskMws,
	}
}

func (rt *Router) registerRoute(route AnyRoute) {
	methodMatcher := rt.getOrCreateMethodMatcher(route.Method())
	methodMatcher.matcher.RegisterPattern(route.OriginalPattern())
	methodMatcher.routes[route.OriginalPattern()] = route
	rt.allRoutes = append(rt.allRoutes, route)
}

func createReqDataGetter[I any, O any](route *Route[I, O]) reqDataGetter {
	return reqDataGetterImpl[I](
		func(r *http.Request, tasksCtx *tasks.Ctx, match *matcher.BestMatch) (*ReqData[I], error) {
			reqData := new(ReqData[I])
			reqData.params = match.Params
			reqData.splatVals = match.SplatValues
			reqData.tasksCtx = tasksCtx
			reqData.req = r
			reqData.responseProxy = response.NewProxy()
			inputPtr := route.IPtr()
			if route.handlerType == "task" && route.router.parseInput != nil &&
				!genericsutil.IsNone(route.I()) {
				if err := route.router.parseInput(reqData.Request(), inputPtr); err != nil {
					return nil, err
				}
			}
			reqData.input = *(inputPtr.(*I))
			return reqData, nil
		},
	)
}

func (rt *Router) getOrCreateMethodMatcher(method string) *methodMatcher {
	if mm, ok := rt.methodToMatcherMap[method]; ok {
		return mm
	}
	mm := &methodMatcher{
		matcher:        matcher.New(rt.matcherOpts),
		routes:         make(map[string]AnyRoute),
		reqDataGetters: make(map[string]reqDataGetter),
		httpMws:        emptyHTTPMws,
		taskMws:        emptyTaskMws,
	}
	rt.methodToMatcherMap[method] = mm
	return mm
}

type findBestOutput struct {
	methodMatcher     *methodMatcher
	match             *matcher.BestMatch
	didMatch          bool
	headFellBackToGet bool
}

func (rt *Router) findBestMatcherAndMatch(
	method string,
	realPath string,
) *findBestOutput {
	isHead := method == http.MethodHead
	if isHead {
		if headMatcher, ok := rt.methodToMatcherMap[http.MethodHead]; ok {
			if match, found := headMatcher.matcher.FindBestMatch(realPath); found {
				return &findBestOutput{
					methodMatcher: headMatcher,
					match:         match,
					didMatch:      true,
				}
			}
		}
		method = http.MethodGet
	}
	methodMatcher, ok := rt.methodToMatcherMap[method]
	if !ok {
		return &findBestOutput{}
	}
	match, ok := methodMatcher.matcher.FindBestMatch(realPath)
	if !ok {
		return &findBestOutput{}
	}
	return &findBestOutput{
		methodMatcher:     methodMatcher,
		match:             match,
		didMatch:          true,
		headFellBackToGet: isHead,
	}
}

func (rt *Router) hasAnyTaskMiddleware(
	methodMatcher *methodMatcher,
	route AnyRoute,
) bool {
	return len(route.getTaskMws()) > 0 ||
		len(methodMatcher.taskMws) > 0 ||
		len(rt.taskMws) > 0
}

type httpMiddlewareWithOptions struct {
	mw   HTTPMiddleware
	opts *MiddlewareOptions
}

type taskMiddlewareWithOptions struct {
	mw   tasks.AnyTask
	opts *MiddlewareOptions
}

type methodMatcher struct {
	matcher               *matcher.Matcher
	httpMws               []httpMiddlewareWithOptions
	taskMws               []taskMiddlewareWithOptions
	httpMiddlewareVersion atomic.Uint64
	routes                map[string]AnyRoute
	reqDataGetters        map[string]reqDataGetter
}

func getFirstOpt(opts []*MiddlewareOptions) *MiddlewareOptions {
	if len(opts) > 0 {
		return opts[0]
	}
	return nil
}

func (rt *Router) incrementHTTPMiddlewareVersion() {
	if rt == nil {
		return
	}
	rt.httpMiddlewareVersion.Add(1)
}

func (rt *Router) currentHTTPMiddlewareVersion() uint64 {
	if rt == nil {
		return 0
	}
	return rt.httpMiddlewareVersion.Load()
}

func (mm *methodMatcher) incrementHTTPMiddlewareVersion() {
	if mm == nil {
		return
	}
	mm.httpMiddlewareVersion.Add(1)
}

func (mm *methodMatcher) currentHTTPMiddlewareVersion() uint64 {
	if mm == nil {
		return 0
	}
	return mm.httpMiddlewareVersion.Load()
}

func (route *Route[I, O]) incrementHTTPMiddlewareVersion() {
	if route == nil {
		return
	}
	route.httpMiddlewareVersion.Add(1)
}

func (route *Route[I, O]) currentHTTPMiddlewareVersion() uint64 {
	if route == nil {
		return 0
	}
	return route.httpMiddlewareVersion.Load()
}

func (route *Route[I, O]) getHandlerType() string { return route.handlerType }

func (route *Route[I, O]) getHTTPHandler() http.Handler { return route.userHTTPHandler }

func (route *Route[I, O]) getTaskHandler() tasks.AnyTask { return route.taskHandler }

func (route *Route[I, O]) getHTTPMws() []httpMiddlewareWithOptions { return route.httpMws }

func (route *Route[I, O]) getTaskMws() []taskMiddlewareWithOptions { return route.taskMws }

func (route *Route[I, O]) getNeedsTasksCtx() bool { return route.needsTasksCtx }

type compiledHTTPCacheEntry struct {
	handler                      http.Handler
	globalHTTPMiddlewareVersion  uint64
	methodHTTPMiddlewareVersion  uint64
	patternHTTPMiddlewareVersion uint64
}

func (r *Route[I, O]) httpChain(rt *Router, mm *methodMatcher) http.Handler {
	globalHTTPMiddlewareVersion := rt.currentHTTPMiddlewareVersion()
	methodHTTPMiddlewareVersion := mm.currentHTTPMiddlewareVersion()
	patternHTTPMiddlewareVersion := r.currentHTTPMiddlewareVersion()

	cachedValue, hasCachedValue := r.compiledHTTP.Load().(*compiledHTTPCacheEntry)
	if hasCachedValue &&
		cachedValue != nil &&
		cachedValue.globalHTTPMiddlewareVersion == globalHTTPMiddlewareVersion &&
		cachedValue.methodHTTPMiddlewareVersion == methodHTTPMiddlewareVersion &&
		cachedValue.patternHTTPMiddlewareVersion == patternHTTPMiddlewareVersion {
		return cachedValue.handler
	}
	compiledHandler := applyHTTPMiddlewares(
		r.getHTTPHandler(),
		r.httpMws,
		mm.httpMws,
		rt.httpMws,
	)
	r.compiledHTTP.Store(&compiledHTTPCacheEntry{
		handler:                      compiledHandler,
		globalHTTPMiddlewareVersion:  globalHTTPMiddlewareVersion,
		methodHTTPMiddlewareVersion:  methodHTTPMiddlewareVersion,
		patternHTTPMiddlewareVersion: patternHTTPMiddlewareVersion,
	})
	return compiledHandler
}

type reqDataMarker interface {
	getInput() any
	getUnderlyingReqDataInstance() any
	Params() Params
	SplatValues() []string
	TasksCtx() *tasks.Ctx
	Request() *http.Request
	ResponseProxy() *response.Proxy
}

func (rd *ReqData[I]) getInput() any                     { return rd.input }
func (rd *ReqData[I]) getUnderlyingReqDataInstance() any { return rd }

type reqDataGetter interface {
	getReqData(
		r *http.Request, tasksCtx *tasks.Ctx, match *matcher.BestMatch,
	) (reqDataMarker, error)
}

type reqDataGetterImpl[I any] func(
	*http.Request, *tasks.Ctx, *matcher.BestMatch,
) (*ReqData[I], error)

func (f reqDataGetterImpl[I]) getReqData(
	r *http.Request, tasksCtx *tasks.Ctx, m *matcher.BestMatch,
) (reqDataMarker, error) {
	return f(r, tasksCtx, m)
}

func treatGetAsHead(
	handler http.Handler,
	w http.ResponseWriter,
	r *http.Request,
) {
	headRecorder := httptest.NewRecorder()
	handler.ServeHTTP(headRecorder, r)

	for k, values := range headRecorder.Header() {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}

	if w.Header().Get("Content-Length") == "" {
		w.Header().Set("Content-Length", strconv.Itoa(headRecorder.Body.Len()))
	}
	w.WriteHeader(headRecorder.Code)
}

func writeErrorResponseWithHeadFallbackSupport(
	headFellBackToGet bool,
	w http.ResponseWriter,
	r *http.Request,
	statusCode int,
	errorText string,
) {
	errorHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, errorText, statusCode)
		},
	)
	if headFellBackToGet {
		treatGetAsHead(errorHandler, w, r)
		return
	}
	errorHandler.ServeHTTP(w, r)
}

// InjectTasksCtxMiddleware ensures requests have a tasks context in request store.
// This is primarily useful when nested task execution is used outside Router.ServeHTTP.
func InjectTasksCtxMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetTasksCtx(r) != nil {
			next.ServeHTTP(w, r)
			return
		}

		tasksCtx := tasks.NewCtx(r.Context())
		next.ServeHTTP(
			w,
			muxcore.RequestWithData(
				r,
				muxcore.NewRequestDataWithTasksCtxOnly(r, tasksCtx),
			),
		)
	})
}

/////////////////////////////////////////////////////////////////////
/////// REQ DATA HELPERS TO REDUCE DIRECT RESPONSE PROXY USAGE
/////////////////////////////////////////////////////////////////////

// Head creates a new HeadEls instance and registers it with the response proxy.
func (rd *ReqData[I]) HeadEls() *headels.HeadEls {
	e := headels.New()
	rd.responseProxy.AddHeadEls(e)
	return e
}

// Redirect sets a redirect on the response proxy. Defaults to 302 if no code
// is provided. It returns the same result contract as response.Proxy.Redirect.
func (rd *ReqData[I]) Redirect(url string, code ...int) (bool, error) {
	return rd.responseProxy.Redirect(rd.req, url, code...)
}

// SetResponseStatus sets the status code and optional error text on the response proxy.
func (rd *ReqData[I]) SetResponseStatus(status int, errorText ...string) {
	rd.responseProxy.SetStatus(status, errorText...)
}

// SetResponseCookie adds a cookie to the response proxy.
func (rd *ReqData[I]) SetResponseCookie(cookie *http.Cookie) {
	rd.responseProxy.SetCookie(cookie)
}

// SetResponseHeader sets a header on the response proxy, replacing any existing values for that key.
func (rd *ReqData[I]) SetResponseHeader(key, value string) {
	rd.responseProxy.SetHeader(key, value)
}

// AddResponseHeader appends a header value to the response proxy without replacing existing values.
func (rd *ReqData[I]) AddResponseHeader(key, value string) {
	rd.responseProxy.AddHeader(key, value)
}

// ResponseStatus returns the status code and error text from the response proxy.
func (rd *ReqData[I]) ResponseStatus() (int, string) {
	return rd.responseProxy.Status()
}

// ResponseHeader returns the first value for a header on the response proxy, or empty string if not set.
func (rd *ReqData[I]) ResponseHeader(key string) string {
	return rd.responseProxy.Header(key)
}

// ResponseHeaders returns all values for a header on the response proxy.
func (rd *ReqData[I]) ResponseHeaders(key string) []string {
	return rd.responseProxy.Headers(key)
}

// ResponseCookies returns all cookies set on the response proxy.
func (rd *ReqData[I]) ResponseCookies() []*http.Cookie {
	return rd.responseProxy.Cookies()
}

// ResponseLocation returns the redirect URL from the response proxy, if a redirect has been set.
func (rd *ReqData[I]) ResponseLocation() string {
	return rd.responseProxy.Location()
}

// IsResponseError returns true if the response proxy status is 400 or higher.
func (rd *ReqData[I]) IsResponseError() bool {
	return rd.responseProxy.IsError()
}

// IsResponseRedirect returns true if a redirect has been set on the response proxy.
func (rd *ReqData[I]) IsResponseRedirect() bool {
	return rd.responseProxy.IsRedirect()
}

// IsResponseSuccess returns true if the response proxy status is in the 2xx range.
func (rd *ReqData[I]) IsResponseSuccess() bool {
	return rd.responseProxy.IsSuccess()
}
