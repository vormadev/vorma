package mux

import (
	"fmt"
	"maps"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/vormadev/vorma/kit/genericsutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/opt"
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

type NestedRouter struct {
	matcher        *matcher.Matcher
	routes         map[string]AnyNestedRoute
	compiledRoutes atomic.Value // []compiledRoute
	routeIndexMap  atomic.Value // map[string]int
	version        uint64       // Version counter for atomic updates
	mu             sync.RWMutex
}

func (nr *NestedRouter) AllRoutes() map[string]AnyNestedRoute {
	return nr.routes
}

func (nr *NestedRouter) IsRegistered(originalPattern string) bool {
	_, exists := nr.routes[originalPattern]
	return exists
}

func (nr *NestedRouter) HasTaskHandler(originalPattern string) bool {
	route, exists := nr.routes[originalPattern]
	if !exists {
		return false
	}
	return route.getTaskHandler() != nil
}

func (nr *NestedRouter) GetExplicitIndexSegment() string {
	return nr.matcher.GetExplicitIndexSegment()
}

func (nr *NestedRouter) GetDynamicParamPrefixRune() rune {
	return nr.matcher.GetDynamicParamPrefixRune()
}

func (nr *NestedRouter) GetSplatSegmentRune() rune {
	return nr.matcher.GetSplatSegmentRune()
}

func (nr *NestedRouter) GetMatcher() *matcher.Matcher {
	return nr.matcher
}

type NestedOptions struct {
	DynamicParamPrefixRune rune
	SplatSegmentRune       rune
	ExplicitIndexSegment   string
}

func NewNestedRouter(opts *NestedOptions) *NestedRouter {
	matcherOpts := new(matcher.Options)
	if opts == nil {
		opts = new(NestedOptions)
	}
	matcherOpts.DynamicParamPrefixRune = opt.Resolve(opts, opts.DynamicParamPrefixRune, ':')
	matcherOpts.SplatSegmentRune = opt.Resolve(opts, opts.SplatSegmentRune, '*')
	matcherOpts.ExplicitIndexSegment = opt.Resolve(opts, opts.ExplicitIndexSegment, "")
	nr := &NestedRouter{
		matcher: matcher.New(matcherOpts),
		routes:  make(map[string]AnyNestedRoute),
	}
	// Initialize atomic values
	nr.compiledRoutes.Store(make([]compiledRoute, 0))
	nr.routeIndexMap.Store(make(map[string]int))
	return nr
}

type NestedRoute[O any] struct {
	genericsutil.ZeroHelper[None, O]
	router          *NestedRouter
	originalPattern string
	taskHandler     tasks.AnyTask
}

type AnyNestedRoute interface {
	OriginalPattern() string
	genericsutil.AnyZeroHelper
	getTaskHandler() tasks.AnyTask
}

func (route *NestedRoute[O]) OriginalPattern() string {
	return route.originalPattern
}

func (route *NestedRoute[O]) getTaskHandler() tasks.AnyTask {
	return route.taskHandler
}

func RegisterNestedTaskHandler[O any](
	router *NestedRouter, pattern string, taskHandler *TaskHandler[None, O],
) *NestedRoute[O] {
	route := &NestedRoute[O]{
		router:          router,
		originalPattern: pattern,
		taskHandler:     taskHandler,
	}
	mustRegisterNestedRoute(route)
	// Pre-compile
	router.mu.Lock()
	compiled := compiledRoute{
		pattern:     pattern,
		taskHandler: taskHandler,
		hasHandler:  true,
	}
	router.addCompiledRoute(compiled)
	router.mu.Unlock()
	return route
}

func RegisterNestedPatternWithoutHandler(router *NestedRouter, pattern string) {
	route := &NestedRoute[None]{
		router:          router,
		originalPattern: pattern,
		taskHandler:     nil,
	}
	mustRegisterNestedRoute(route)
	// Pre-compile
	router.mu.Lock()
	compiled := compiledRoute{
		pattern:     pattern,
		taskHandler: nil,
		hasHandler:  false,
	}
	router.addCompiledRoute(compiled)
	router.mu.Unlock()
}

type NestedTasksResult struct {
	pattern string
	data    any
	err     error
	ranTask bool
}

func (ntr *NestedTasksResult) Pattern() string { return ntr.pattern }
func (ntr *NestedTasksResult) OK() bool        { return ntr.err == nil }
func (ntr *NestedTasksResult) Data() any       { return ntr.data }
func (ntr *NestedTasksResult) Err() error      { return ntr.err }
func (ntr *NestedTasksResult) RanTask() bool   { return ntr.ranTask }

type NestedTasksResults struct {
	Params          Params
	SplatValues     []string
	Map             map[string]*NestedTasksResult
	Slice           []*NestedTasksResult
	ResponseProxies []*response.Proxy
}

func (ntr *NestedTasksResults) GetHasTaskHandler(i int) bool {
	if i < 0 || i >= len(ntr.Slice) {
		return false
	}
	return ntr.Slice[i].ranTask
}

func FindNestedMatches(nestedRouter *NestedRouter, r *http.Request) (*matcher.FindNestedMatchesResults, bool) {
	return nestedRouter.matcher.FindNestedMatches(r.URL.Path)
}

func FindNestedMatchesAndRunTasks(nestedRouter *NestedRouter, r *http.Request) (*NestedTasksResults, bool) {
	findResults, ok := FindNestedMatches(nestedRouter, r)
	if !ok {
		return nil, false
	}
	return RunNestedTasks(nestedRouter, r, findResults), true
}

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
	boundTasks := make([]tasks.BoundTask, 0, numMatches/2) // Assume ~50% have handlers

	// Track pooled objects for cleanup
	pooledReqData := make([]*ReqData[None], 0, numMatches/2)

	// Ensure cleanup happens
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
		reqData.tasksCtx = tasksCtx
		reqData.input = noneInstance
		reqData.req = r
		reqData.responseProxy = proxy
		pooledReqData = append(pooledReqData, reqData)

		boundTask := &optimizedBoundTask{
			taskHandler: compiled.taskHandler,
			reqData:     reqData,
			result:      result,
		}
		boundTasks = append(boundTasks, boundTask)
	}

	// Execute all tasks in parallel if we have any
	if len(boundTasks) > 0 {
		if err := tasksCtx.RunParallel(boundTasks...); err != nil {
			muxLog.Error("tasks.Go reported an error during nested task execution", "error", err)
		}
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
		panic(fmt.Sprintf("Pattern '%s' is already registered in NestedRouter. Perhaps you're unintentionally registering it twice?", route.originalPattern))
	}
	route.router.matcher.RegisterPattern(route.originalPattern)
	route.router.routes[route.originalPattern] = route
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
}

func (oc *optimizedBoundTask) Run(ctx *tasks.Ctx) error {
	data, err := oc.taskHandler.RunWithAnyInput(ctx, oc.reqData)
	oc.result.data = data
	oc.result.err = err
	return err
}
