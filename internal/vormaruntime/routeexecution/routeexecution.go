// Package routeexecution orchestrates stage-one loaders route-data execution
// for vormaruntime.
//
// The routepipeline package owns pure route-data planning primitives, while
// this package coordinates match discovery, cache-key/cached-subset loading,
// loader-task output normalization, and callback hooks for runtime-specific
// concerns such as warning logs and client-safe error messages.
package routeexecution

import (
	"net/http"

	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
	"github.com/vormadev/vorma/kit/nestedmatcher"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
)

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
