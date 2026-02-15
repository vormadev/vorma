package vormaruntime

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
)

var gmpdCache sync.Map

type cachedItemSubset struct {
	ImportURLs      []string
	ExportKeys      []string
	ErrorExportKeys []string
	Deps            []string
}

type SplatValues []string

// RouteDataCore contains the core route data that is serialized to JSON for the client.
type RouteDataCore struct {
	OutermostServerError    string   `json:"outermostServerError,omitempty"`
	OutermostServerErrorIdx *int     `json:"outermostServerErrorIdx,omitempty"`
	ErrorExportKeys         []string `json:"errorExportKeys,omitempty"`

	MatchedPatterns []string `json:"matchedPatterns,omitempty"`
	LoadersData     []any    `json:"loadersData,omitempty"`
	ImportURLs      []string `json:"importURLs,omitempty"`
	ExportKeys      []string `json:"exportKeys,omitempty"`
	HasRootData     bool     `json:"hasRootData,omitempty"`

	Params      mux.Params  `json:"params,omitempty"`
	SplatValues SplatValues `json:"splatValues,omitempty"`
	Deps        []string    `json:"deps,omitempty"`
}

// RouteAssets contains resolved CSS bundles and head elements.
type RouteAssets struct {
	SortedAndPreEscapedHeadEls *headels.SortedAndPreEscapedHeadEls
	CSSBundles                 []string
	ViteDevURL                 string
}

// RouteResult is the full result of route resolution, including early-return signals.
type RouteResult struct {
	terminalState routeTerminalState
	buildID       string

	core         *RouteDataCore
	headElements []*htmlutil.Element
	cssBundles   []string
	assets       *RouteAssets
	isDev        bool
}

type routeTerminalState uint8

const (
	routeTerminalStateNone routeTerminalState = iota
	routeTerminalStateNotFound
	routeTerminalStateRedirect
	routeTerminalStateError
	routeTerminalStateStaleBuild
)

// RouteDataFinal is the final structure serialized to JSON for the client.
type RouteDataFinal struct {
	*RouteDataCore
	Title      *htmlutil.Element   `json:"title,omitempty"`
	Meta       []*htmlutil.Element `json:"metaHeadEls,omitempty"`
	Rest       []*htmlutil.Element `json:"restHeadEls,omitempty"`
	CSSBundles []string            `json:"cssBundles,omitempty"`
	ViteDevURL string              `json:"viteDevURL,omitempty"`
}

type routeDataExecutionInputs struct {
	matchResults            *matcher.FindNestedMatchesResults
	matches                 []*matcher.Match
	matchedPatterns         []string
	pathsSnapshot           map[string]*Path
	clientEntryDepsSnapshot []string
	clientEntryOutSnapshot  string
	depToCSSBundlesSnapshot map[string][]string
	cached                  *cachedItemSubset
	buildID                 string
	isDev                   bool
}

type routeErrorCutPlan struct {
	cutIdx           int
	headRouteCount   int
	clientMessage    string
	depsForRouteData []string
}

func (v *Vorma) getRouteDataStage1(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *mux.NestedRouter,
	requestedBuildID string,
) *RouteResult {
	inputs, found := v.prepareRouteDataExecutionInputs(r, nestedRouter)
	if inputs.buildID != "" {
		w.Header().Set(VormaBuildIDHeaderKey, inputs.buildID)
	}
	if requestedBuildID != "" && requestedBuildID != inputs.buildID {
		return &RouteResult{
			terminalState: routeTerminalStateStaleBuild,
			buildID:       inputs.buildID,
		}
	}
	if !found {
		return &RouteResult{
			terminalState: routeTerminalStateNotFound,
			buildID:       inputs.buildID,
		}
	}

	tasksResults := mux.RunNestedTasks(nestedRouter, r, inputs.matchResults)
	if tasksResults == nil {
		v.Log.Error("Missing TasksCtx for loaders request. Use a mux.Router stack or wrap with mux.InjectTasksCtxMiddleware.")
		res := response.New(w)
		res.InternalServerError()
		return &RouteResult{
			terminalState: routeTerminalStateError,
			buildID:       inputs.buildID,
		}
	}

	hasRootData := computeHasRootData(inputs.matchResults, tasksResults)

	terminalState := applyMergedResponseProxyAndDetectTerminalState(w, r, tasksResults)
	if terminalState != routeTerminalStateNone {
		return &RouteResult{
			terminalState: terminalState,
			buildID:       inputs.buildID,
		}
	}

	loadersData, loadersErrs := v.collectLoadersDataAndErrors(tasksResults, inputs.matchedPatterns)
	outermostErrorIdx := findFirstErrorIndex(loadersErrs)
	cutPlan := v.buildRouteErrorCutPlan(inputs, loadersErrs, outermostErrorIdx)
	core := buildRouteDataCore(
		inputs.matchResults,
		inputs.matchedPatterns,
		loadersData,
		inputs.cached,
		hasRootData,
		outermostErrorIdx,
		cutPlan.clientMessage,
		cutPlan.cutIdx,
		cutPlan.depsForRouteData,
	)
	cssBundles := getCSSBundlesFromSnapshot(
		core.Deps,
		inputs.clientEntryOutSnapshot,
		inputs.depToCSSBundlesSnapshot,
	)

	return &RouteResult{
		buildID: inputs.buildID,
		core:    core,
		headElements: collectFlattenedHeadElementsForPrefix(
			tasksResults.ResponseProxies,
			cutPlan.headRouteCount,
		),
		cssBundles: cssBundles,
		isDev:      inputs.isDev,
	}
}

func (v *Vorma) buildRouteErrorCutPlan(
	inputs routeDataExecutionInputs,
	loadersErrs []error,
	outermostErrorIdx *int,
) routeErrorCutPlan {
	plan := routeErrorCutPlan{
		cutIdx:           len(inputs.matches),
		headRouteCount:   len(inputs.matches),
		depsForRouteData: inputs.cached.Deps,
	}
	if outermostErrorIdx == nil {
		return plan
	}

	derefErrorIdx := *outermostErrorIdx
	plan.clientMessage = v.resolveClientLoaderErrorMessage(
		loadersErrs[derefErrorIdx],
		inputs.matchedPatterns[derefErrorIdx],
	)
	plan.cutIdx = derefErrorIdx + 1
	plan.headRouteCount = derefErrorIdx
	if plan.cutIdx < len(inputs.matches) {
		plan.depsForRouteData = getDepsFromData(
			inputs.matches[:plan.cutIdx],
			inputs.pathsSnapshot,
			inputs.clientEntryDepsSnapshot,
		)
	}
	return plan
}

func (v *Vorma) prepareRouteDataExecutionInputs(
	r *http.Request,
	nestedRouter *mux.NestedRouter,
) (routeDataExecutionInputs, bool) {
	v.mu.RLock()

	buildID := v._buildID
	isDev := v._isDev

	matchResults, found := mux.FindNestedMatches(nestedRouter, r)
	if !found {
		v.mu.RUnlock()
		return routeDataExecutionInputs{
			buildID: buildID,
			isDev:   isDev,
		}, false
	}

	matches := matchResults.Matches
	routeDataSnapshotVersion := v._routeDataSnapshotVersion
	pathsSnapshot := v._paths
	clientEntryDepsSnapshot := v._clientEntryDeps
	clientEntryOutSnapshot := v._clientEntryOut
	depToCSSBundlesSnapshot := v._depToCSSBundleMap
	v.mu.RUnlock()

	matchedPatterns := collectMatchedPatterns(matches)
	cacheKey := v.buildRouteDataCacheKey(matches, isDev, buildID)
	cached := loadOrBuildCachedItemSubset(
		v,
		cacheKey,
		matches,
		pathsSnapshot,
		clientEntryDepsSnapshot,
		isDev,
		routeDataSnapshotVersion,
	)

	return routeDataExecutionInputs{
		matchResults:            matchResults,
		matches:                 matches,
		matchedPatterns:         matchedPatterns,
		pathsSnapshot:           pathsSnapshot,
		clientEntryDepsSnapshot: clientEntryDepsSnapshot,
		clientEntryOutSnapshot:  clientEntryOutSnapshot,
		depToCSSBundlesSnapshot: depToCSSBundlesSnapshot,
		cached:                  cached,
		buildID:                 buildID,
		isDev:                   isDev,
	}, true
}

func collectMatchedPatterns(matches []*matcher.Match) []string {
	matchedPatterns := make([]string, len(matches))
	for i, match := range matches {
		matchedPatterns[i] = match.OriginalPattern()
	}
	return matchedPatterns
}

func loadOrBuildCachedItemSubset(
	v *Vorma,
	cacheKey string,
	matches []*matcher.Match,
	pathsSnapshot map[string]*Path,
	clientEntryDepsSnapshot []string,
	isDev bool,
	expectedSnapshotVersion uint64,
) *cachedItemSubset {
	if cachedValue, isCached := gmpdCache.Load(cacheKey); isCached {
		return cachedValue.(*cachedItemSubset)
	}

	cached := buildCachedItemSubset(matches, pathsSnapshot, clientEntryDepsSnapshot, isDev)
	if v.isRouteDataSnapshotVersionCurrent(expectedSnapshotVersion) {
		gmpdCache.Store(cacheKey, cached)
	}
	return cached
}

func (v *Vorma) isRouteDataSnapshotVersionCurrent(expectedSnapshotVersion uint64) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._routeDataSnapshotVersion == expectedSnapshotVersion
}

func computeHasRootData(
	matchResults *matcher.FindNestedMatchesResults,
	tasksResults *mux.NestedTasksResults,
) bool {
	return len(matchResults.Matches) > 0 &&
		matchResults.Matches[0].NormalizedPattern() == "" &&
		tasksResults.GetHasTaskHandler(0)
}

func applyMergedResponseProxyAndDetectTerminalState(
	w http.ResponseWriter,
	r *http.Request,
	tasksResults *mux.NestedTasksResults,
) routeTerminalState {
	mergedResponseProxy := response.MergeProxyResponses(tasksResults.ResponseProxies...)
	if mergedResponseProxy == nil {
		return routeTerminalStateNone
	}

	mergedResponseProxy.ApplyToResponseWriter(w, r)
	if mergedResponseProxy.IsError() {
		return routeTerminalStateError
	}
	if mergedResponseProxy.IsRedirect() {
		return routeTerminalStateRedirect
	}

	return routeTerminalStateNone
}

func buildCachedItemSubset(
	matches []*matcher.Match,
	pathsSnapshot map[string]*Path,
	clientEntryDepsSnapshot []string,
	isDev bool,
) *cachedItemSubset {
	cached := &cachedItemSubset{
		ImportURLs:      make([]string, 0, len(matches)),
		ExportKeys:      make([]string, 0, len(matches)),
		ErrorExportKeys: make([]string, 0, len(matches)),
	}

	for _, match := range matches {
		foundPath := pathsSnapshot[match.OriginalPattern()]
		if foundPath == nil || foundPath.SrcPath == "" {
			cached.ImportURLs = append(cached.ImportURLs, "")
			cached.ExportKeys = append(cached.ExportKeys, "")
			cached.ErrorExportKeys = append(cached.ErrorExportKeys, "")
			continue
		}
		pathToUse := foundPath.OutPath
		if isDev {
			pathToUse = foundPath.SrcPath
		}
		cached.ImportURLs = append(cached.ImportURLs, "/"+pathToUse)
		cached.ExportKeys = append(cached.ExportKeys, foundPath.ExportKey)
		cached.ErrorExportKeys = append(cached.ErrorExportKeys, foundPath.ErrorExportKey)
	}

	cached.Deps = getDepsFromData(matches, pathsSnapshot, clientEntryDepsSnapshot)
	return cached
}

func (v *Vorma) collectLoadersDataAndErrors(
	tasksResults *mux.NestedTasksResults,
	matchedPatterns []string,
) ([]any, []error) {
	numberOfLoaders := len(matchedPatterns)
	loadersData := make([]any, numberOfLoaders)
	loadersErrs := make([]error, numberOfLoaders)
	if numberOfLoaders == 0 {
		return loadersData, loadersErrs
	}

	for i := 0; i < numberOfLoaders; i++ {
		result := tasksResults.Slice[i]
		loadersData[i] = result.Data()
		loadersErrs[i] = result.Err()

		if result.RanTask() && loadersErrs[i] == nil {
			shouldWarn := reflectutil.ExcludingNoneGetIsNilOrUltimatelyPointsToNil(loadersData[i])
			if shouldWarn {
				v.Log.Warn("Do not return nil values from loaders unless the underlying type is an empty struct or you are returning an error.",
					"pattern", matchedPatterns[i])
			}
		}
	}
	return loadersData, loadersErrs
}

func findFirstErrorIndex(errs []error) *int {
	for i, err := range errs {
		if err != nil {
			out := i
			return &out
		}
	}
	return nil
}

func (v *Vorma) resolveClientLoaderErrorMessage(err error, pattern string) string {
	var clientMsg string
	var errToLog error

	var loaderErr LoaderErrorMarker
	if errors.As(err, &loaderErr) {
		clientMsg = loaderErr.ClientMessage()
		errToLog = loaderErr.ServerError()
		if clientMsg == "" {
			clientMsg = "An error occurred"
			v.Log.Warn("LoaderError has empty ClientMessage(); sending generic error to client.")
		}
	} else {
		clientMsg = "An error occurred"
		errToLog = err
		v.Log.Warn("Sending generic error to client. Use vorma.LoaderError for custom client messages.")
	}

	if errToLog != nil {
		v.Log.Error("loader error", "pattern", pattern, "error", errToLog)
	}
	return clientMsg
}

func buildRouteDataCore(
	matchResults *matcher.FindNestedMatchesResults,
	matchedPatterns []string,
	loadersData []any,
	cached *cachedItemSubset,
	hasRootData bool,
	outermostErrorIdx *int,
	clientMsg string,
	cutIdx int,
	deps []string,
) *RouteDataCore {
	return &RouteDataCore{
		OutermostServerError:    clientMsg,
		OutermostServerErrorIdx: outermostErrorIdx,
		ErrorExportKeys:         cached.ErrorExportKeys[:cutIdx],
		MatchedPatterns:         matchedPatterns[:cutIdx],
		LoadersData:             loadersData[:cutIdx],
		ImportURLs:              cached.ImportURLs[:cutIdx],
		ExportKeys:              cached.ExportKeys[:cutIdx],
		HasRootData:             hasRootData,
		Params:                  matchResults.Params,
		SplatValues:             matchResults.SplatValues,
		Deps:                    deps,
	}
}

func collectFlattenedHeadElementsForPrefix(
	responseProxies []*response.Proxy,
	routeCount int,
) []*htmlutil.Element {
	if routeCount <= 0 || len(responseProxies) == 0 {
		return nil
	}
	if routeCount > len(responseProxies) {
		routeCount = len(responseProxies)
	}

	headElsByRoute := make([][]*htmlutil.Element, 0, routeCount)
	total := 0
	for routeIdx := 0; routeIdx < routeCount; routeIdx++ {
		routeElements := responseProxies[routeIdx].GetHeadEls().Collect()
		headElsByRoute = append(headElsByRoute, routeElements)
		total += len(routeElements)
	}

	flattenedHeadEls := make([]*htmlutil.Element, 0, total)
	for _, routeElements := range headElsByRoute {
		flattenedHeadEls = append(flattenedHeadEls, routeElements...)
	}
	return flattenedHeadEls
}

func (v *Vorma) buildRouteDataCacheKey(matches []*matcher.Match, isDev bool, buildID string) string {
	var sb strings.Builder

	appIdentity := v._routeDataCacheAppIdentity
	if appIdentity == "" {
		appIdentity = computeRouteDataCacheAppIdentity(v)
	}

	// Include app identity + mode + build to prevent cross-app/mode/build cache leakage.
	sb.Grow(len(appIdentity) + len(buildID) + (len(matches) * 16) + 3)
	sb.WriteString(appIdentity)
	sb.WriteByte('|')

	if isDev {
		sb.WriteByte('1')
	} else {
		sb.WriteByte('0')
	}
	sb.WriteByte('|')
	sb.WriteString(buildID)
	sb.WriteByte('|')
	for _, match := range matches {
		sb.WriteString(match.NormalizedPattern())
		sb.WriteByte(';')
	}
	return sb.String()
}

func computeRouteDataCacheAppIdentity(v *Vorma) string {
	var ptrBuf [20]byte
	ptrBytes := strconv.AppendUint(ptrBuf[:0], uint64(uintptr(unsafe.Pointer(v))), 16)
	return string(ptrBytes)
}

func (v *Vorma) getUIRouteData(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *mux.NestedRouter,
	isJSON bool,
	requestedBuildID string,
) *RouteResult {
	res := response.New(w)
	routeResult := v.getRouteDataStage1(w, r, nestedRouter, requestedBuildID)
	if routeResult.terminalState != routeTerminalStateNone {
		return routeResult
	}

	defaultHeadElsRaw, err := v.getDefaultHeadElsRaw(r)
	if err != nil {
		v.Log.Error("Error in getUIRouteData", "error", err.Error())
		res.InternalServerError()
		return &RouteResult{
			terminalState: routeTerminalStateError,
			buildID:       routeResult.buildID,
		}
	}

	assets := v.buildRouteAssets(routeResult, defaultHeadElsRaw, isJSON)
	return &RouteResult{
		buildID: routeResult.buildID,
		core:    routeResult.core,
		assets:  assets,
	}
}

func (v *Vorma) getDefaultHeadElsRaw(r *http.Request) ([]*htmlutil.Element, error) {
	defaultHeadEls := headels.New()
	if v.getDefaultHeadEls == nil {
		return defaultHeadEls.Collect(), nil
	}
	if err := v.getDefaultHeadEls(r, v, defaultHeadEls); err != nil {
		return nil, err
	}
	return defaultHeadEls.Collect(), nil
}

func (v *Vorma) buildRouteAssets(
	routeResult *RouteResult,
	defaultHeadElsRaw []*htmlutil.Element,
	isJSON bool,
) *RouteAssets {
	cssBundles := routeResult.cssBundles
	combinedHeadEls := combineDefaultAndRouteHeadElements(defaultHeadElsRaw, routeResult.headElements)

	if shouldAppendProductionAssetLinks(routeResult.isDev, isJSON) {
		combinedHeadEls = appendProductionAssetLinks(
			combinedHeadEls,
			v.Wave.GetPublicPathPrefix(),
			routeResult.core.Deps,
			cssBundles,
		)
	}

	headEls := v.headElsInst.ToSortedAndPreEscapedHeadEls(combinedHeadEls)
	return &RouteAssets{
		SortedAndPreEscapedHeadEls: headEls,
		CSSBundles:                 cssBundles,
		ViteDevURL:                 v.getViteDevURL(),
	}
}

func combineDefaultAndRouteHeadElements(
	defaultHeadElsRaw []*htmlutil.Element,
	routeHeadEls []*htmlutil.Element,
) []*htmlutil.Element {
	out := make([]*htmlutil.Element, 0, len(defaultHeadElsRaw)+len(routeHeadEls))
	out = append(out, defaultHeadElsRaw...)
	out = append(out, routeHeadEls...)
	return out
}

func shouldAppendProductionAssetLinks(isDev bool, isJSON bool) bool {
	return !isDev && !isJSON
}

func appendProductionAssetLinks(
	headElements []*htmlutil.Element,
	publicPathPrefix string,
	deps []string,
	cssBundles []string,
) []*htmlutil.Element {
	capacityToGrow := len(deps) + len(cssBundles)
	out := make([]*htmlutil.Element, 0, len(headElements)+capacityToGrow)
	out = append(out, headElements...)

	for _, dep := range deps {
		out = append(out, &htmlutil.Element{
			Tag:                 "link",
			AttributesKnownSafe: map[string]string{"rel": "modulepreload", "href": publicPathPrefix + dep},
			SelfClosing:         true,
		})
	}

	for _, cssBundle := range cssBundles {
		out = append(out, &htmlutil.Element{
			Tag:                 "link",
			AttributesKnownSafe: map[string]string{"rel": "stylesheet", "href": publicPathPrefix + cssBundle},
			Attributes:          map[string]string{"data-vorma-css-bundle": cssBundle},
			SelfClosing:         true,
		})
	}

	return out
}
