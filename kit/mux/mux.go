// Package mux provides typed HTTP routing with pattern matching, middleware
// composition, and nested route execution.
//
// It supports two routing models:
//   - Router: resolves one best-match route per request (traditional routing).
//   - NestedRouter: resolves hierarchical ancestor matches (loader/layout routing).
//
// Handlers may be standard http.Handlers or typed task handlers that return
// JSON-serializable data through a tasks.Cache execution context.
package mux

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/contextutil"
	"github.com/vormadev/vorma/kit/genericsutil"
	"github.com/vormadev/vorma/kit/head"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/schema"
	"github.com/vormadev/vorma/kit/tasks"
)

// __TODO add tests for MatchedPattern getters

var (
	mux_log       = colorlog.New("mux")
	nested_log    = colorlog.New("nestedmux")
	request_store = contextutil.NewStore[*req_ctx_transport](
		"__vorma_kit_mux_req_ctx_transport",
	)
	empty_splat    = []string{}
	empty_http_mws = []http_mw_with_opts{}
	empty_task_mws = []task_mw_with_opts{}

	needs_tasks_cache_type = reflect.TypeFor[TasksCacheRequirer]()
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC BASE TYPES
/////////////////////////////////////////////////////////////////////

type (
	None                      = genericsutil.None
	TaskHandler[I, O any]     = tasks.Task[*RequestCtx[I], O]
	Params                    = matcher.Params
	Middleware                = func(http.Handler) http.Handler
	TaskMiddlewareFunc[O any] func(*RequestCtx[None]) (O, error)
	TaskMiddleware[O any]     = tasks.Task[*RequestCtx[None], O]
	TaskHandlerFunc[I, O any] func(*RequestCtx[I]) (O, error)
)

// RequestCtx is request-scoped context passed to task handlers and middleware.
type RequestCtx[I any] struct {
	matched_pattern string
	params          Params
	splat_vals      []string
	tasks_cache     *tasks.Cache
	input           I
	req             *http.Request
	response_proxy  *response.Proxy
}

// MiddlewareOptions configures conditional middleware execution.
type MiddlewareOptions struct {
	If func(r *http.Request) bool
}

/////////////////////////////////////////////////////////////////////
/////// ROUTER
/////////////////////////////////////////////////////////////////////

// Options configures Router matching and input parsing.
type Options struct {
	MountRoot              string
	DynamicParamPrefix     rune
	SplatSegmentIdentifier rune
	ParseInput             func(r *http.Request, inputPtr any) error
}

// Router matches one best route per request and executes its handler pipeline.
type Router struct {
	mw_versions
	parse_input     func(r *http.Request, iPtr any) error
	http_mws        []http_mw_with_opts
	task_mws        []task_mw_with_opts
	method_matchers map[string]*method_matcher
	matcher_opts    *matcher.Options
	not_found       http.Handler
	mount_root      string
	all_routes      []AnyRoute
}

// NewRouter creates a Router.
func NewRouter(options ...Options) *Router {
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	mopts := &matcher.Options{
		DynamicParamPrefix:     or_default(opts.DynamicParamPrefix, ':'),
		SplatSegmentIdentifier: or_default(opts.SplatSegmentIdentifier, '*'),
	}
	return &Router{
		parse_input:     opts.ParseInput,
		method_matchers: make(map[string]*method_matcher),
		matcher_opts:    mopts,
		mount_root:      normalize_mount_root(opts.MountRoot),
		http_mws:        empty_http_mws,
		task_mws:        empty_task_mws,
	}
}

// AllRoutes returns a snapshot of all registered routes.
func (rt *Router) AllRoutes() []AnyRoute {
	out := make([]AnyRoute, len(rt.all_routes))
	copy(out, rt.all_routes)
	return out
}

// DynamicParamPrefix returns the configured dynamic param prefix.
func (rt *Router) DynamicParamPrefix() rune { return rt.matcher_opts.DynamicParamPrefix }

// SplatSegmentIdentifier returns the configured splat identifier.
func (rt *Router) SplatSegmentIdentifier() rune { return rt.matcher_opts.SplatSegmentIdentifier }

// MountRoot returns the mount root, optionally joined with a pattern.
// Panics if more than one argument is provided.
func (rt *Router) MountRoot(pattern ...string) string {
	if len(pattern) == 0 {
		return rt.mount_root
	}
	if len(pattern) > 1 {
		panic("MountRoot accepts zero or one argument")
	}
	return path.Join(rt.mount_root, pattern[0])
}

// SetGlobalNotFoundHTTPHandler sets the fallback handler for unmatched requests.
func (rt *Router) SetGlobalNotFoundHTTPHandler(
	h http.Handler,
) {
	rt.not_found = h
}

// ServeHTTP matches the request and executes the resolved route pipeline.
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path_to_use := r.URL.Path
	if rt.mount_root != "" && strings.HasPrefix(path_to_use, rt.mount_root) {
		path_to_use = "/" + path_to_use[len(rt.mount_root):]
	}

	best := rt.find_best(r.Method, path_to_use)
	if !best.did_match {
		if rt.not_found != nil {
			rt.not_found.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	match := best.match
	mm := best.mm
	route := mm.routes[match.OriginalPattern()]

	// Fast path: pure HTTP handler without task middleware.
	if route.get_handler_type() == "http" &&
		!rt.has_any_task_mw(mm, route) &&
		!route.get_needs_tasks_cache() {
		r = request_store.RequestWithContextValue(r, &req_ctx_transport{
			matched_pattern: match.OriginalPattern(),
			params:          match.Params, splat_vals: match.SplatValues, req: r,
		})
		h := route.http_chain(rt, mm)
		if best.head_fallback {
			treat_get_as_head(h, w, r)
		} else {
			h.ServeHTTP(w, r)
		}
		return
	}

	// Slow path: create tasks cache and full request context.
	tasks_cache := tasks.NewCache(r.Context())
	r = request_store.RequestWithContextValue(r, &req_ctx_transport{
		matched_pattern: match.OriginalPattern(),
		params:          match.Params, splat_vals: match.SplatValues,
		tasks_cache: tasks_cache, req: r, response_proxy: response.NewProxy(),
	})

	getter := mm.req_ctx_getters[match.OriginalPattern()]
	rc, err := getter.get_req_ctx(r, tasks_cache, match)
	if err != nil {
		code := http.StatusInternalServerError
		msg := "Internal Server Error"
		if schema.IsValidationError(err) {
			code = http.StatusBadRequest
			msg = err.Error()
			mux_log.Error(
				"Validation error",
				"error",
				err,
				"pattern",
				match.OriginalPattern(),
			)
		} else {
			mux_log.Error("Internal server error", "error", err, "pattern", match.OriginalPattern())
		}
		write_error_with_head(best.head_fallback, w, r, code, msg)
		return
	}

	var final http.Handler
	if route.get_handler_type() == "http" {
		final = route.http_chain(rt, mm)
	} else {
		final = rt.task_final_handler(route, rc)
	}
	h := rt.apply_mw_pipeline(tasks_cache, rc, mm, route, final)
	if best.head_fallback {
		treat_get_as_head(h, w, r)
	} else {
		h.ServeHTTP(w, r)
	}
}

/////////////////////////////////////////////////////////////////////
/////// ROUTE
/////////////////////////////////////////////////////////////////////

// Route stores registration metadata and handlers for a method+pattern pair.
type Route[I, O any] struct {
	genericsutil.ZeroHelper[I, O]
	mw_versions
	router            *Router
	method            string
	original_pattern  string
	http_mws          []http_mw_with_opts
	task_mws          []task_mw_with_opts
	handler_type      string
	user_http         http.Handler
	task_handler      tasks.AnyTask
	needs_tasks_cache bool
	compiled_http     atomic.Value
	compiled_task_mw  atomic.Value
}

// AnyRoute is the type-erased route interface.
type AnyRoute interface {
	OriginalPattern() string
	Method() string
	genericsutil.AnyZeroHelper
	get_handler_type() string
	get_http_handler() http.Handler
	get_task_handler() tasks.AnyTask
	get_http_mws() []http_mw_with_opts
	get_task_mws() []task_mw_with_opts
	get_needs_tasks_cache() bool
	http_chain(rt *Router, mm *method_matcher) http.Handler
	task_mw_chain(rt *Router, mm *method_matcher) []task_mw_with_opts
}

func (r *Route[I, O]) OriginalPattern() string { return r.original_pattern }
func (r *Route[I, O]) Method() string          { return r.method }

func (r *Route[I, O]) get_handler_type() string       { return r.handler_type }
func (r *Route[I, O]) get_http_handler() http.Handler { return r.user_http }

func (r *Route[I, O]) get_task_handler() tasks.AnyTask   { return r.task_handler }
func (r *Route[I, O]) get_http_mws() []http_mw_with_opts { return r.http_mws }
func (r *Route[I, O]) get_task_mws() []task_mw_with_opts { return r.task_mws }

func (r *Route[I, O]) get_needs_tasks_cache() bool { return r.needs_tasks_cache }

/////////////////////////////////////////////////////////////////////
/////// ROUTE REGISTRATION
/////////////////////////////////////////////////////////////////////

// AddTaskHandler registers a task-based route handler.
func AddTaskHandler[I, O any](
	router *Router, method, pattern string, handler *TaskHandler[I, O],
) *Route[I, O] {
	route := new_route[I, O](router, method, pattern)
	route.handler_type = "task"
	route.task_handler = handler
	mm := router.get_or_create_mm(method)
	mm.req_ctx_getters[pattern] = create_req_ctx_getter(route)
	router.register_route(route)
	return route
}

// AddHTTPHandler registers an http.Handler route.
func AddHTTPHandler(
	router *Router, method, pattern string, handler http.Handler,
) *Route[any, any] {
	route := new_route[any, any](router, method, pattern)
	route.handler_type = "http"
	route.user_http = handler
	route.needs_tasks_cache = reflectutil.TypeImplements(
		reflect.TypeOf(handler), needs_tasks_cache_type,
	)
	mm := router.get_or_create_mm(method)
	mm.req_ctx_getters[pattern] = create_req_ctx_getter(route)
	router.register_route(route)
	return route
}

// AddHTTPHandlerFunc registers an http.HandlerFunc route.
func AddHTTPHandlerFunc(
	router *Router, method, pattern string, fn http.HandlerFunc,
) *Route[any, any] {
	return AddHTTPHandler(router, method, pattern, fn)
}

func (rt *Router) AddHTTPHandler(
	method, pattern string,
	h http.Handler,
) *Route[any, any] {
	return AddHTTPHandler(rt, method, pattern, h)
}

func (rt *Router) AddHTTPHandlerFunc(
	method, pattern string,
	fn http.HandlerFunc,
) *Route[any, any] {
	return AddHTTPHandlerFunc(rt, method, pattern, fn)
}

// TaskHandlerFromFunc creates a TaskHandler from a function.
func TaskHandlerFromFunc[I, O any](
	fn TaskHandlerFunc[I, O],
) *TaskHandler[I, O] {
	return tasks.NewTask(
		func(_ *tasks.Cache, rc *RequestCtx[I]) (O, error) { return fn(rc) },
	)
}

// TaskMiddlewareFromFunc creates TaskMiddleware from a function.
func TaskMiddlewareFromFunc[O any](
	fn TaskMiddlewareFunc[O],
) *TaskMiddleware[O] {
	return tasks.NewTask(
		func(_ *tasks.Cache, rc *RequestCtx[None]) (O, error) { return fn(rc) },
	)
}

/////////////////////////////////////////////////////////////////////
/////// MIDDLEWARE REGISTRATION
/////////////////////////////////////////////////////////////////////

func UseMiddleware(
	rt *Router,
	mw Middleware,
	opts ...*MiddlewareOptions,
) {
	rt.http_mws = append(
		rt.http_mws,
		http_mw_with_opts{mw: mw, opts: first_opt(opts)},
	)
	rt.http_ver.Add(1)
}

func UseTaskMiddleware[O any](
	rt *Router,
	mw *TaskMiddleware[O],
	opts ...*MiddlewareOptions,
) {
	rt.task_mws = append(
		rt.task_mws,
		task_mw_with_opts{mw: mw, opts: first_opt(opts)},
	)
	rt.task_ver.Add(1)
}

func (rt *Router) UseMiddleware(
	mw Middleware,
	opts ...*MiddlewareOptions,
) {
	UseMiddleware(rt, mw, opts...)
}

func UseMiddlewareByMethod(
	rt *Router,
	method string,
	mw Middleware,
	opts ...*MiddlewareOptions,
) {
	mm := rt.get_or_create_mm(method)
	mm.http_mws = append(
		mm.http_mws,
		http_mw_with_opts{mw: mw, opts: first_opt(opts)},
	)
	mm.http_ver.Add(1)
}

func UseTaskMiddlewareByMethod[O any](
	rt *Router,
	method string,
	mw *TaskMiddleware[O],
	opts ...*MiddlewareOptions,
) {
	mm := rt.get_or_create_mm(method)
	mm.task_mws = append(
		mm.task_mws,
		task_mw_with_opts{mw: mw, opts: first_opt(opts)},
	)
	mm.task_ver.Add(1)
}

func (rt *Router) UseMiddlewareByMethod(
	method string,
	mw Middleware,
	opts ...*MiddlewareOptions,
) {
	UseMiddlewareByMethod(rt, method, mw, opts...)
}

func UseMiddlewareByPattern[I, O any](
	route *Route[I, O],
	mw Middleware,
	opts ...*MiddlewareOptions,
) {
	route.http_mws = append(
		route.http_mws,
		http_mw_with_opts{mw: mw, opts: first_opt(opts)},
	)
	route.http_ver.Add(1)
}

func UseTaskMiddlewareByPattern[PI, PO, MWO any](
	route *Route[PI, PO],
	mw *TaskMiddleware[MWO],
	opts ...*MiddlewareOptions,
) {
	route.task_mws = append(
		route.task_mws,
		task_mw_with_opts{mw: mw, opts: first_opt(opts)},
	)
	route.task_ver.Add(1)
}

func (route *Route[I, O]) UseMiddlewareByPattern(
	mw Middleware,
	opts ...*MiddlewareOptions,
) {
	UseMiddlewareByPattern(route, mw, opts...)
}

/////////////////////////////////////////////////////////////////////
/////// REQUEST CONTEXT METHODS
/////////////////////////////////////////////////////////////////////

func (rc *RequestCtx[I]) Params() Params           { return rc.params }
func (rc *RequestCtx[I]) Param(key string) string  { return rc.params[key] }
func (rc *RequestCtx[I]) MatchedPattern() string   { return rc.matched_pattern }
func (rc *RequestCtx[I]) SplatValues() []string    { return rc.splat_vals }
func (rc *RequestCtx[I]) TasksCache() *tasks.Cache { return rc.tasks_cache }
func (rc *RequestCtx[I]) Request() *http.Request   { return rc.req }

func (rc *RequestCtx[I]) ResponseProxy() *response.Proxy { return rc.response_proxy }
func (rc *RequestCtx[I]) Input() I                       { return rc.input }

func (rc *RequestCtx[I]) SetTasksCache(ctx *tasks.Cache) { rc.tasks_cache = ctx }
func (rc *RequestCtx[I]) SetRequest(req *http.Request)   { rc.req = req }
func (rc *RequestCtx[I]) get_input() any                 { return rc.input }
func (rc *RequestCtx[I]) get_underlying_req_ctx() any    { return rc }

// ResetForReuse reinitializes fields for pooled reuse.
func (rc *RequestCtx[I]) ResetForReuse(
	matched_pattern string,
	params Params,
	splat []string,
	input I,
	r *http.Request,
	proxy *response.Proxy,
) {
	rc.matched_pattern = matched_pattern
	rc.params = params
	rc.splat_vals = splat
	rc.input = input
	rc.req = r
	rc.response_proxy = proxy
	rc.tasks_cache = nil
}

// ClearForPool zeroes fields before returning to a pool.
func (rc *RequestCtx[I]) ClearForPool() {
	var zero I
	rc.matched_pattern = ""
	rc.params = nil
	rc.splat_vals = nil
	rc.tasks_cache = nil
	rc.input = zero
	rc.req = nil
	rc.response_proxy = nil
}

/////// Response proxy convenience helpers

func (rc *RequestCtx[I]) HeadBuilder() *head.Builder {
	return rc.response_proxy.HeadBuilder()
}
func (rc *RequestCtx[I]) Redirect(url string, code ...int) (bool, error) {
	return rc.response_proxy.Redirect(rc.req, url, code...)
}
func (rc *RequestCtx[I]) SetResponseStatus(status int, text ...string) {
	rc.response_proxy.SetStatus(status, text...)
}

func (rc *RequestCtx[I]) SetResponseCookie(
	c *http.Cookie,
) {
	rc.response_proxy.SetCookie(c)
}

func (rc *RequestCtx[I]) SetResponseHeader(
	k, v string,
) {
	rc.response_proxy.SetHeader(k, v)
}

func (rc *RequestCtx[I]) AddResponseHeader(
	k, v string,
) {
	rc.response_proxy.AddHeader(k, v)
}

func (rc *RequestCtx[I]) ResponseStatus() (int, string) { return rc.response_proxy.Status() }

func (rc *RequestCtx[I]) ResponseHeader(
	k string,
) string {
	return rc.response_proxy.Header(k)
}

func (rc *RequestCtx[I]) ResponseHeaders(
	k string,
) []string {
	return rc.response_proxy.Headers(k)
}

func (rc *RequestCtx[I]) ResponseCookies() []*http.Cookie { return rc.response_proxy.Cookies() }

func (rc *RequestCtx[I]) ResponseLocation() string { return rc.response_proxy.Location() }

func (rc *RequestCtx[I]) IsResponseError() bool { return rc.response_proxy.IsError() }

func (rc *RequestCtx[I]) IsResponseRedirect() bool { return rc.response_proxy.IsRedirect() }

func (rc *RequestCtx[I]) IsResponseSuccess() bool { return rc.response_proxy.IsSuccess() }

/////////////////////////////////////////////////////////////////////
/////// REQUEST DATA ACCESS
/////////////////////////////////////////////////////////////////////

// GetTasksCache extracts the tasks cache from a request.
func GetTasksCache(r *http.Request) *tasks.Cache {
	if rc := request_store.Value(r.Context()); rc != nil {
		return rc.tasks_cache
	}
	return nil
}

// GetMatchedPattern extracts the matched route pattern from a request.
func GetMatchedPattern(r *http.Request) string {
	if rc := request_store.Value(r.Context()); rc != nil {
		return rc.matched_pattern
	}
	return ""
}

// RequestWithTasksCache returns a request carrying only a tasks cache.
func RequestWithTasksCache(r *http.Request, ctx *tasks.Cache) *http.Request {
	return request_store.RequestWithContextValue(r, &req_ctx_transport{
		tasks_cache: ctx, req: r,
	})
}

// GetParam returns one route param by key.
func GetParam(r *http.Request, key string) string { return GetParams(r)[key] }

// GetParams returns all route params for the request.
func GetParams(r *http.Request) Params {
	if rc := request_store.Value(r.Context()); rc != nil {
		return rc.params
	}
	return nil
}

// GetSplatValues returns all matched splat segments.
func GetSplatValues(r *http.Request) []string {
	if rc := request_store.Value(r.Context()); rc != nil &&
		rc.splat_vals != nil {
		return rc.splat_vals
	}
	return empty_splat
}

/////////////////////////////////////////////////////////////////////
/////// TASKS CACHE REQUIRER
/////////////////////////////////////////////////////////////////////

// TasksCacheRequirer marks HTTP handlers that need a tasks cache injected.
type TasksCacheRequirer interface {
	http.Handler
	NeedsTasksCache()
}

// TasksCacheRequirerFunc adapts a function to a TasksCacheRequirer.
type TasksCacheRequirerFunc func(http.ResponseWriter, *http.Request)

func (h TasksCacheRequirerFunc) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	h(w, r)
}

func (h TasksCacheRequirerFunc) NeedsTasksCache() {}

// InjectTasksCacheMiddleware ensures requests carry a tasks cache.
func InjectTasksCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetTasksCache(r) != nil {
			next.ServeHTTP(w, r)
			return
		}
		task_ctx := tasks.NewCache(r.Context())
		next.ServeHTTP(
			w,
			request_store.RequestWithContextValue(r, &req_ctx_transport{
				tasks_cache: task_ctx, req: r,
			}),
		)
	})
}

/////////////////////////////////////////////////////////////////////
/////// NESTED ROUTER
/////////////////////////////////////////////////////////////////////

// NestedOptions configures NestedRouter pattern matching.
type NestedOptions struct {
	DynamicParamPrefix             rune
	SplatSegmentIdentifier         rune
	ExplicitIndexSegmentIdentifier string
	ParseInput                     func(r *http.Request, inputPtr any) error
}

// NestedRouter stores nested route patterns and optional task handlers.
type NestedRouter struct {
	mu              sync.RWMutex
	matcher_inst    *matcher.Matcher
	routes          map[string]AnyNestedRoute
	parse_input     func(r *http.Request, inputPtr any) error
	compiled_routes atomic.Value // compiled_routes_snapshot
}

// NewNestedRouter creates a NestedRouter.
func NewNestedRouter(options ...NestedOptions) *NestedRouter {
	var opts NestedOptions
	if len(options) > 0 {
		opts = options[0]
	}
	mopts := &matcher.Options{
		DynamicParamPrefix: or_default(
			opts.DynamicParamPrefix,
			':',
		),
		SplatSegmentIdentifier: or_default(
			opts.SplatSegmentIdentifier,
			'*',
		),
		ExplicitIndexSegmentIdentifier: opts.ExplicitIndexSegmentIdentifier,
		Quiet:                          false,
	}
	matcher_inst, err := matcher.New(mopts)
	if err != nil {
		panic(err)
	}
	nr := &NestedRouter{
		matcher_inst: matcher_inst,
		routes:       make(map[string]AnyNestedRoute),
		parse_input:  opts.ParseInput,
	}
	nr.compiled_routes.Store(compiled_routes_snapshot{
		routes:    make([]compiled_route, 0),
		index_map: make(map[string]int),
	})
	return nr
}

// AllRoutes returns a snapshot copy of registered routes.
func (nr *NestedRouter) AllRoutes() map[string]AnyNestedRoute {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	out := make(map[string]AnyNestedRoute, len(nr.routes))
	maps.Copy(out, nr.routes)
	return out
}

// IsRegistered reports whether a pattern is registered.
func (nr *NestedRouter) IsRegistered(pattern string) bool {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	_, ok := nr.routes[pattern]
	return ok
}

// HasTaskHandler reports whether a pattern is registered with a handler.
func (nr *NestedRouter) HasTaskHandler(pattern string) bool {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	route, ok := nr.routes[pattern]
	return ok && route.get_nested_task_handler() != nil
}

func (nr *NestedRouter) ExplicitIndexSegmentIdentifier() string {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher_inst.ExplicitIndexSegmentIdentifier()
}
func (nr *NestedRouter) DynamicParamPrefix() rune {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher_inst.DynamicParamPrefix()
}
func (nr *NestedRouter) SplatSegmentIdentifier() rune {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher_inst.SplatSegmentIdentifier()
}

// Matcher returns a copy of the nested matcher.
func (nr *NestedRouter) Matcher() *matcher.Matcher {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	cp, err := matcher.New(nr.current_matcher_opts(true))
	if err != nil {
		panic(err)
	}
	for pat := range nr.routes {
		if _, err := cp.RegisterPattern(pat); err != nil {
			panic(err)
		}
	}
	return cp
}

/////////////////////////////////////////////////////////////////////
/////// NESTED ROUTE
/////////////////////////////////////////////////////////////////////

// NestedRoute stores metadata and an optional task handler for one nested pattern.
type NestedRoute[I, O any] struct {
	genericsutil.ZeroHelper[I, O]
	router           *NestedRouter
	original_pattern string
	task_handler     tasks.AnyTask
	req_ctx_getter   nested_req_ctx_getter
}

// AnyNestedRoute is the type-erased nested route interface.
type AnyNestedRoute interface {
	OriginalPattern() string
	genericsutil.AnyZeroHelper
	get_nested_task_handler() tasks.AnyTask
	get_nested_req_ctx_getter() nested_req_ctx_getter
}

func (r *NestedRoute[I, O]) OriginalPattern() string { return r.original_pattern }

func (r *NestedRoute[I, O]) get_nested_task_handler() tasks.AnyTask {
	return r.task_handler
}

func (r *NestedRoute[I, O]) get_nested_req_ctx_getter() nested_req_ctx_getter {
	return r.req_ctx_getter
}

/////////////////////////////////////////////////////////////////////
/////// NESTED REGISTRATION
/////////////////////////////////////////////////////////////////////

// AddNestedTaskHandler registers a nested pattern with a task handler.
func AddNestedTaskHandler[I, O any](
	router *NestedRouter, pattern string, handler *TaskHandler[I, O],
) *NestedRoute[I, O] {
	route := &NestedRoute[I, O]{
		router: router, original_pattern: pattern, task_handler: handler,
	}
	route.req_ctx_getter = route.new_nested_req_ctx_getter()
	must_register_nested(route)
	return route
}

// AddNestedPatternWithoutHandler registers a nested pattern with no handler.
func AddNestedPatternWithoutHandler(router *NestedRouter, pattern string) {
	route := &NestedRoute[None, None]{
		router: router, original_pattern: pattern, task_handler: nil,
	}
	must_register_nested(route)
}

// AddPatternWithoutHandlerIfMissing registers a pattern without a handler
// only when it is not already present. Returns true if a new route was added.
func (nr *NestedRouter) AddPatternWithoutHandlerIfMissing(pattern string) bool {
	if nr == nil {
		return false
	}
	nr.mu.Lock()
	defer nr.mu.Unlock()
	if _, ok := nr.routes[pattern]; ok {
		return false
	}
	route := &NestedRoute[None, None]{
		router: nr, original_pattern: pattern, task_handler: nil,
	}
	if _, err := nr.matcher_inst.RegisterPattern(pattern); err != nil {
		panic(err)
	}
	nr.routes[pattern] = route
	nr.add_compiled(compiled_route{pattern: pattern, has_handler: false})
	return true
}

/////////////////////////////////////////////////////////////////////
/////// NESTED TASK RESULTS
/////////////////////////////////////////////////////////////////////

// NestedTasksResult stores execution output for one matched nested pattern.
type NestedTasksResult struct {
	pattern  string
	data     any
	err      error
	ran_task bool
}

func (r *NestedTasksResult) Pattern() string { return r.pattern }
func (r *NestedTasksResult) OK() bool        { return r.err == nil }
func (r *NestedTasksResult) Data() any       { return r.data }
func (r *NestedTasksResult) Err() error      { return r.err }
func (r *NestedTasksResult) RanTask() bool   { return r.ran_task }

// NestedTasksResults contains ordered task results for nested matches.
type NestedTasksResults struct {
	Params          Params
	SplatValues     []string
	Results         []*NestedTasksResult
	ResponseProxies []*response.Proxy
}

// HasTaskHandlerAt reports whether the result at index i ran a task handler.
func (r *NestedTasksResults) HasTaskHandlerAt(i int) bool {
	if i < 0 || i >= len(r.Results) {
		return false
	}
	return r.Results[i].ran_task
}

/////////////////////////////////////////////////////////////////////
/////// NESTED MATCHING AND EXECUTION
/////////////////////////////////////////////////////////////////////

// FindNestedMatches resolves nested route matches for the request path.
func FindNestedMatches(
	nr *NestedRouter, r *http.Request,
) (*matcher.FindNestedMatchesResults, bool) {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher_inst.FindNestedMatches(r.URL.Path)
}

// FindNestedMatchesAndRunTasks finds matches and executes all matched handlers.
func FindNestedMatchesAndRunTasks(
	nr *NestedRouter, r *http.Request,
) (*NestedTasksResults, bool) {
	results, ok := FindNestedMatches(nr, r)
	if !ok {
		return nil, false
	}
	return RunNestedTasks(nr, r, results), true
}

// RunNestedTasks executes all task handlers for the matched routes in parallel.
//
// This function uses object pooling for RequestCtx[None]. Safety depends on task
// execution completing before pooled objects are returned, which is guaranteed
// by the blocking WaitGroup/single-goroutine execution pattern below.
func RunNestedTasks(
	nr *NestedRouter,
	r *http.Request,
	find_results *matcher.FindNestedMatchesResults,
) *NestedTasksResults {
	tasks_cache := GetTasksCache(r)
	if tasks_cache == nil {
		nested_log.Error("No TasksCache found in request for RunNestedTasks")
		return nil
	}

	matches := find_results.Matches
	n := len(matches)
	if n == 0 {
		return nil
	}

	results := &NestedTasksResults{
		Params:          find_results.Params,
		SplatValues:     find_results.SplatValues,
		Results:         make([]*NestedTasksResult, n),
		ResponseProxies: make([]*response.Proxy, n),
	}
	results_buf := make([]NestedTasksResult, n)

	snap := nr.current_compiled_snapshot()
	bound := make([]nested_bound_task, 0, n)
	var root_cancel context.CancelFunc

	defer func() {
		if root_cancel != nil {
			root_cancel()
		}
	}()

	for i, match := range matches {
		pat := match.OriginalPattern()
		res := &results_buf[i]
		res.pattern = pat
		results.Results[i] = res

		idx, ok := snap.index_map[pat]
		if !ok || idx >= len(snap.routes) {
			continue
		}
		cr := &snap.routes[idx]
		if !cr.has_handler {
			continue
		}

		res.ran_task = true
		proxy := response.NewProxy()
		results.ResponseProxies[i] = proxy

		rc, err := cr.req_ctx_getter.get_nested_req_ctx(
			r,
			tasks_cache,
			pat,
			results.Params,
			results.SplatValues,
			proxy,
		)
		if err != nil {
			res.err = err
			break
		}

		bound = append(bound, nested_bound_task{
			task_handler: cr.task_handler, req_ctx: rc, result: res,
		})
	}

	// Execute tasks. We intentionally avoid tasksCtx.RunParallel here because
	// its fail-fast cancellation masks per-route errors. Instead, we build a
	// directional cancellation chain: a parent failure cancels only descendants.
	if len(bound) > 0 {
		if len(bound) == 1 {
			bound[0].req_ctx.SetTasksCache(tasks_cache)
		} else {
			current := tasks_cache
			for i := range bound {
				bt := &bound[i]
				if i < len(bound)-1 {
					child_native, cancel := context.WithCancel(current.Context())
					if root_cancel == nil {
						root_cancel = cancel
					}
					child := current.WithContext(child_native)
					bt.req_ctx.SetTasksCache(child)
					bt.req_ctx.SetRequest(bt.req_ctx.Request().WithContext(child_native))
					bt.cancel_descendants = cancel
					current = child
					continue
				}
				bt.req_ctx.SetTasksCache(current)
				bt.req_ctx.SetRequest(bt.req_ctx.Request().WithContext(current.Context()))
			}
		}
		run_nested_bound(bound)
	}

	return results
}

/////////////////////////////////////////////////////////////////////
/////// NESTED ROUTER MANAGEMENT
/////////////////////////////////////////////////////////////////////

// ReplaceRoutes atomically replaces all routes with a new set.
func (nr *NestedRouter) ReplaceRoutes(new_routes map[string]AnyNestedRoute) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	nr.replace_routes_locked(new_routes)
}

// RebuildPreservingHandlers atomically rebuilds the router, preserving
// routes that have task handlers and replacing handler-less routes with
// the provided patterns. Intended for dev-time hot rebuilds.
func (nr *NestedRouter) RebuildPreservingHandlers(patterns []string) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	new_routes := make(map[string]AnyNestedRoute)
	for pat, route := range nr.routes {
		if route.get_nested_task_handler() != nil {
			new_routes[pat] = route
		}
	}
	for _, pat := range patterns {
		if _, ok := new_routes[pat]; !ok {
			new_routes[pat] = &NestedRoute[None, None]{
				router: nr, original_pattern: pat, task_handler: nil,
			}
		}
	}
	nr.replace_routes_locked(new_routes)
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE TYPES
/////////////////////////////////////////////////////////////////////

type req_ctx_transport struct {
	matched_pattern string
	params          Params
	splat_vals      []string
	tasks_cache     *tasks.Cache
	req             *http.Request
	response_proxy  *response.Proxy
}

type mw_versions struct {
	http_ver atomic.Uint64
	task_ver atomic.Uint64
}

type http_mw_with_opts struct {
	mw   Middleware
	opts *MiddlewareOptions
}

type task_mw_with_opts struct {
	mw   tasks.AnyTask
	opts *MiddlewareOptions
}

type method_matcher struct {
	mw_versions
	matcher_inst    *matcher.Matcher
	http_mws        []http_mw_with_opts
	task_mws        []task_mw_with_opts
	routes          map[string]AnyRoute
	req_ctx_getters map[string]req_ctx_getter
}

type compiled_route struct {
	pattern        string
	task_handler   tasks.AnyTask
	req_ctx_getter nested_req_ctx_getter
	has_handler    bool
}

type compiled_routes_snapshot struct {
	routes    []compiled_route
	index_map map[string]int
}

type find_best_result struct {
	mm            *method_matcher
	match         *matcher.BestMatch
	did_match     bool
	head_fallback bool
}

type req_ctx_marker interface {
	get_input() any
	get_underlying_req_ctx() any
	Params() Params
	MatchedPattern() string
	SplatValues() []string
	TasksCache() *tasks.Cache
	SetTasksCache(*tasks.Cache)
	Request() *http.Request
	SetRequest(*http.Request)
	ResponseProxy() *response.Proxy
}

type req_ctx_getter interface {
	get_req_ctx(
		*http.Request,
		*tasks.Cache,
		*matcher.BestMatch,
	) (req_ctx_marker, error)
}

type req_ctx_getter_impl[I any] func(*http.Request, *tasks.Cache, *matcher.BestMatch) (*RequestCtx[I], error)

func (f req_ctx_getter_impl[I]) get_req_ctx(
	r *http.Request, ctx *tasks.Cache, m *matcher.BestMatch,
) (req_ctx_marker, error) {
	return f(r, ctx, m)
}

type nested_req_ctx_getter interface {
	get_nested_req_ctx(
		*http.Request,
		*tasks.Cache,
		string,
		Params,
		[]string,
		*response.Proxy,
	) (req_ctx_marker, error)
}

type nested_req_ctx_getter_impl[I any] func(
	*http.Request,
	*tasks.Cache,
	string,
	Params,
	[]string,
	*response.Proxy,
) (*RequestCtx[I], error)

func (f nested_req_ctx_getter_impl[I]) get_nested_req_ctx(
	r *http.Request,
	ctx *tasks.Cache,
	pattern string,
	params Params,
	splat_values []string,
	proxy *response.Proxy,
) (req_ctx_marker, error) {
	return f(r, ctx, pattern, params, splat_values, proxy)
}

type nested_bound_task struct {
	task_handler       tasks.AnyTask
	req_ctx            req_ctx_marker
	result             *NestedTasksResult
	cancel_descendants context.CancelFunc
}

type mw_bound_task struct {
	task  tasks.AnyTask
	input *RequestCtx[None]
}

func (m *mw_bound_task) Run(ctx *tasks.Cache) error {
	m.input.req = m.input.req.WithContext(ctx.Context())
	_, err := m.task.RunWithAnyInput(ctx, m.input)
	return err
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE: ROUTER INTERNALS
/////////////////////////////////////////////////////////////////////

func or_default[T comparable](val, fallback T) T {
	var zero T
	if val == zero {
		return fallback
	}
	return val
}

func first_opt(opts []*MiddlewareOptions) *MiddlewareOptions {
	if len(opts) > 0 {
		return opts[0]
	}
	return nil
}

func normalize_mount_root(root string) string {
	if root == "" || root == "/" {
		return ""
	}
	if root[0] != '/' {
		root = "/" + root
	}
	if root[len(root)-1] != '/' {
		root = root + "/"
	}
	return root
}

func new_route[I, O any](rt *Router, method, pattern string) *Route[I, O] {
	return &Route[I, O]{
		router: rt, method: method, original_pattern: pattern,
		http_mws: empty_http_mws, task_mws: empty_task_mws,
	}
}

func (rt *Router) register_route(route AnyRoute) {
	mm := rt.get_or_create_mm(route.Method())
	if _, err := mm.matcher_inst.RegisterPattern(route.OriginalPattern()); err != nil {
		panic(err)
	}
	mm.routes[route.OriginalPattern()] = route
	rt.all_routes = append(rt.all_routes, route)
}

func (rt *Router) get_or_create_mm(method string) *method_matcher {
	if mm, ok := rt.method_matchers[method]; ok {
		return mm
	}
	matcher_inst, err := matcher.New(rt.matcher_opts)
	if err != nil {
		panic(err)
	}
	mm := &method_matcher{
		matcher_inst:    matcher_inst,
		routes:          make(map[string]AnyRoute),
		req_ctx_getters: make(map[string]req_ctx_getter),
		http_mws:        empty_http_mws,
		task_mws:        empty_task_mws,
	}
	rt.method_matchers[method] = mm
	return mm
}

func create_req_ctx_getter[I, O any](route *Route[I, O]) req_ctx_getter {
	return req_ctx_getter_impl[I](
		func(r *http.Request, ctx *tasks.Cache, match *matcher.BestMatch) (*RequestCtx[I], error) {
			rc := new(RequestCtx[I])
			rc.matched_pattern = match.OriginalPattern()
			rc.params = match.Params
			rc.splat_vals = match.SplatValues
			rc.tasks_cache = ctx
			rc.req = r
			rc.response_proxy = response.NewProxy()
			ptr := route.IPtr()
			if route.handler_type == "task" &&
				route.router.parse_input != nil &&
				!genericsutil.IsNone(route.I()) {
				if err := route.router.parse_input(rc.Request(), ptr); err != nil {
					return nil, err
				}
			}
			rc.input = *(ptr.(*I))
			return rc, nil
		},
	)
}

func (route *NestedRoute[I, O]) new_nested_req_ctx_getter() nested_req_ctx_getter {
	return nested_req_ctx_getter_impl[I](
		func(
			r *http.Request,
			ctx *tasks.Cache,
			pattern string,
			params Params,
			splat_values []string,
			proxy *response.Proxy,
		) (*RequestCtx[I], error) {
			rc := new(RequestCtx[I])
			rc.matched_pattern = pattern
			rc.params = params
			rc.splat_vals = splat_values
			rc.tasks_cache = ctx
			rc.req = r
			rc.response_proxy = proxy
			ptr := route.IPtr()
			if route.router.parse_input != nil && !genericsutil.IsNone(route.I()) {
				if err := route.router.parse_input(rc.Request(), ptr); err != nil {
					return nil, err
				}
			}
			rc.input = *(ptr.(*I))
			return rc, nil
		},
	)
}

func (rt *Router) find_best(method, real_path string) *find_best_result {
	is_head := method == http.MethodHead
	if is_head {
		if mm, ok := rt.method_matchers[http.MethodHead]; ok {
			if m, ok := mm.matcher_inst.FindBestMatch(real_path); ok {
				return &find_best_result{mm: mm, match: m, did_match: true}
			}
		}
		method = http.MethodGet
	}
	mm, ok := rt.method_matchers[method]
	if !ok {
		return &find_best_result{}
	}
	m, ok := mm.matcher_inst.FindBestMatch(real_path)
	if !ok {
		return &find_best_result{}
	}
	return &find_best_result{
		mm:            mm,
		match:         m,
		did_match:     true,
		head_fallback: is_head,
	}
}

func (rt *Router) has_any_task_mw(mm *method_matcher, route AnyRoute) bool {
	return len(route.get_task_mws()) > 0 || len(mm.task_mws) > 0 ||
		len(rt.task_mws) > 0
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE: HTTP MIDDLEWARE CHAIN (VERSION-CACHED)
/////////////////////////////////////////////////////////////////////

type cached_http_chain struct {
	handler                        http.Handler
	global_ver, method_ver, rt_ver uint64
}

func (r *Route[I, O]) http_chain(rt *Router, mm *method_matcher) http.Handler {
	gv := rt.http_ver.Load()
	mv := mm.http_ver.Load()
	rv := r.http_ver.Load()
	if c, ok := r.compiled_http.Load().(*cached_http_chain); ok &&
		c.global_ver == gv && c.method_ver == mv && c.rt_ver == rv {
		return c.handler
	}
	h := apply_http_mws(
		r.get_http_handler(),
		r.http_mws,
		mm.http_mws,
		rt.http_mws,
	)
	r.compiled_http.Store(
		&cached_http_chain{
			handler:    h,
			global_ver: gv,
			method_ver: mv,
			rt_ver:     rv,
		},
	)
	return h
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE: TASK MIDDLEWARE CHAIN (VERSION-CACHED)
/////////////////////////////////////////////////////////////////////

type cached_task_mw_chain struct {
	middlewares                    []task_mw_with_opts
	global_ver, method_ver, rt_ver uint64
}

func (r *Route[I, O]) task_mw_chain(
	rt *Router,
	mm *method_matcher,
) []task_mw_with_opts {
	gv := rt.task_ver.Load()
	mv := mm.task_ver.Load()
	rv := r.task_ver.Load()
	if c, ok := r.compiled_task_mw.Load().(*cached_task_mw_chain); ok &&
		c.global_ver == gv && c.method_ver == mv && c.rt_ver == rv {
		return c.middlewares
	}
	total := len(rt.task_mws) + len(mm.task_mws) + len(r.task_mws)
	var mws []task_mw_with_opts
	if total > 0 {
		mws = make([]task_mw_with_opts, 0, total)
		mws = append(mws, rt.task_mws...)
		mws = append(mws, mm.task_mws...)
		mws = append(mws, r.task_mws...)
	}
	r.compiled_task_mw.Store(&cached_task_mw_chain{
		middlewares: mws, global_ver: gv, method_ver: mv, rt_ver: rv,
	})
	return mws
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE: MIDDLEWARE APPLICATION
/////////////////////////////////////////////////////////////////////

func apply_http_mw(mwo http_mw_with_opts, h http.Handler) http.Handler {
	if mwo.opts != nil && mwo.opts.If != nil {
		orig := h
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mwo.opts.If(r) {
				mwo.mw(orig).ServeHTTP(w, r)
			} else {
				orig.ServeHTTP(w, r)
			}
		})
	}
	return mwo.mw(h)
}

func apply_http_mws(
	h http.Handler,
	route_mws, method_mws, global_mws []http_mw_with_opts,
) http.Handler {
	for i := len(route_mws) - 1; i >= 0; i-- {
		h = apply_http_mw(route_mws[i], h)
	}
	for i := len(method_mws) - 1; i >= 0; i-- {
		h = apply_http_mw(method_mws[i], h)
	}
	for i := len(global_mws) - 1; i >= 0; i-- {
		h = apply_http_mw(global_mws[i], h)
	}
	return h
}

func (rt *Router) task_final_handler(
	route AnyRoute,
	rc req_ctx_marker,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)
		data, err := route.get_task_handler().
			RunWithAnyInput(rc.TasksCache(), rc.get_underlying_req_ctx())
		if err != nil {
			mux_log.Error(
				"Error executing task handler",
				"error",
				err,
				"pattern",
				route.OriginalPattern(),
			)
			res.InternalServerError()
			return
		}
		proxy := rc.ResponseProxy()
		proxy.ApplyToResponseWriter(w, r)
		if proxy.IsError() || proxy.IsRedirect() {
			return
		}
		if reflectutil.IsNilLikeExceptNone(data) {
			mux_log.Warn(
				"Do not return nil values from task handlers unless the type is an empty struct or you are returning an error.",
				"pattern",
				route.OriginalPattern(),
			)
		}
		res.JSON(data)
	})
}

func (rt *Router) apply_mw_pipeline(
	tasks_cache *tasks.Cache,
	rc req_ctx_marker,
	mm *method_matcher,
	route AnyRoute,
	final http.Handler,
) http.Handler {
	var with_http http.Handler
	if route.get_handler_type() == "http" {
		with_http = final
	} else {
		with_http = apply_http_mws(final, route.get_http_mws(), mm.http_mws, rt.http_mws)
	}
	collected := route.task_mw_chain(rt, mm)
	if len(collected) == 0 {
		return with_http
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bound := make([]tasks.Prepared, 0, len(collected))
		proxies := make([]*response.Proxy, 0, len(collected))
		for _, tw := range collected {
			if tw.opts != nil && tw.opts.If != nil && !tw.opts.If(r) {
				continue
			}
			p := response.NewProxy()
			rc := &RequestCtx[None]{
				matched_pattern: rc.MatchedPattern(),
				params:          rc.Params(),
				splat_vals:      rc.SplatValues(),
				tasks_cache:     tasks_cache,
				input:           None{},
				req:             r,
				response_proxy:  p,
			}
			proxies = append(proxies, p)
			bound = append(bound, &mw_bound_task{task: tw.mw, input: rc})
		}
		if len(bound) == 0 {
			with_http.ServeHTTP(w, r)
			return
		}
		if err := tasks_cache.RunParallel(bound...); err != nil {
			mux_log.Error(
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
		merged := response.MergeProxyResponses(proxies...)
		merged.ApplyToResponseWriter(w, r)
		if merged.IsError() || merged.IsRedirect() {
			return
		}
		with_http.ServeHTTP(w, r)
	})
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE: HEAD RESPONSE HANDLING
/////////////////////////////////////////////////////////////////////

type head_writer struct {
	header        http.Header
	status        int
	wrote_header  bool
	bytes_written int
}

func (hw *head_writer) Header() http.Header { return hw.header }

func (hw *head_writer) WriteHeader(code int) {
	if hw.wrote_header {
		return
	}
	hw.wrote_header = true
	hw.status = code
}

func (hw *head_writer) Write(data []byte) (int, error) {
	if !hw.wrote_header {
		hw.WriteHeader(http.StatusOK)
	}
	if len(data) > 0 && hw.header.Get("Content-Type") == "" &&
		hw.header.Get("Transfer-Encoding") == "" {
		n := min(len(data), 512)
		hw.header.Set("Content-Type", http.DetectContentType(data[:n]))
	}
	hw.bytes_written += len(data)
	return len(data), nil
}

func treat_get_as_head(h http.Handler, w http.ResponseWriter, r *http.Request) {
	hw := &head_writer{header: make(http.Header), status: http.StatusOK}
	h.ServeHTTP(hw, r)
	for k, vals := range hw.Header() {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	if w.Header().Get("Content-Length") == "" &&
		w.Header().Get("Transfer-Encoding") == "" {
		w.Header().Set("Content-Length", strconv.Itoa(hw.bytes_written))
	}
	w.WriteHeader(hw.status)
}

func write_error_with_head(
	is_head bool,
	w http.ResponseWriter,
	r *http.Request,
	code int,
	msg string,
) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, msg, code)
	})
	if is_head {
		treat_get_as_head(h, w, r)
	} else {
		h.ServeHTTP(w, r)
	}
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE: NESTED ROUTER INTERNALS
/////////////////////////////////////////////////////////////////////

func (nr *NestedRouter) current_compiled_snapshot() compiled_routes_snapshot {
	snap, ok := nr.compiled_routes.Load().(compiled_routes_snapshot)
	if !ok {
		return compiled_routes_snapshot{}
	}
	return snap
}

func (nr *NestedRouter) add_compiled(cr compiled_route) {
	snap := nr.current_compiled_snapshot()
	new_routes := make([]compiled_route, len(snap.routes)+1)
	copy(new_routes, snap.routes)
	new_routes[len(snap.routes)] = cr
	new_idx := make(map[string]int, len(snap.index_map)+1)
	maps.Copy(new_idx, snap.index_map)
	new_idx[cr.pattern] = len(snap.routes)
	nr.compiled_routes.Store(
		compiled_routes_snapshot{routes: new_routes, index_map: new_idx},
	)
}

func must_register_nested[I, O any](route *NestedRoute[I, O]) {
	route.router.mu.Lock()
	defer route.router.mu.Unlock()
	if _, ok := route.router.routes[route.original_pattern]; ok {
		panic(fmt.Sprintf(
			"Pattern '%s' is already registered in NestedRouter.",
			route.original_pattern,
		))
	}
	if _, err := route.router.matcher_inst.RegisterPattern(
		route.original_pattern,
	); err != nil {
		panic(err)
	}
	route.router.routes[route.original_pattern] = route
	route.router.add_compiled(compiled_route{
		pattern:        route.original_pattern,
		task_handler:   route.task_handler,
		req_ctx_getter: route.req_ctx_getter,
		has_handler:    route.task_handler != nil,
	})
}

func (nr *NestedRouter) replace_routes_locked(
	new_routes map[string]AnyNestedRoute,
) {
	cp := make(map[string]AnyNestedRoute, len(new_routes))
	maps.Copy(cp, new_routes)

	new_matcher, err := matcher.New(nr.current_matcher_opts(true))
	if err != nil {
		panic(err)
	}
	for pat := range cp {
		if _, err := new_matcher.RegisterPattern(pat); err != nil {
			panic(err)
		}
	}

	compiled := make([]compiled_route, 0, len(cp))
	idx := make(map[string]int, len(cp))
	for pat, route := range cp {
		th := route.get_nested_task_handler()
		idx[pat] = len(compiled)
		compiled = append(compiled, compiled_route{
			pattern:        pat,
			task_handler:   th,
			req_ctx_getter: route.get_nested_req_ctx_getter(),
			has_handler:    th != nil,
		})
	}

	nr.matcher_inst = new_matcher
	nr.routes = cp
	nr.compiled_routes.Store(
		compiled_routes_snapshot{routes: compiled, index_map: idx},
	)
}

func (nr *NestedRouter) current_matcher_opts(quiet bool) *matcher.Options {
	return &matcher.Options{
		DynamicParamPrefix:             nr.matcher_inst.DynamicParamPrefix(),
		SplatSegmentIdentifier:         nr.matcher_inst.SplatSegmentIdentifier(),
		ExplicitIndexSegmentIdentifier: nr.matcher_inst.ExplicitIndexSegmentIdentifier(),
		Quiet:                          quiet,
	}
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE: NESTED TASK EXECUTION
/////////////////////////////////////////////////////////////////////

func (bt *nested_bound_task) run() error {
	data, err := bt.task_handler.RunWithAnyInput(
		bt.req_ctx.TasksCache(),
		bt.req_ctx,
	)
	bt.result.data = data
	bt.result.err = err
	if err != nil && bt.cancel_descendants != nil {
		bt.cancel_descendants()
	}
	return err
}

func run_nested_bound(bound []nested_bound_task) {
	switch len(bound) {
	case 0:
		return
	case 1:
		_ = bound[0].run()
		return
	}
	var wg sync.WaitGroup
	wg.Add(len(bound) - 1)
	for i := 1; i < len(bound); i++ {
		go func(bt *nested_bound_task) {
			defer wg.Done()
			_ = bt.run()
		}(&bound[i])
	}
	_ = bound[0].run()
	wg.Wait()
}
