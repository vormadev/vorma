package nestedmux

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/genericsutil"
	"github.com/vormadev/vorma/kit/internal/muxcore"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmatcher"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/tasks"
)

var (
	nestedMuxLog = colorlog.New("nestedmux")
	noneInstance = mux.None{}
	reqDataPool  = sync.Pool{
		New: func() any {
			return &mux.ReqData[mux.None]{}
		},
	}
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC API
/////////////////////////////////////////////////////////////////////

type ReqData = mux.ReqData[mux.None]

type compiledRoute struct {
	pattern     string
	taskHandler tasks.AnyTask
	hasHandler  bool
}

type compiledRoutesSnapshot struct {
	compiledRoutes []compiledRoute
	routeIndexMap  map[string]int
}

// Router stores nested route patterns and optional task handlers.
type Router struct {
	mu                  sync.RWMutex
	matcher             *nestedmatcher.Matcher
	routes              map[string]AnyRoute
	compiledRoutesState atomic.Value // compiledRoutesSnapshot
}

func (nr *Router) currentCompiledRoutesSnapshot() compiledRoutesSnapshot {
	snapshot, loaded := nr.compiledRoutesState.Load().(compiledRoutesSnapshot)
	if !loaded {
		return compiledRoutesSnapshot{}
	}
	return snapshot
}

// AllRoutes returns a snapshot copy of registered nested routes.
func (nr *Router) AllRoutes() map[string]AnyRoute {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	allRoutesCopy := make(map[string]AnyRoute, len(nr.routes))
	maps.Copy(allRoutesCopy, nr.routes)
	return allRoutesCopy
}

// IsRegistered reports whether a pattern is registered.
func (nr *Router) IsRegistered(originalPattern string) bool {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	_, exists := nr.routes[originalPattern]
	return exists
}

// HasTaskHandler reports whether a pattern is registered with a handler.
func (nr *Router) HasTaskHandler(originalPattern string) bool {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	route, exists := nr.routes[originalPattern]
	if !exists {
		return false
	}
	return route.getTaskHandler() != nil
}

// ExplicitIndexSegmentIdentifier returns the explicit index marker string.
func (nr *Router) ExplicitIndexSegmentIdentifier() string {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher.ExplicitIndexSegmentIdentifier()
}

// DynamicParamPrefix returns the dynamic param prefix rune.
func (nr *Router) DynamicParamPrefix() rune {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher.DynamicParamPrefix()
}

// SplatSegmentIdentifier returns the splat segment identifier rune.
func (nr *Router) SplatSegmentIdentifier() rune {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher.SplatSegmentIdentifier()
}

// Matcher returns a copy of the nested matcher state.
func (nr *Router) Matcher() *nestedmatcher.Matcher {
	nr.mu.RLock()
	defer nr.mu.RUnlock()

	matcherCopy := nestedmatcher.New(nr.currentMatcherOptions(true))
	for pattern := range nr.routes {
		matcherCopy.RegisterPattern(pattern)
	}
	return matcherCopy
}

// Options configures pattern matching behavior for Router.
type Options struct {
	DynamicParamPrefix             rune
	SplatSegmentIdentifier         rune
	ExplicitIndexSegmentIdentifier string
}

// NewRouter creates a nested router with the provided options.
func NewRouter(opts *Options) *Router {
	if opts == nil {
		opts = new(Options)
	}
	matcherOpts := muxcore.BuildNestedMatcherOptions(
		muxcore.NestedMatcherOptionsInput{
			DynamicParamPrefix:             opts.DynamicParamPrefix,
			SplatSegmentIdentifier:         opts.SplatSegmentIdentifier,
			ExplicitIndexSegmentIdentifier: opts.ExplicitIndexSegmentIdentifier,
			Quiet:                          false,
		},
	)
	nr := &Router{
		matcher: nestedmatcher.New(matcherOpts),
		routes:  make(map[string]AnyRoute),
	}
	nr.compiledRoutesState.Store(compiledRoutesSnapshot{
		compiledRoutes: make([]compiledRoute, 0),
		routeIndexMap:  make(map[string]int),
	})
	return nr
}

// Route stores metadata and optional task handler for one nested pattern.
type Route[O any] struct {
	genericsutil.ZeroHelper[mux.None, O]
	router          *Router
	originalPattern string
	taskHandler     tasks.AnyTask
}

// AnyRoute is the internal polymorphic route contract for Router.
type AnyRoute interface {
	OriginalPattern() string
	genericsutil.AnyZeroHelper
	getTaskHandler() tasks.AnyTask
}

// OriginalPattern returns the pattern as registered.
func (route *Route[O]) OriginalPattern() string {
	return route.originalPattern
}

func (route *Route[O]) getTaskHandler() tasks.AnyTask {
	return route.taskHandler
}

// AddTaskHandler registers a nested pattern with a task handler.
func AddTaskHandler[O any](
	router *Router,
	pattern string,
	taskHandler *mux.TaskHandler[mux.None, O],
) *Route[O] {
	route := &Route[O]{
		router:          router,
		originalPattern: pattern,
		taskHandler:     taskHandler,
	}
	mustRegisterRoute(route)
	return route
}

// AddPatternWithoutHandler registers a nested pattern with no task handler.
func AddPatternWithoutHandler(router *Router, pattern string) {
	route := &Route[mux.None]{
		router:          router,
		originalPattern: pattern,
		taskHandler:     nil,
	}
	mustRegisterRoute(route)
}

// AddPatternWithoutHandlerIfMissing registers a pattern without a task
// handler only when it is not already present. It returns true if a new route
// was registered.
func (nr *Router) AddPatternWithoutHandlerIfMissing(
	pattern string,
) bool {
	if nr == nil {
		return false
	}

	nr.mu.Lock()
	defer nr.mu.Unlock()

	if _, exists := nr.routes[pattern]; exists {
		return false
	}

	route := &Route[mux.None]{
		router:          nr,
		originalPattern: pattern,
		taskHandler:     nil,
	}
	nr.matcher.RegisterPattern(pattern)
	nr.routes[pattern] = route
	nr.addCompiledRoute(compiledRoute{
		pattern:     pattern,
		taskHandler: nil,
		hasHandler:  false,
	})

	return true
}

// TasksResult stores task execution output for one matched pattern.
type TasksResult struct {
	pattern string
	data    any
	err     error
	ranTask bool
}

// Pattern returns the matched route pattern.
func (ntr *TasksResult) Pattern() string { return ntr.pattern }

// OK reports whether task execution succeeded.
func (ntr *TasksResult) OK() bool { return ntr.err == nil }

// Data returns task output data.
func (ntr *TasksResult) Data() any { return ntr.data }

// Err returns task execution error.
func (ntr *TasksResult) Err() error { return ntr.err }

// RanTask reports whether this match had a task handler.
func (ntr *TasksResult) RanTask() bool { return ntr.ranTask }

// TasksResults contains ordered and keyed task results for nested matches.
type TasksResults struct {
	Params          mux.Params
	SplatValues     []string
	Map             map[string]*TasksResult
	Slice           []*TasksResult
	ResponseProxies []*response.Proxy
}

// HasTaskHandlerAt reports whether the result at i ran a task handler.
func (ntr *TasksResults) HasTaskHandlerAt(i int) bool {
	if i < 0 || i >= len(ntr.Slice) {
		return false
	}
	return ntr.Slice[i].ranTask
}

// FindMatches resolves nested route matches for the request path.
func FindMatches(
	nestedRouter *Router,
	r *http.Request,
) (*nestedmatcher.Results, bool) {
	nestedRouter.mu.RLock()
	defer nestedRouter.mu.RUnlock()
	return nestedRouter.matcher.FindMatches(r.URL.Path)
}

// FindMatchesAndRunTasks finds nested matches and runs all matched task handlers.
func FindMatchesAndRunTasks(
	nestedRouter *Router,
	r *http.Request,
) (*TasksResults, bool) {
	findResults, ok := FindMatches(nestedRouter, r)
	if !ok {
		return nil, false
	}
	return RunTasks(nestedRouter, r, findResults), true
}

// RunTasks executes all task handlers for the matched routes in parallel.
//
// IMPORTANT: This function uses object pooling for mux.ReqData objects. The safety of this
// depends on tasksCtx.RunParallel blocking until all tasks complete. If RunParallel
// were to return before tasks finish (async dispatch), this would cause use-after-free bugs.
// The current tasks.Ctx implementation blocks until completion, making this safe.
func RunTasks(
	nestedRouter *Router,
	r *http.Request,
	findNestedMatchesResults *nestedmatcher.Results,
) *TasksResults {
	return runTasks(
		nestedRouter,
		r,
		findNestedMatchesResults,
		true,
	)
}

// RunTasksWithoutPatternMap executes matched handlers while skipping
// TasksResults.Map materialization. This is useful for high-throughput callers
// that consume ordered results only.
func RunTasksWithoutPatternMap(
	nestedRouter *Router,
	r *http.Request,
	findNestedMatchesResults *nestedmatcher.Results,
) *TasksResults {
	return runTasks(
		nestedRouter,
		r,
		findNestedMatchesResults,
		false,
	)
}

func runTasks(
	nestedRouter *Router,
	r *http.Request,
	findNestedMatchesResults *nestedmatcher.Results,
	includePatternMap bool,
) *TasksResults {
	tasksCtx := mux.GetTasksCtx(r)
	if tasksCtx == nil {
		nestedMuxLog.Error("No TasksCtx found in request for RunTasks")
		return nil
	}

	matches := findNestedMatchesResults.Matches
	numMatches := len(matches)
	if numMatches == 0 {
		return nil
	}

	// Create results structure with pre-allocated capacity
	results := &TasksResults{
		Params:          findNestedMatchesResults.Params,
		SplatValues:     findNestedMatchesResults.SplatValues,
		Slice:           make([]*TasksResult, numMatches),
		ResponseProxies: make([]*response.Proxy, numMatches),
	}
	if includePatternMap {
		results.Map = make(map[string]*TasksResult, numMatches)
	}
	resultsByRoute := make([]TasksResult, numMatches)

	compiledRoutesSnapshotForRun := nestedRouter.currentCompiledRoutesSnapshot()
	compiledRoutes := compiledRoutesSnapshotForRun.compiledRoutes
	routeIndexMap := compiledRoutesSnapshotForRun.routeIndexMap

	// Pre-allocate boundTasks based on estimated task count
	boundTasks := make(
		[]optimizedBoundTask,
		0,
		numMatches,
	)
	var rootDescendantsCancel context.CancelFunc

	// Ensure cleanup happens after task execution completes.
	defer func() {
		if rootDescendantsCancel != nil {
			rootDescendantsCancel()
		}
		for i := range boundTasks {
			boundTask := &boundTasks[i]
			tasks.ReleaseSharedStateChildContext(boundTask.borrowedTasksCtx)
			if boundTask.reqData != nil {
				boundTask.reqData.ClearForPool()
				reqDataPool.Put(boundTask.reqData)
			}
		}
	}()

	// Single pass with optimized lookup
	for i, match := range matches {
		pattern := match.OriginalPattern()

		// Get pooled result
		result := &resultsByRoute[i]
		result.pattern = pattern

		if includePatternMap {
			results.Map[pattern] = result
		}
		results.Slice[i] = result

		// Fast lookup using pre-computed index
		idx, exists := routeIndexMap[pattern]
		if !exists || idx >= len(compiledRoutes) {
			continue
		}

		compiled := &compiledRoutes[idx]
		if !compiled.hasHandler {
			continue
		}

		result.ranTask = true

		// Create response proxy
		proxy := response.NewProxy()
		results.ResponseProxies[i] = proxy

		// Get pooled mux.ReqData and fully initialize it.
		reqData := reqDataPool.Get().(*mux.ReqData[mux.None])
		reqData.ResetForReuse(
			results.Params,
			results.SplatValues,
			noneInstance,
			r,
			proxy,
		)

		boundTask := optimizedBoundTask{
			taskHandler:       compiled.taskHandler,
			reqData:           reqData,
			result:            result,
			cancelDescendants: nil,
		}
		boundTasks = append(boundTasks, boundTask)
	}

	// Execute all tasks in parallel if we have any.
	// We intentionally do NOT use tasksCtx.RunParallel here because that path
	// fail-fast cancels sibling tasks on first error, which can mask per-route
	// errors in nested routing.
	if len(boundTasks) > 0 {
		if len(boundTasks) == 1 {
			boundTasks[0].reqData.SetTasksCtx(tasksCtx)
		} else {
			currentCtx := tasksCtx
			for i := range boundTasks {
				bt := &boundTasks[i]
				if i < len(boundTasks)-1 {
					childNativeCtx, cancel := context.WithCancel(currentCtx.NativeContext())
					if rootDescendantsCancel == nil {
						rootDescendantsCancel = cancel
					}
					childCtx := currentCtx.AcquireSharedStateChildContextWithNativeContext(
						childNativeCtx,
					)
					bt.reqData.SetTasksCtx(childCtx)
					bt.borrowedTasksCtx = childCtx
					bt.cancelDescendants = cancel
					currentCtx = childCtx
					continue
				}
				// Leaf task reuses the current ancestor-cancelable context.
				bt.reqData.SetTasksCtx(currentCtx)
			}
		}
		runBoundTasks(boundTasks)
	}

	return results
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE API
/////////////////////////////////////////////////////////////////////

func mustRegisterRoute[O any](route *Route[O]) {
	route.router.mu.Lock()
	defer route.router.mu.Unlock()

	if _, exists := route.router.routes[route.originalPattern]; exists {
		panic(
			fmt.Sprintf(
				"Pattern '%s' is already registered in Router. Perhaps you're unintentionally registering it twice?",
				route.originalPattern,
			),
		)
	}
	route.router.matcher.RegisterPattern(route.originalPattern)
	route.router.routes[route.originalPattern] = route
	route.router.addCompiledRoute(compiledRoute{
		pattern:     route.originalPattern,
		taskHandler: route.taskHandler,
		hasHandler:  route.taskHandler != nil,
	})
}

// addCompiledRoute adds a compiled route with atomic update
// Must be called with mu.Lock held
func (nr *Router) addCompiledRoute(compiled compiledRoute) {
	currentSnapshot := nr.currentCompiledRoutesSnapshot()
	currentRoutes := currentSnapshot.compiledRoutes
	currentIndexMap := currentSnapshot.routeIndexMap

	// Create new slices/maps
	newRoutes := make([]compiledRoute, len(currentRoutes)+1)
	copy(newRoutes, currentRoutes)
	newRoutes[len(currentRoutes)] = compiled

	newIndexMap := make(map[string]int, len(currentIndexMap)+1)
	maps.Copy(newIndexMap, currentIndexMap)
	newIndexMap[compiled.pattern] = len(currentRoutes)

	nr.compiledRoutesState.Store(compiledRoutesSnapshot{
		compiledRoutes: newRoutes,
		routeIndexMap:  newIndexMap,
	})
}

type optimizedBoundTask struct {
	taskHandler tasks.AnyTask
	reqData     *mux.ReqData[mux.None]
	result      *TasksResult
	// Non-nil only when this task owns a borrowed child tasks context.
	borrowedTasksCtx *tasks.Ctx
	// Called when this task fails; should cancel deeper matched routes only.
	cancelDescendants context.CancelFunc
}

func (oc *optimizedBoundTask) Run() error {
	data, err := oc.taskHandler.RunWithAnyInput(
		oc.reqData.TasksCtx(),
		oc.reqData,
	)
	oc.result.data = data
	oc.result.err = err
	if err != nil && oc.cancelDescendants != nil {
		oc.cancelDescendants()
	}
	return err
}

func runBoundTasks(boundTasks []optimizedBoundTask) {
	switch len(boundTasks) {
	case 0:
		return
	case 1:
		_ = boundTasks[0].Run()
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(boundTasks) - 1)
	for i := 1; i < len(boundTasks); i++ {
		go runBoundTaskInGoroutine(&wg, &boundTasks[i])
	}
	_ = boundTasks[0].Run()
	wg.Wait()
}

func runBoundTaskInGoroutine(
	wg *sync.WaitGroup,
	boundTask *optimizedBoundTask,
) {
	defer wg.Done()
	_ = boundTask.Run()
}

// ReplaceRoutes atomically replaces all routes with a new set.
// This rebuilds the matcher from scratch, which is safe and fast for dev mode.
func (nr *Router) ReplaceRoutes(newRoutes map[string]AnyRoute) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	nr.replaceRoutesLocked(newRoutes)
}

// RebuildPreservingHandlers atomically rebuilds the router, preserving routes
// that have task handlers and replacing handler-less routes with the provided patterns.
// This is intended for dev-time fast rebuilds when only pattern definitions change.
func (nr *Router) RebuildPreservingHandlers(patterns []string) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	newRoutes := make(map[string]AnyRoute)

	// Preserve existing routes with handlers
	for pattern, route := range nr.routes {
		if route.getTaskHandler() != nil {
			newRoutes[pattern] = route
		}
	}

	// Add patterns without handlers
	for _, pattern := range patterns {
		if _, exists := newRoutes[pattern]; !exists {
			newRoutes[pattern] = &Route[mux.None]{
				router:          nr,
				originalPattern: pattern,
				taskHandler:     nil,
			}
		}
	}

	nr.replaceRoutesLocked(newRoutes)
}

// replaceRoutesLocked is the internal implementation. Caller must hold nr.mu.Lock().
func (nr *Router) replaceRoutesLocked(
	newRoutes map[string]AnyRoute,
) {
	routesCopy := make(map[string]AnyRoute, len(newRoutes))
	maps.Copy(routesCopy, newRoutes)

	newMatcher := nestedmatcher.New(nr.currentMatcherOptions(true))

	for pattern := range routesCopy {
		newMatcher.RegisterPattern(pattern)
	}

	newCompiled := make([]compiledRoute, 0, len(routesCopy))
	newIndexMap := make(map[string]int, len(routesCopy))

	for pattern, route := range routesCopy {
		taskHandler := route.getTaskHandler()
		compiled := compiledRoute{
			pattern:     pattern,
			taskHandler: taskHandler,
			hasHandler:  taskHandler != nil,
		}
		newIndexMap[pattern] = len(newCompiled)
		newCompiled = append(newCompiled, compiled)
	}

	nr.matcher = newMatcher
	nr.routes = routesCopy
	nr.compiledRoutesState.Store(compiledRoutesSnapshot{
		compiledRoutes: newCompiled,
		routeIndexMap:  newIndexMap,
	})
}

func (nr *Router) currentMatcherOptions(quiet bool) *nestedmatcher.Options {
	return &nestedmatcher.Options{
		DynamicParamPrefix:             nr.matcher.DynamicParamPrefix(),
		SplatSegmentIdentifier:         nr.matcher.SplatSegmentIdentifier(),
		ExplicitIndexSegmentIdentifier: nr.matcher.ExplicitIndexSegmentIdentifier(),
		Quiet:                          quiet,
	}
}
