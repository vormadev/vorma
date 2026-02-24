// Package runtimehttp owns HTTP router construction, action-input parsing, and
// stage-one route execution orchestration for vormaruntime.
//
// Keeping these concerns isolated makes runtime HTTP behavior testable without
// the full mutable runtime state machine.
package runtimehttp

import (
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
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
	matchedPatterns := routepipeline.CollectMatchedPatterns(matches)
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
