package mux

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/vormadev/vorma/kit/genericsutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/tasks"
)

var (
	noneInstance = None{}
	reqDataPool  = sync.Pool{
		New: func() any {
			return &ReqData[None]{input: noneInstance}
		},
	}
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC API
/////////////////////////////////////////////////////////////////////

type NestedReqData = ReqData[None]

type compiledRoute struct {
	pattern     string
	taskHandler tasks.AnyTask
	hasHandler  bool
}

// NestedRouter stores nested route patterns and optional task handlers.
type NestedRouter struct {
	mu             sync.RWMutex
	matcher        *matcher.Matcher
	routes         map[string]AnyNestedRoute
	compiledRoutes atomic.Value // []compiledRoute
	routeIndexMap  atomic.Value // map[string]int
	version        uint64       // Version counter for atomic updates
}

// AllRoutes returns a snapshot copy of registered nested routes.
func (nr *NestedRouter) AllRoutes() map[string]AnyNestedRoute {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	allRoutesCopy := make(map[string]AnyNestedRoute, len(nr.routes))
	maps.Copy(allRoutesCopy, nr.routes)
	return allRoutesCopy
}

// IsRegistered reports whether a pattern is registered.
func (nr *NestedRouter) IsRegistered(originalPattern string) bool {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	_, exists := nr.routes[originalPattern]
	return exists
}

// HasTaskHandler reports whether a pattern is registered with a handler.
func (nr *NestedRouter) HasTaskHandler(originalPattern string) bool {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	route, exists := nr.routes[originalPattern]
	if !exists {
		return false
	}
	return route.getTaskHandler() != nil
}

// ExplicitIndexSegmentIdentifier returns the explicit index marker string.
func (nr *NestedRouter) ExplicitIndexSegmentIdentifier() string {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher.ExplicitIndexSegmentIdentifier()
}

// DynamicParamPrefix returns the dynamic param prefix rune.
func (nr *NestedRouter) DynamicParamPrefix() rune {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher.DynamicParamPrefix()
}

// SplatSegmentIdentifier returns the splat segment identifier rune.
func (nr *NestedRouter) SplatSegmentIdentifier() rune {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.matcher.SplatSegmentIdentifier()
}

// Matcher returns a copy of the nested matcher state.
func (nr *NestedRouter) Matcher() *matcher.Matcher {
	nr.mu.RLock()
	defer nr.mu.RUnlock()

	matcherCopy := matcher.New(&matcher.Options{
		DynamicParamPrefix:             nr.matcher.DynamicParamPrefix(),
		SplatSegmentIdentifier:         nr.matcher.SplatSegmentIdentifier(),
		ExplicitIndexSegmentIdentifier: nr.matcher.ExplicitIndexSegmentIdentifier(),
		Quiet:                          true,
	})
	for pattern := range nr.routes {
		matcherCopy.RegisterPattern(pattern)
	}
	return matcherCopy
}

// NestedOptions configures pattern matching behavior for NestedRouter.
type NestedOptions struct {
	DynamicParamPrefix             rune
	SplatSegmentIdentifier         rune
	ExplicitIndexSegmentIdentifier string
}

// NewNestedRouter creates a nested router with the provided options.
func NewNestedRouter(opts *NestedOptions) *NestedRouter {
	matcherOpts := new(matcher.Options)
	if opts == nil {
		opts = new(NestedOptions)
	}
	matcherOpts.DynamicParamPrefix = genericsutil.OrDefault(
		opts.DynamicParamPrefix,
		':',
	)
	matcherOpts.SplatSegmentIdentifier = genericsutil.OrDefault(
		opts.SplatSegmentIdentifier,
		'*',
	)
	matcherOpts.ExplicitIndexSegmentIdentifier = genericsutil.OrDefault(
		opts.ExplicitIndexSegmentIdentifier,
		"",
	)
	nr := &NestedRouter{
		matcher: matcher.New(matcherOpts),
		routes:  make(map[string]AnyNestedRoute),
	}
	// Initialize atomic values
	nr.compiledRoutes.Store(make([]compiledRoute, 0))
	nr.routeIndexMap.Store(make(map[string]int))
	return nr
}

// NestedRoute stores metadata and optional task handler for one nested pattern.
type NestedRoute[O any] struct {
	genericsutil.ZeroHelper[None, O]
	router          *NestedRouter
	originalPattern string
	taskHandler     tasks.AnyTask
}

// AnyNestedRoute is the internal polymorphic route contract for NestedRouter.
type AnyNestedRoute interface {
	OriginalPattern() string
	genericsutil.AnyZeroHelper
	getTaskHandler() tasks.AnyTask
}

// OriginalPattern returns the pattern as registered.
func (route *NestedRoute[O]) OriginalPattern() string {
	return route.originalPattern
}

func (route *NestedRoute[O]) getTaskHandler() tasks.AnyTask {
	return route.taskHandler
}

// AddNestedTaskHandler registers a nested pattern with a task handler.
func AddNestedTaskHandler[O any](
	router *NestedRouter, pattern string, taskHandler *TaskHandler[None, O],
) *NestedRoute[O] {
	route := &NestedRoute[O]{
		router:          router,
		originalPattern: pattern,
		taskHandler:     taskHandler,
	}
	mustRegisterNestedRoute(route)
	return route
}

// AddNestedPatternWithoutHandler registers a nested pattern with no task handler.
func AddNestedPatternWithoutHandler(router *NestedRouter, pattern string) {
	route := &NestedRoute[None]{
		router:          router,
		originalPattern: pattern,
		taskHandler:     nil,
	}
	mustRegisterNestedRoute(route)
}

// AddNestedPatternWithoutHandlerIfMissing registers a pattern without a task
// handler only when it is not already present. It returns true if a new route
// was registered.
func (nr *NestedRouter) AddNestedPatternWithoutHandlerIfMissing(
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

	route := &NestedRoute[None]{
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

// NestedTasksResult stores task execution output for one matched pattern.
type NestedTasksResult struct {
	pattern string
	data    any
	err     error
	ranTask bool
}

// Pattern returns the matched route pattern.
func (ntr *NestedTasksResult) Pattern() string { return ntr.pattern }

// OK reports whether task execution succeeded.
func (ntr *NestedTasksResult) OK() bool { return ntr.err == nil }

// Data returns task output data.
func (ntr *NestedTasksResult) Data() any { return ntr.data }

// Err returns task execution error.
func (ntr *NestedTasksResult) Err() error { return ntr.err }

// RanTask reports whether this match had a task handler.
func (ntr *NestedTasksResult) RanTask() bool { return ntr.ranTask }

// NestedTasksResults contains ordered and keyed task results for nested matches.
type NestedTasksResults struct {
	Params          Params
	SplatValues     []string
	Map             map[string]*NestedTasksResult
	Slice           []*NestedTasksResult
	ResponseProxies []*response.Proxy
}

// HasTaskHandlerAt reports whether the result at i ran a task handler.
func (ntr *NestedTasksResults) HasTaskHandlerAt(i int) bool {
	if i < 0 || i >= len(ntr.Slice) {
		return false
	}
	return ntr.Slice[i].ranTask
}

// FindNestedMatches resolves nested route matches for the request path.
func FindNestedMatches(
	nestedRouter *NestedRouter,
	r *http.Request,
) (*matcher.FindNestedMatchesResults, bool) {
	nestedRouter.mu.RLock()
	defer nestedRouter.mu.RUnlock()
	return nestedRouter.matcher.FindNestedMatches(r.URL.Path)
}

// FindNestedMatchesAndRunTasks finds nested matches and runs all matched task handlers.
func FindNestedMatchesAndRunTasks(
	nestedRouter *NestedRouter,
	r *http.Request,
) (*NestedTasksResults, bool) {
	findResults, ok := FindNestedMatches(nestedRouter, r)
	if !ok {
		return nil, false
	}
	return RunNestedTasks(nestedRouter, r, findResults), true
}

// RunNestedTasks executes all task handlers for the matched routes in parallel.
//
// IMPORTANT: This function uses object pooling for ReqData objects. The safety of this
// depends on tasksCtx.RunParallel blocking until all tasks complete. If RunParallel
// were to return before tasks finish (async dispatch), this would cause use-after-free bugs.
// The current tasks.Ctx implementation blocks until completion, making this safe.
func RunNestedTasks(
	nestedRouter *NestedRouter,
	r *http.Request,
	findNestedMatchesResults *matcher.FindNestedMatchesResults,
) *NestedTasksResults {
	tasksCtx := GetTasksCtx(r)
	if tasksCtx == nil {
		muxLog.Error("No TasksCtx found in request for RunNestedTasks")
		return nil
	}

	matches := findNestedMatchesResults.Matches
	numMatches := len(matches)
	if numMatches == 0 {
		return nil
	}

	// Create results structure with pre-allocated capacity
	results := &NestedTasksResults{
		Params:          findNestedMatchesResults.Params,
		SplatValues:     findNestedMatchesResults.SplatValues,
		Map:             make(map[string]*NestedTasksResult, numMatches),
		Slice:           make([]*NestedTasksResult, numMatches),
		ResponseProxies: make([]*response.Proxy, numMatches),
	}

	// Get compiled routes atomically
	compiledRoutes := nestedRouter.compiledRoutes.Load().([]compiledRoute)
	routeIndexMap := nestedRouter.routeIndexMap.Load().(map[string]int)

	// Pre-allocate boundTasks based on estimated task count
	boundTasks := make(
		[]*optimizedBoundTask,
		0,
		numMatches/2,
	) // Assume ~50% have handlers
	taskCancels := make([]context.CancelFunc, 0, numMatches/2)

	// Track pooled objects for cleanup
	pooledReqData := make([]*ReqData[None], 0, numMatches/2)

	// Ensure cleanup happens after RunParallel completes (which blocks until all tasks finish)
	defer func() {
		// Return ReqData objects to pool after clearing
		for _, rd := range pooledReqData {
			rd.params = nil
			rd.splatVals = nil
			rd.tasksCtx = nil
			rd.req = nil
			rd.responseProxy = nil
			rd.input = noneInstance
			reqDataPool.Put(rd)
		}
	}()

	// Single pass with optimized lookup
	for i, match := range matches {
		pattern := match.OriginalPattern()

		// Get pooled result
		result := &NestedTasksResult{}
		result.pattern = pattern
		result.data = nil
		result.err = nil
		result.ranTask = false

		results.Map[pattern] = result
		results.Slice[i] = result

		// Fast lookup using pre-computed index
		idx, exists := routeIndexMap[pattern]
		if !exists || idx >= len(compiledRoutes) {
			results.ResponseProxies[i] = response.NewProxy()
			continue
		}

		compiled := &compiledRoutes[idx]
		if !compiled.hasHandler {
			results.ResponseProxies[i] = response.NewProxy()
			continue
		}

		result.ranTask = true

		// Create response proxy
		proxy := response.NewProxy()
		results.ResponseProxies[i] = proxy

		// Get pooled ReqData and fully initialize it
		reqData := reqDataPool.Get().(*ReqData[None])
		reqData.params = results.Params
		reqData.splatVals = results.SplatValues
		reqData.input = noneInstance
		reqData.req = r
		reqData.responseProxy = proxy
		pooledReqData = append(pooledReqData, reqData)
		reqData.tasksCtx = nil

		boundTask := &optimizedBoundTask{
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
			boundTasks[0].reqData.tasksCtx = tasksCtx
		} else {
			currentCtx := tasksCtx
			for i, bt := range boundTasks {
				if i < len(boundTasks)-1 {
					childNativeCtx, cancel := context.WithCancel(currentCtx.NativeContext())
					taskCancels = append(taskCancels, cancel)
					childCtx := currentCtx.WithNativeContext(childNativeCtx)
					bt.reqData.tasksCtx = childCtx
					bt.cancelDescendants = cancel
					currentCtx = childCtx
					continue
				}
				// Leaf task reuses the current ancestor-cancelable context.
				bt.reqData.tasksCtx = currentCtx
			}
		}
		runNestedBoundTasks(boundTasks)
	}

	for _, cancel := range taskCancels {
		cancel()
	}

	return results
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE API
/////////////////////////////////////////////////////////////////////

func mustRegisterNestedRoute[O any](route *NestedRoute[O]) {
	route.router.mu.Lock()
	defer route.router.mu.Unlock()

	if _, exists := route.router.routes[route.originalPattern]; exists {
		panic(
			fmt.Sprintf(
				"Pattern '%s' is already registered in NestedRouter. Perhaps you're unintentionally registering it twice?",
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
func (nr *NestedRouter) addCompiledRoute(compiled compiledRoute) {
	// Get current state
	currentRoutes := nr.compiledRoutes.Load().([]compiledRoute)
	currentIndexMap := nr.routeIndexMap.Load().(map[string]int)

	// Create new slices/maps
	newRoutes := make([]compiledRoute, len(currentRoutes)+1)
	copy(newRoutes, currentRoutes)
	newRoutes[len(currentRoutes)] = compiled

	newIndexMap := make(map[string]int, len(currentIndexMap)+1)
	maps.Copy(newIndexMap, currentIndexMap)
	newIndexMap[compiled.pattern] = len(currentRoutes)

	// Atomic update
	nr.compiledRoutes.Store(newRoutes)
	nr.routeIndexMap.Store(newIndexMap)
	atomic.AddUint64(&nr.version, 1)
}

type optimizedBoundTask struct {
	taskHandler tasks.AnyTask
	reqData     *ReqData[None]
	result      *NestedTasksResult
	// Called when this task fails; should cancel deeper matched routes only.
	cancelDescendants context.CancelFunc
}

func (oc *optimizedBoundTask) Run() error {
	data, err := oc.taskHandler.RunWithAnyInput(oc.reqData.tasksCtx, oc.reqData)
	oc.result.data = data
	oc.result.err = err
	if err != nil && oc.cancelDescendants != nil {
		oc.cancelDescendants()
	}
	return err
}

func runNestedBoundTasks(boundTasks []*optimizedBoundTask) {
	switch len(boundTasks) {
	case 0:
		return
	case 1:
		_ = boundTasks[0].Run()
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(boundTasks))
	for _, bt := range boundTasks {
		bt := bt
		go func() {
			defer wg.Done()
			_ = bt.Run()
		}()
	}
	wg.Wait()
}

// ReplaceRoutes atomically replaces all routes with a new set.
// This rebuilds the matcher from scratch, which is safe and fast for dev mode.
func (nr *NestedRouter) ReplaceRoutes(newRoutes map[string]AnyNestedRoute) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	nr.replaceRoutesLocked(newRoutes)
}

// RebuildPreservingHandlers atomically rebuilds the router, preserving routes
// that have task handlers and replacing handler-less routes with the provided patterns.
// This is intended for dev-time fast rebuilds when only pattern definitions change.
func (nr *NestedRouter) RebuildPreservingHandlers(patterns []string) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	newRoutes := make(map[string]AnyNestedRoute)

	// Preserve existing routes with handlers
	for pattern, route := range nr.routes {
		if route.getTaskHandler() != nil {
			newRoutes[pattern] = route
		}
	}

	// Add patterns without handlers
	for _, pattern := range patterns {
		if _, exists := newRoutes[pattern]; !exists {
			newRoutes[pattern] = &NestedRoute[None]{
				router:          nr,
				originalPattern: pattern,
				taskHandler:     nil,
			}
		}
	}

	nr.replaceRoutesLocked(newRoutes)
}

// replaceRoutesLocked is the internal implementation. Caller must hold nr.mu.Lock().
func (nr *NestedRouter) replaceRoutesLocked(
	newRoutes map[string]AnyNestedRoute,
) {
	routesCopy := make(map[string]AnyNestedRoute, len(newRoutes))
	maps.Copy(routesCopy, newRoutes)

	opts := &matcher.Options{
		DynamicParamPrefix:             nr.matcher.DynamicParamPrefix(),
		SplatSegmentIdentifier:         nr.matcher.SplatSegmentIdentifier(),
		ExplicitIndexSegmentIdentifier: nr.matcher.ExplicitIndexSegmentIdentifier(),
		Quiet:                          true,
	}

	newMatcher := matcher.New(opts)

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
	nr.compiledRoutes.Store(newCompiled)
	nr.routeIndexMap.Store(newIndexMap)
	atomic.AddUint64(&nr.version, 1)
}
