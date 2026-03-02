// Package runtimehttp owns HTTP router construction, action-input parsing, and
// stage-one route execution orchestration for vormaruntime.
//
// Keeping these concerns isolated makes runtime HTTP behavior testable without
// the full mutable runtime state machine.
package runtimehttp

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/vormadev/vorma/internal/vormaruntime/rendering"
	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmatcher"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/validate"
)

// LoadersRouterSpec configures nested loader-router pattern semantics.
type LoadersRouterSpec struct {
	DynamicParamPrefix             rune
	SplatSegmentIdentifier         rune
	ExplicitIndexSegmentIdentifier string
}

// ActionsRouterSpec configures actions-router matching and method policy.
type ActionsRouterSpec struct {
	DynamicParamPrefix     rune
	SplatSegmentIdentifier rune
	MountRoot              string
	SupportedMethods       []string
	IsFormDataInput        func(any) bool
}

// LoadersRouter wraps nestedmux.Router for loader route registration.
type LoadersRouter struct {
	NestedRouter *nestedmux.Router
}

// ActionsRouter wraps mux.Router for action route registration.
type ActionsRouter struct {
	*mux.Router
	supportedMethods map[string]bool
}

// EnsureLoaderPatternsRegisteredInput configures loader-pattern registration
// for the nested router used by loaders handlers.
type EnsureLoaderPatternsRegisteredInput struct {
	NestedRouter *nestedmux.Router
	Paths        map[string]*runtimecore.RoutePath
}

// NewLoadersRouter constructs a LoadersRouter from one spec.
func NewLoadersRouter(spec LoadersRouterSpec) *LoadersRouter {
	return &LoadersRouter{
		NestedRouter: BuildLoadersNestedRouter(spec),
	}
}

// EnsureLoaderPatternsRegistered validates path metadata and ensures each
// pattern is present in the nested router before loaders handler execution.
func EnsureLoaderPatternsRegistered(
	input EnsureLoaderPatternsRegisteredInput,
) {
	if input.NestedRouter == nil {
		panic("nestedRouter is nil")
	}
	for mapKeyPattern, pathEntry := range input.Paths {
		if pathEntry == nil {
			panic(
				fmt.Sprintf(
					"paths entry for pattern %q is nil",
					mapKeyPattern,
				),
			)
		}
		input.NestedRouter.AddPatternWithoutHandlerIfMissing(
			pathEntry.OriginalPattern,
		)
	}
}

// NewActionsRouter constructs an ActionsRouter from one spec.
func NewActionsRouter(spec ActionsRouterSpec) *ActionsRouter {
	router, supportedMethods := BuildActionsRouter(spec)
	return &ActionsRouter{
		Router:           router,
		supportedMethods: supportedMethods,
	}
}

// SupportedMethodsClone returns a defensive copy of supported methods.
func (actionsRouter *ActionsRouter) SupportedMethodsClone() map[string]bool {
	if actionsRouter == nil || actionsRouter.supportedMethods == nil {
		return nil
	}
	clone := make(map[string]bool, len(actionsRouter.supportedMethods))
	maps.Copy(clone, actionsRouter.supportedMethods)
	return clone
}

// BuildLoadersNestedRouter constructs the nested loaders router.
func BuildLoadersNestedRouter(spec LoadersRouterSpec) *nestedmux.Router {
	explicitIndexSegment := spec.ExplicitIndexSegmentIdentifier
	if explicitIndexSegment == "" {
		explicitIndexSegment = "_index"
	}
	return nestedmux.NewRouter(&nestedmux.Options{
		DynamicParamPrefix:             spec.DynamicParamPrefix,
		SplatSegmentIdentifier:         spec.SplatSegmentIdentifier,
		ExplicitIndexSegmentIdentifier: explicitIndexSegment,
	})
}

// BuildSupportedMethodsMap normalizes supported method names.
func BuildSupportedMethodsMap(supportedMethods []string) map[string]bool {
	out := make(map[string]bool, len(supportedMethods))
	if len(supportedMethods) == 0 {
		out["GET"] = true
		out["POST"] = true
		out["PUT"] = true
		out["DELETE"] = true
		out["PATCH"] = true
		return out
	}
	for _, method := range supportedMethods {
		upperMethod := strings.ToUpper(strings.TrimSpace(method))
		if upperMethod == "" {
			continue
		}
		out[upperMethod] = true
	}
	return out
}

// ValidateExpectedBuildIDOrWriteConflictInput defines the inputs for
// ValidateExpectedBuildIDOrWriteConflict.
type ValidateExpectedBuildIDOrWriteConflictInput struct {
	ResponseWriter            http.ResponseWriter
	Request                   *http.Request
	CurrentBuildID            string
	ExpectedBuildIDHeaderName string
	Log                       *slog.Logger
}

// ValidateExpectedBuildIDOrWriteConflict validates the dev-reload expected
// build id header and writes a conflict response when the requested build id
// does not match current runtime state.
func ValidateExpectedBuildIDOrWriteConflict(
	input ValidateExpectedBuildIDOrWriteConflictInput,
) bool {
	if input.Request == nil {
		return true
	}

	requestExpectedBuildID := strings.TrimSpace(
		input.Request.Header.Get(input.ExpectedBuildIDHeaderName),
	)
	if requestExpectedBuildID == "" {
		return true
	}

	currentBuildID := strings.TrimSpace(input.CurrentBuildID)
	if requestExpectedBuildID == currentBuildID {
		return true
	}

	if input.Log != nil {
		input.Log.Warn(
			"dev reload endpoint rejected request due to expected build id mismatch",
			"request_expected_build_id",
			requestExpectedBuildID,
			"current_build_id",
			currentBuildID,
		)
	}

	if input.ResponseWriter != nil {
		http.Error(
			input.ResponseWriter,
			"expected build id does not match current build id",
			http.StatusConflict,
		)
	}
	return false
}

// DevReloadActionEndpointsInput defines the inputs for
// HandleDevReloadActionEndpoints.
type DevReloadActionEndpointsInput struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	IsDevMode      bool

	RoutesEndpointPath   string
	TemplateEndpointPath string

	ValidateExpectedBuildIDOrWriteConflict func(
		responseWriter http.ResponseWriter,
		request *http.Request,
	) bool
	ReloadRoutesFromDisk   func() error
	ReloadTemplateFromDisk func() error
	Log                    *slog.Logger
}

// HandleDevReloadActionEndpoints dispatches dev-reload action endpoints and
// writes method/validation/error responses when needed.
func HandleDevReloadActionEndpoints(
	input DevReloadActionEndpointsInput,
) bool {
	if !input.IsDevMode || input.Request == nil {
		return false
	}

	switch input.Request.URL.Path {
	case input.RoutesEndpointPath:
		return handleSingleDevReloadActionEndpoint(
			handleSingleDevReloadActionEndpointInput{
				ResponseWriter:                         input.ResponseWriter,
				Request:                                input.Request,
				ValidateExpectedBuildIDOrWriteConflict: input.ValidateExpectedBuildIDOrWriteConflict,
				ReloadFromDisk:                         input.ReloadRoutesFromDisk,
				LogErrorPrefix:                         "route reload failed",
				Log:                                    input.Log,
			},
		)
	case input.TemplateEndpointPath:
		return handleSingleDevReloadActionEndpoint(
			handleSingleDevReloadActionEndpointInput{
				ResponseWriter:                         input.ResponseWriter,
				Request:                                input.Request,
				ValidateExpectedBuildIDOrWriteConflict: input.ValidateExpectedBuildIDOrWriteConflict,
				ReloadFromDisk:                         input.ReloadTemplateFromDisk,
				LogErrorPrefix:                         "template reload failed",
				Log:                                    input.Log,
			},
		)
	default:
		return false
	}
}

type handleSingleDevReloadActionEndpointInput struct {
	ResponseWriter                         http.ResponseWriter
	Request                                *http.Request
	ValidateExpectedBuildIDOrWriteConflict func(
		responseWriter http.ResponseWriter,
		request *http.Request,
	) bool
	ReloadFromDisk func() error
	LogErrorPrefix string
	Log            *slog.Logger
}

func handleSingleDevReloadActionEndpoint(
	input handleSingleDevReloadActionEndpointInput,
) bool {
	if input.Request.Method != http.MethodPost {
		if input.ResponseWriter != nil {
			input.ResponseWriter.Header().Set("Allow", http.MethodPost)
			http.Error(
				input.ResponseWriter,
				"method not allowed",
				http.StatusMethodNotAllowed,
			)
		}
		return true
	}

	if input.ValidateExpectedBuildIDOrWriteConflict != nil &&
		!input.ValidateExpectedBuildIDOrWriteConflict(
			input.ResponseWriter,
			input.Request,
		) {
		return true
	}

	if input.ReloadFromDisk == nil {
		if input.Log != nil {
			input.Log.Error(
				input.LogErrorPrefix,
				"error",
				"reload operation is nil",
			)
		}
		if input.ResponseWriter != nil {
			http.Error(
				input.ResponseWriter,
				"reload operation is not configured",
				http.StatusInternalServerError,
			)
		}
		return true
	}

	if err := input.ReloadFromDisk(); err != nil {
		if input.Log != nil {
			input.Log.Error(input.LogErrorPrefix, "error", err.Error())
		}
		if input.ResponseWriter != nil {
			http.Error(
				input.ResponseWriter,
				err.Error(),
				http.StatusInternalServerError,
			)
		}
		return true
	}

	if input.ResponseWriter != nil {
		_, _ = input.ResponseWriter.Write([]byte("ok"))
	}
	return true
}

// ParseActionInput applies the runtime action-input parse policy for one
// request/input pair.
func ParseActionInput(
	request *http.Request,
	inputPtr any,
	supportedMethods map[string]bool,
	isFormDataInput func(any) bool,
) error {
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		return validate.URLSearchParamsInto(request, inputPtr)
	}

	if !supportedMethods[request.Method] {
		return &validate.ValidationError{
			Err: errors.New("unsupported method"),
		}
	}

	contentType, _, _ := mime.ParseMediaType(
		request.Header.Get("Content-Type"),
	)
	if contentType == "application/x-www-form-urlencoded" ||
		contentType == "multipart/form-data" {
		if isFormDataInput != nil && isFormDataInput(inputPtr) {
			return nil
		}
		return &validate.ValidationError{
			Err: errors.New(
				"form content type requires vormaruntime.FormData input",
			),
		}
	}

	return validate.JSONBodyInto(request, inputPtr)
}

// BuildActionsRouter constructs the actions mux router and its supported-method
// lookup map.
func BuildActionsRouter(spec ActionsRouterSpec) (*mux.Router, map[string]bool) {
	mountRoot := spec.MountRoot
	if mountRoot == "" {
		mountRoot = "/api/"
	}
	supportedMethods := BuildSupportedMethodsMap(spec.SupportedMethods)

	actionsRouter := mux.NewRouter(&mux.Options{
		DynamicParamPrefix:     spec.DynamicParamPrefix,
		SplatSegmentIdentifier: spec.SplatSegmentIdentifier,
		MountRoot:              mountRoot,
		ParseInput: func(request *http.Request, inputPtr any) error {
			return ParseActionInput(
				request,
				inputPtr,
				supportedMethods,
				spec.IsFormDataInput,
			)
		},
	})
	return actionsRouter, supportedMethods
}

// PrepareExecutionInputsInput is the input contract for PrepareExecutionInputs.
type PrepareExecutionInputsInput struct {
	Request                  *http.Request
	NestedRouter             *nestedmux.Router
	RuntimeSnapshot          routepipeline.RuntimeSnapshot
	IsSnapshotVersionCurrent func(expectedSnapshotVersion uint64) bool
}

// PrepareExecutionInputs finds nested matches and resolves the stage-one
// route-data execution inputs for one request.
func PrepareExecutionInputs(
	input PrepareExecutionInputsInput,
) (routepipeline.RouteDataExecutionInputs, bool) {
	matchResults, found := nestedmux.FindMatches(
		input.NestedRouter,
		input.Request,
	)
	if !found {
		return routepipeline.RouteDataExecutionInputs{
			RuntimeSnapshot: input.RuntimeSnapshot,
		}, false
	}

	return BuildExecutionInputsFromMatchResults(
		BuildExecutionInputsFromMatchResultsInput{
			MatchResults:             matchResults,
			RuntimeSnapshot:          input.RuntimeSnapshot,
			IsSnapshotVersionCurrent: input.IsSnapshotVersionCurrent,
		},
	), true
}

// BuildExecutionInputsFromMatchResultsInput is the input contract for
// BuildExecutionInputsFromMatchResults.
type BuildExecutionInputsFromMatchResultsInput struct {
	MatchResults             *nestedmatcher.Results
	RuntimeSnapshot          routepipeline.RuntimeSnapshot
	IsSnapshotVersionCurrent func(expectedSnapshotVersion uint64) bool
}

// BuildExecutionInputsFromMatchResults resolves stage-one execution inputs from
// already-computed nested match results and one runtime snapshot.
func BuildExecutionInputsFromMatchResults(
	input BuildExecutionInputsFromMatchResultsInput,
) routepipeline.RouteDataExecutionInputs {
	if input.MatchResults == nil {
		return routepipeline.RouteDataExecutionInputs{
			RuntimeSnapshot: input.RuntimeSnapshot,
		}
	}

	matchResults := input.MatchResults
	matches := matchResults.Matches
	cacheKey := routepipeline.BuildRouteDataCacheKey(
		matches,
		input.RuntimeSnapshot.IsDev,
		input.RuntimeSnapshot.BuildID,
		input.RuntimeSnapshot.RouteDataSnapshotVersion,
	)
	cached := routepipeline.LoadOrBuildCachedItemSubset(
		cacheKey,
		matches,
		input.RuntimeSnapshot.Paths,
		input.RuntimeSnapshot.ClientEntryDeps,
		input.RuntimeSnapshot.IsDev,
		input.RuntimeSnapshot.RouteDataSnapshotVersion,
		input.RuntimeSnapshot.RouteDataCache,
		input.IsSnapshotVersionCurrent,
	)
	matchedPatterns := cached.MatchedPatterns
	if len(matchedPatterns) != len(matches) {
		matchedPatterns = routepipeline.CollectMatchedPatterns(matches)
	}

	return routepipeline.RouteDataExecutionInputs{
		MatchResults:    matchResults,
		Matches:         matches,
		MatchedPatterns: matchedPatterns,
		Cached:          cached,
		RuntimeSnapshot: input.RuntimeSnapshot,
	}
}

// PlanRouteResultFromTaskResultsInput is the input contract for
// PlanRouteResultFromTaskResults.
type PlanRouteResultFromTaskResultsInput struct {
	ExecutionInputs                 routepipeline.RouteDataExecutionInputs
	TasksResults                    *nestedmux.TasksResults
	WarnNilLoaderData               func(pattern string)
	ResolveClientLoaderErrorMessage func(err error, pattern string) string
}

// PlanRouteResultFromTaskResults plans stage-one route results from loader task
// outputs and merged response state.
func PlanRouteResultFromTaskResults(
	input PlanRouteResultFromTaskResultsInput,
) *routepipeline.RouteResult {
	mergedResponseProxy := response.MergeProxyResponses(
		input.TasksResults.ResponseProxies...,
	)
	hasRootData := routepipeline.ComputeHasRootData(
		input.ExecutionInputs.MatchResults,
		input.TasksResults,
	)

	loadersData, loadersErrs := collectLoadersDataAndErrors(
		input.TasksResults,
		input.ExecutionInputs.MatchedPatterns,
		input.WarnNilLoaderData,
	)

	outermostErrorIdx := routepipeline.FindFirstErrorIndex(loadersErrs)
	clientLoaderErrorMessage := ""
	if outermostErrorIdx != nil &&
		input.ResolveClientLoaderErrorMessage != nil {
		derefErrorIdx := *outermostErrorIdx
		clientLoaderErrorMessage = input.ResolveClientLoaderErrorMessage(
			loadersErrs[derefErrorIdx],
			input.ExecutionInputs.MatchedPatterns[derefErrorIdx],
		)
	}

	return routepipeline.PlanRouteResultFromResolvedTaskOutcomes(
		routepipeline.RouteStageOnePlannerInput{
			MatchResults:              input.ExecutionInputs.MatchResults,
			Matches:                   input.ExecutionInputs.Matches,
			MatchedPatterns:           input.ExecutionInputs.MatchedPatterns,
			Cached:                    input.ExecutionInputs.Cached,
			RuntimeSnapshot:           input.ExecutionInputs.RuntimeSnapshot,
			HasRootData:               hasRootData,
			LoadersData:               loadersData,
			OutermostLoaderErrorIndex: outermostErrorIdx,
			ClientLoaderErrorMessage:  clientLoaderErrorMessage,
			ResponseProxies:           input.TasksResults.ResponseProxies,
			MergedResponseProxy:       mergedResponseProxy,
		},
	)
}

func collectLoadersDataAndErrors(
	tasksResults *nestedmux.TasksResults,
	matchedPatterns []string,
	warnNilLoaderData func(pattern string),
) ([]any, []error) {
	loadersData, loadersErrs := routepipeline.CollectLoadersDataAndErrors(
		tasksResults,
	)
	for i := range loadersData {
		result := tasksResults.Slice[i]
		if result.RanTask() && loadersErrs[i] == nil {
			isNilOrPointsToNil := reflectutil.
				ExcludingNoneGetIsNilOrUltimatelyPointsToNil(loadersData[i])
			if isNilOrPointsToNil && warnNilLoaderData != nil {
				pattern := ""
				if i < len(matchedPatterns) {
					pattern = matchedPatterns[i]
				}
				warnNilLoaderData(pattern)
			}
		}
	}
	return loadersData, loadersErrs
}

// RouteDataStageOneInput is the input contract for ExecuteRouteDataStageOne.
type RouteDataStageOneInput struct {
	ResponseWriter             http.ResponseWriter
	Request                    *http.Request
	NestedRouter               *nestedmux.Router
	RequestedBuildID           string
	BuildIDHeaderKey           string
	PrepareExecutionInputs     func(*http.Request, *nestedmux.Router) (routepipeline.RouteDataExecutionInputs, bool)
	ResolveClientLoaderMessage func(error, string) string
	Log                        *slog.Logger
}

// ExecuteRouteDataStageOne computes stage-one route results including stale
// build detection, route-match handling, and loader execution planning.
func ExecuteRouteDataStageOne(
	input RouteDataStageOneInput,
) *routepipeline.RouteResult {
	executionInputs, found := input.PrepareExecutionInputs(
		input.Request,
		input.NestedRouter,
	)
	if executionInputs.RuntimeSnapshot.BuildID != "" &&
		input.ResponseWriter != nil &&
		strings.TrimSpace(input.BuildIDHeaderKey) != "" {
		input.ResponseWriter.Header().Set(
			input.BuildIDHeaderKey,
			executionInputs.RuntimeSnapshot.BuildID,
		)
	}
	if strings.TrimSpace(input.RequestedBuildID) != "" &&
		input.RequestedBuildID != executionInputs.RuntimeSnapshot.BuildID {
		if input.Log != nil {
			input.Log.Debug(
				"Stale build loaders request",
				"path",
				input.Request.URL.Path,
				"requested_build_id",
				input.RequestedBuildID,
				"current_build_id",
				executionInputs.RuntimeSnapshot.BuildID,
				"route_data_snapshot_version",
				executionInputs.RuntimeSnapshot.RouteDataSnapshotVersion,
			)
		}
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateStaleBuild,
			BuildID:       executionInputs.RuntimeSnapshot.BuildID,
		}
	}
	if !found {
		if input.Log != nil {
			input.Log.Debug(
				"No route match for loaders request",
				"path",
				input.Request.URL.Path,
				"build_id",
				executionInputs.RuntimeSnapshot.BuildID,
				"route_data_snapshot_version",
				executionInputs.RuntimeSnapshot.RouteDataSnapshotVersion,
			)
		}
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateNotFound,
			BuildID:       executionInputs.RuntimeSnapshot.BuildID,
		}
	}

	tasksResults := nestedmux.RunTasksWithoutPatternMap(
		input.NestedRouter,
		input.Request,
		executionInputs.MatchResults,
	)
	if tasksResults == nil {
		if input.Log != nil {
			input.Log.Error(
				"Missing TasksCtx for loaders request. Use a mux.Router stack or wrap with mux.InjectTasksCtxMiddleware.",
			)
		}
		if input.ResponseWriter != nil {
			res := response.New(input.ResponseWriter)
			res.InternalServerError()
		}
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateError,
			BuildID:       executionInputs.RuntimeSnapshot.BuildID,
		}
	}

	return PlanRouteResultFromTaskResults(
		PlanRouteResultFromTaskResultsInput{
			ExecutionInputs: executionInputs,
			TasksResults:    tasksResults,
			WarnNilLoaderData: func(pattern string) {
				if input.Log == nil {
					return
				}
				input.Log.Warn(
					"Do not return nil values from loaders unless the underlying type is an empty struct or you are returning an error.",
					"pattern",
					pattern,
				)
			},
			ResolveClientLoaderErrorMessage: input.ResolveClientLoaderMessage,
		},
	)
}

// BuildUIRouteDataFromResolvedRouteInput is the input contract for
// BuildUIRouteDataFromResolvedRoute.
type BuildUIRouteDataFromResolvedRouteInput struct {
	ResponseWriter               http.ResponseWriter
	RouteResult                  *routepipeline.RouteResult
	IsJSON                       bool
	DefaultHeadElements          []*htmlutil.Element
	PublicPathPrefix             string
	ToSortedAndPreEscapedHeadEls func([]*htmlutil.Element) *headels.SortedAndPreEscapedHeadEls
	Log                          *slog.Logger
}

// BuildUIRouteDataFromResolvedRoute resolves assets for one non-terminal stage
// one route result.
func BuildUIRouteDataFromResolvedRoute(
	input BuildUIRouteDataFromResolvedRouteInput,
) *routepipeline.RouteResult {
	res := response.New(input.ResponseWriter)
	assets, err := routepipeline.BuildRouteAssets(
		routepipeline.BuildRouteAssetsInput{
			RouteResult:                    input.RouteResult,
			DefaultHeadElements:            input.DefaultHeadElements,
			IsJSON:                         input.IsJSON,
			PublicPathPrefix:               input.PublicPathPrefix,
			ToSortedAndPreEscapedHeadElsFn: input.ToSortedAndPreEscapedHeadEls,
		},
	)
	if err != nil {
		if input.Log != nil {
			input.Log.Error(
				"Error in getUIRouteData asset resolution",
				"error",
				err.Error(),
			)
		}
		res.InternalServerError()
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateError,
			BuildID:       input.RouteResult.BuildID,
		}
	}

	return &routepipeline.RouteResult{
		BuildID:                   input.RouteResult.BuildID,
		Core:                      input.RouteResult.Core,
		Assets:                    assets,
		HTMLRenderSnapshot:        input.RouteResult.HTMLRenderSnapshot,
		RouteManifestFileSnapshot: input.RouteResult.RouteManifestFileSnapshot,
	}
}

// ResolveUIRouteDataInput is the input contract for ResolveUIRouteData.
type ResolveUIRouteDataInput struct {
	ResponseWriter   http.ResponseWriter
	Request          *http.Request
	NestedRouter     *nestedmux.Router
	IsJSON           bool
	RequestedBuildID string

	ExecuteRouteDataStageOne func(
		http.ResponseWriter,
		*http.Request,
		*nestedmux.Router,
		string,
	) *routepipeline.RouteResult
	GetDefaultHeadElements func(*http.Request) ([]*htmlutil.Element, error)
	BuildResolvedRouteData func(
		http.ResponseWriter,
		*routepipeline.RouteResult,
		bool,
		[]*htmlutil.Element,
	) *routepipeline.RouteResult
	Log *slog.Logger
}

// ResolveUIRouteData resolves route data for one loaders request, including
// optional custom default-head execution behavior.
func ResolveUIRouteData(
	input ResolveUIRouteDataInput,
) *routepipeline.RouteResult {
	if input.GetDefaultHeadElements == nil {
		return resolveUIRouteDataWithoutDefaultHead(input)
	}
	return resolveUIRouteDataWithDefaultHead(input)
}

func resolveUIRouteDataWithoutDefaultHead(
	input ResolveUIRouteDataInput,
) *routepipeline.RouteResult {
	routeResult := input.ExecuteRouteDataStageOne(
		input.ResponseWriter,
		input.Request,
		input.NestedRouter,
		input.RequestedBuildID,
	)
	if routeResult.MergedResponseProxy != nil {
		routeResult.MergedResponseProxy.ApplyToResponseWriter(
			input.ResponseWriter,
			input.Request,
		)
	}
	if routeResult.TerminalState != routepipeline.RouteTerminalStateNone {
		return routeResult
	}
	return input.BuildResolvedRouteData(
		input.ResponseWriter,
		routeResult,
		input.IsJSON,
		nil,
	)
}

func resolveUIRouteDataWithDefaultHead(
	input ResolveUIRouteDataInput,
) *routepipeline.RouteResult {
	res := response.New(input.ResponseWriter)
	var (
		defaultHeadElements   []*htmlutil.Element
		defaultHeadErr        error
		defaultHeadWaitGroup  sync.WaitGroup
		cancelDefaultHeadWork context.CancelFunc
	)

	defaultHeadContext, cancelDefaultHead := context.WithCancel(
		input.Request.Context(),
	)
	cancelDefaultHeadWork = cancelDefaultHead
	defaultHeadRequest := input.Request.Clone(defaultHeadContext)

	defaultHeadWaitGroup.Add(1)
	go func() {
		defer defaultHeadWaitGroup.Done()
		defaultHeadElements, defaultHeadErr = input.GetDefaultHeadElements(
			defaultHeadRequest,
		)
	}()

	defer func() {
		if cancelDefaultHeadWork != nil {
			cancelDefaultHeadWork()
		}
	}()

	routeResult := input.ExecuteRouteDataStageOne(
		input.ResponseWriter,
		input.Request,
		input.NestedRouter,
		input.RequestedBuildID,
	)
	if routeResult.MergedResponseProxy != nil {
		routeResult.MergedResponseProxy.ApplyToResponseWriter(
			input.ResponseWriter,
			input.Request,
		)
	}
	if routeResult.TerminalState != routepipeline.RouteTerminalStateNone {
		if cancelDefaultHeadWork != nil {
			cancelDefaultHeadWork()
		}
		return routeResult
	}

	defaultHeadWaitGroup.Wait()
	if defaultHeadErr != nil {
		if input.Log != nil {
			input.Log.Error(
				"Error in getUIRouteData",
				"error",
				defaultHeadErr.Error(),
			)
		}
		res.InternalServerError()
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateError,
			BuildID:       routeResult.BuildID,
		}
	}

	return input.BuildResolvedRouteData(
		input.ResponseWriter,
		routeResult,
		input.IsJSON,
		defaultHeadElements,
	)
}

// BuildLoadersHTMLResponseBytesForRuntimeInput is the input contract for
// BuildLoadersHTMLResponseBytesForRuntime.
type BuildLoadersHTMLResponseBytesForRuntimeInput struct {
	Request     *http.Request
	RouteResult *routepipeline.RouteResult
	RouteData   *routepipeline.RouteDataFinal

	GetRootTemplateDataOrEmpty func(*http.Request) (map[string]any, error)
	RenderHeadElements         func(*headels.SortedAndPreEscapedHeadEls) (template.HTML, error)
	CriticalCSSStyleElement    template.HTML
	StyleSheetLinkElement      template.HTML

	VormaSymbolStr               string
	TemplateDataKeyHeadElements  string
	TemplateDataKeyBodyScripts   string
	TemplateDataKeySSRScript     string
	TemplateDataKeySSRScriptHash string
	TemplateDataKeyRootElementID string
	ClientRootElementID          string
	PublicPathPrefix             string
	RefreshScript                template.HTML
	ClientEntry                  string
	UseReactVariant              bool
}

// BuildLoadersHTMLResponseBytesForRuntime builds one loaders HTML response
// directly from runtime route result/data inputs.
func BuildLoadersHTMLResponseBytesForRuntime(
	input BuildLoadersHTMLResponseBytesForRuntimeInput,
) ([]byte, string, error) {
	rootTemplateData, rootTemplateDataError := input.GetRootTemplateDataOrEmpty(
		input.Request,
	)
	if rootTemplateDataError != nil {
		return nil, "Error getting root template data", rootTemplateDataError
	}

	htmlRenderSnapshot := input.RouteResult.HTMLRenderSnapshot
	htmlBytes, htmlRenderError := rendering.BuildLoadersHTMLResponseBytes(
		rendering.BuildLoadersHTMLResponseInput{
			RenderHeadElements: func() (template.HTML, error) {
				return input.RenderHeadElements(
					input.RouteResult.Assets.SortedAndPreEscapedHeadEls,
				)
			},
			CriticalCSSStyleElement: input.CriticalCSSStyleElement,
			StyleSheetLinkElement:   input.StyleSheetLinkElement,
			SSRRuntimeState: rendering.SSRRuntimeState{
				VormaSymbolStr:    input.VormaSymbolStr,
				IsDev:             input.RouteResult.HTMLRenderSnapshot.IsDevMode,
				BuildID:           input.RouteResult.BuildID,
				RootElementID:     input.ClientRootElementID,
				PublicPathPrefix:  input.PublicPathPrefix,
				RouteManifestFile: input.RouteResult.RouteManifestFileSnapshot,
			},
			SSRRouteData: rendering.SSRRouteData{
				ViteDevURL:              input.RouteResult.Assets.ViteDevURL,
				OutermostServerError:    input.RouteData.RouteDataCore.OutermostServerError,
				OutermostServerErrorIdx: input.RouteData.RouteDataCore.OutermostServerErrorIdx,
				ErrorExportKeys:         input.RouteData.RouteDataCore.ErrorExportKeys,
				MatchedPatterns:         input.RouteData.RouteDataCore.MatchedPatterns,
				LoadersData:             input.RouteData.RouteDataCore.LoadersData,
				ImportURLs:              input.RouteData.RouteDataCore.ImportURLs,
				ExportKeys:              input.RouteData.RouteDataCore.ExportKeys,
				HasRootData:             input.RouteData.RouteDataCore.HasRootData,
				Params:                  input.RouteData.RouteDataCore.Params,
				SplatValues:             input.RouteData.RouteDataCore.SplatValues,
			},
			RootTemplateData: rootTemplateData,

			TemplateDataKeyHeadElements:  input.TemplateDataKeyHeadElements,
			TemplateDataKeyBodyScripts:   input.TemplateDataKeyBodyScripts,
			TemplateDataKeySSRScript:     input.TemplateDataKeySSRScript,
			TemplateDataKeySSRScriptHash: input.TemplateDataKeySSRScriptHash,
			TemplateDataKeyRootElementID: input.TemplateDataKeyRootElementID,
			ClientRootElementID:          input.ClientRootElementID,

			BodyScriptsInput: rendering.BodyScriptsInput{
				RenderSnapshot:   htmlRenderSnapshot,
				PublicPathPrefix: input.PublicPathPrefix,
				RefreshScript:    input.RefreshScript,
				ClientEntry:      input.ClientEntry,
				UseReactVariant:  input.UseReactVariant,
			},
		},
	)
	if htmlRenderError != nil {
		return nil, "Error rendering template", htmlRenderError
	}
	return htmlBytes, "", nil
}

// BuildLoadersHandlerInput is the input contract for BuildLoadersHandler.
type BuildLoadersHandlerInput struct {
	NestedRouter *nestedmux.Router

	JSONQueryKey     string
	BuildIDHeaderKey string

	ResolveUIRouteData func(
		http.ResponseWriter,
		*http.Request,
		*nestedmux.Router,
		bool,
		string,
	) *routepipeline.RouteResult
	PublicPathPrefix              func() string
	BuildLoadersHTMLResponseBytes func(
		*http.Request,
		*routepipeline.RouteResult,
		*routepipeline.RouteDataFinal,
	) ([]byte, string, error)
	Log *slog.Logger
}

// BuildLoadersHandler constructs the top-level loaders handler.
func BuildLoadersHandler(
	input BuildLoadersHandlerInput,
) mux.TasksCtxRequirerFunc {
	return mux.TasksCtxRequirerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			res := response.New(w)
			requestedBuildID := r.URL.Query().Get(input.JSONQueryKey)
			isJSON := requestedBuildID != ""

			routeResult := input.ResolveUIRouteData(
				w,
				r,
				input.NestedRouter,
				isJSON,
				requestedBuildID,
			)
			if routeResult.TerminalState == routepipeline.RouteTerminalStateStaleBuild {
				routepipeline.EnsureLoadersCacheControlHeader(w, res)
				res.SetHeader(input.BuildIDHeaderKey, routeResult.BuildID)
				res.SetHeader(
					"X-Wave-Framework-Reload",
					routepipeline.BuildLoadersReloadURL(r),
				)
				res.OK()
				return
			}
			if routepipeline.WriteTerminalLoadersResponse(
				res,
				routeResult,
			) {
				return
			}

			routeData := routepipeline.BuildRouteDataFinal(
				routeResult,
				input.PublicPathPrefix(),
			)

			routepipeline.EnsureLoadersCacheControlHeader(w, res)

			if isJSON {
				if err := routepipeline.WriteLoadersJSONResponse(res, routeData); err != nil {
					if input.Log != nil {
						input.Log.Error(
							fmt.Sprintf("Error marshalling JSON: %v", err),
						)
					}
					res.InternalServerError()
				}
				return
			}

			htmlBytes, errorPrefix, err := input.BuildLoadersHTMLResponseBytes(
				r,
				routeResult,
				routeData,
			)
			if err != nil {
				if input.Log != nil {
					input.Log.Error(fmt.Sprintf("%s: %v", errorPrefix, err))
				}
				res.InternalServerError()
				return
			}
			res.HTMLBytes(htmlBytes)
		},
	)
}

// BuildActionsHandlerInput is the input contract for BuildActionsHandler.
type BuildActionsHandlerInput struct {
	Router *mux.Router

	CurrentBuildID                         func() string
	IsDevMode                              func() bool
	RoutesEndpointPath                     func() string
	TemplateEndpointPath                   func() string
	ValidateExpectedBuildIDOrWriteConflict func(
		http.ResponseWriter,
		*http.Request,
	) bool
	ReloadRoutesFromDisk   func() error
	ReloadTemplateFromDisk func() error
	Log                    *slog.Logger
}

// BuildActionsHandler constructs the top-level actions handler.
func BuildActionsHandler(
	input BuildActionsHandlerInput,
) mux.TasksCtxRequirerFunc {
	return mux.TasksCtxRequirerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			res := response.New(w)
			res.SetHeader("X-Wave-Framework-Build-Id", input.CurrentBuildID())
			handled := HandleDevReloadActionEndpoints(
				DevReloadActionEndpointsInput{
					ResponseWriter:       w,
					Request:              r,
					IsDevMode:            input.IsDevMode(),
					RoutesEndpointPath:   input.RoutesEndpointPath(),
					TemplateEndpointPath: input.TemplateEndpointPath(),
					ValidateExpectedBuildIDOrWriteConflict: func(
						responseWriter http.ResponseWriter,
						request *http.Request,
					) bool {
						return input.ValidateExpectedBuildIDOrWriteConflict(
							responseWriter,
							request,
						)
					},
					ReloadRoutesFromDisk:   input.ReloadRoutesFromDisk,
					ReloadTemplateFromDisk: input.ReloadTemplateFromDisk,
					Log:                    input.Log,
				},
			)
			if handled {
				return
			}
			input.Router.ServeHTTP(w, r)
		},
	)
}
