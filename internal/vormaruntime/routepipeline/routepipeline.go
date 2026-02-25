// Package routepipeline runs Vorma's per-request routing and data-task pipeline.
//
// Keeping this orchestration separate from runtime bootstrap code allows the
// request execution contract to be tested without reinitializing full app state.
package routepipeline

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/vormadev/vorma/internal/vormaruntime/rendering"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmatcher"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/lab/viteutil"
)

// PathData contains route metadata needed by the route data planning pipeline.
type PathData struct {
	OriginalPattern string
	SrcPath         string
	OutPath         string
	ExportKey       string
	ErrorExportKey  string
	Deps            []string
}

// CachedItemSubset stores the cached route metadata used to build route-data
// payloads.
type CachedItemSubset struct {
	ImportURLs      []string
	ExportKeys      []string
	ErrorExportKeys []string
	Deps            []string
}

// RouteDataCore contains the core route data that is serialized to JSON for the
// client.
type RouteDataCore struct {
	OutermostServerError    string   `json:"outermostServerError,omitempty"`
	OutermostServerErrorIdx *int     `json:"outermostServerErrorIdx,omitempty"`
	ErrorExportKeys         []string `json:"errorExportKeys,omitempty"`

	MatchedPatterns []string `json:"matchedPatterns,omitempty"`
	LoadersData     []any    `json:"loadersData,omitempty"`
	ImportURLs      []string `json:"importURLs,omitempty"`
	ExportKeys      []string `json:"exportKeys,omitempty"`
	HasRootData     bool     `json:"hasRootData,omitempty"`

	Params      mux.Params `json:"params,omitempty"`
	SplatValues []string   `json:"splatValues,omitempty"`
	Deps        []string   `json:"deps,omitempty"`
}

// RouteDataFinal is the final structure serialized to JSON for the client.
type RouteDataFinal struct {
	*RouteDataCore
	Title      *htmlutil.Element   `json:"title,omitempty"`
	Meta       []*htmlutil.Element `json:"metaHeadEls,omitempty"`
	Rest       []*htmlutil.Element `json:"restHeadEls,omitempty"`
	CSSBundles []string            `json:"cssBundles,omitempty"`
	ViteDevURL string              `json:"viteDevURL,omitempty"`
}

// RouteAssets contains resolved CSS bundles and head elements.
type RouteAssets struct {
	SortedAndPreEscapedHeadEls *headels.SortedAndPreEscapedHeadEls
	CSSBundles                 []string
	ViteDevURL                 string
}

// BuildLoadersReloadURL builds the canonical reload URL from a JSON loaders
// request URL.
func BuildLoadersReloadURL(r *http.Request) string {
	newURL := *r.URL
	queryValues := r.URL.Query()
	queryValues.Del("vorma_json")
	newURL.RawQuery = queryValues.Encode()
	return newURL.String()
}

// WriteTerminalLoadersResponse writes terminal loader status responses when a
// route result is in a terminal state.
func WriteTerminalLoadersResponse(
	res response.Response,
	routeResult *RouteResult,
) bool {
	switch routeResult.TerminalState {
	case RouteTerminalStateNotFound:
		res.NotFound()
		return true
	case RouteTerminalStateRedirect, RouteTerminalStateError:
		return true
	default:
		return false
	}
}

// BuildRouteDataFinal maps route planning output to the final JSON payload
// shape returned by loaders endpoints.
func BuildRouteDataFinal(
	routeResult *RouteResult,
) *RouteDataFinal {
	return &RouteDataFinal{
		RouteDataCore: routeResult.Core,
		Title:         routeResult.Assets.SortedAndPreEscapedHeadEls.Title,
		Meta:          routeResult.Assets.SortedAndPreEscapedHeadEls.Meta,
		Rest:          routeResult.Assets.SortedAndPreEscapedHeadEls.Rest,
		CSSBundles:    routeResult.Assets.CSSBundles,
		ViteDevURL:    routeResult.Assets.ViteDevURL,
	}
}

// EnsureLoadersCacheControlHeader applies default loaders cache-control
// semantics when the response has not already set cache-control.
func EnsureLoadersCacheControlHeader(
	w http.ResponseWriter,
	res response.Response,
) {
	if w.Header().Get("Cache-Control") == "" {
		res.SetHeader(
			"Cache-Control",
			"private, max-age=0, must-revalidate, no-cache",
		)
	}
}

// WriteLoadersJSONResponse serializes the final route payload as JSON.
func WriteLoadersJSONResponse(
	res response.Response,
	routeData *RouteDataFinal,
) error {
	jsonBytes, err := json.Marshal(routeData)
	if err != nil {
		return err
	}
	res.JSONBytes(jsonBytes)
	return nil
}

// RouteTerminalState describes request-serving terminal states for route-data
// execution.
type RouteTerminalState uint8

const (
	RouteTerminalStateNone RouteTerminalState = iota
	RouteTerminalStateNotFound
	RouteTerminalStateRedirect
	RouteTerminalStateError
	RouteTerminalStateStaleBuild
)

// RouteResult is the full result of route resolution, including early-return
// signals.
type RouteResult struct {
	TerminalState RouteTerminalState
	BuildID       string

	Core                      *RouteDataCore
	HeadElements              []*htmlutil.Element
	CSSBundles                []string
	Assets                    *RouteAssets
	IsDev                     bool
	HTMLRenderSnapshot        rendering.LoadersHTMLRenderSnapshot
	RouteManifestFileSnapshot string
	MergedResponseProxy       *response.Proxy
}

// RuntimeSnapshot captures route-data planning state for a single coherent
// generation.
type RuntimeSnapshot struct {
	BuildID                  string
	IsDev                    bool
	Paths                    map[string]*PathData
	ClientEntryDeps          []string
	ClientEntryOut           string
	DepToCSSBundleMap        map[string][]string
	HTMLRenderSnapshot       rendering.LoadersHTMLRenderSnapshot
	RouteManifestFile        string
	RouteDataSnapshotVersion uint64
	RouteDataCache           *sync.Map
}

// RuntimeSnapshotFromCoreInput captures runtimecore-backed mutable runtime
// state used to build one routepipeline runtime snapshot.
type RuntimeSnapshotFromCoreInput struct {
	BuildID                  string
	IsDev                    bool
	Paths                    map[string]*runtimecore.RoutePath
	ClientEntryDeps          []string
	ClientEntryOut           string
	DepToCSSBundleMap        map[string][]string
	RootTemplate             *template.Template
	RouteManifestFile        string
	RouteDataSnapshotVersion uint64
	RouteDataCache           *sync.Map
}

// BuildRuntimeSnapshotFromCore converts runtimecore-backed mutable state to the
// routepipeline runtime snapshot shape.
func BuildRuntimeSnapshotFromCore(
	input RuntimeSnapshotFromCoreInput,
) RuntimeSnapshot {
	return RuntimeSnapshot{
		BuildID:           input.BuildID,
		IsDev:             input.IsDev,
		Paths:             convertRuntimeCorePathsToPathData(input.Paths),
		ClientEntryDeps:   input.ClientEntryDeps,
		ClientEntryOut:    input.ClientEntryOut,
		DepToCSSBundleMap: input.DepToCSSBundleMap,
		HTMLRenderSnapshot: rendering.LoadersHTMLRenderSnapshot{
			IsDevMode:      input.IsDev,
			ClientEntryOut: input.ClientEntryOut,
			RootTemplate:   input.RootTemplate,
		},
		RouteManifestFile:        input.RouteManifestFile,
		RouteDataSnapshotVersion: input.RouteDataSnapshotVersion,
		RouteDataCache:           input.RouteDataCache,
	}
}

func convertRuntimeCorePathsToPathData(
	paths map[string]*runtimecore.RoutePath,
) map[string]*PathData {
	if paths == nil {
		return nil
	}

	out := make(map[string]*PathData, len(paths))
	for pattern, pathValue := range paths {
		if pathValue == nil {
			out[pattern] = nil
			continue
		}
		out[pattern] = &PathData{
			OriginalPattern: pathValue.OriginalPattern,
			SrcPath:         pathValue.SrcPath,
			OutPath:         pathValue.OutPath,
			ExportKey:       pathValue.ExportKey,
			ErrorExportKey:  pathValue.ErrorExportKey,
			Deps:            pathValue.Deps,
		}
	}
	return out
}

// RouteDataExecutionInputs are the prepared inputs for route-data planning.
type RouteDataExecutionInputs struct {
	MatchResults    *nestedmatcher.Results
	Matches         []*nestedmatcher.Match
	MatchedPatterns []string
	Cached          *CachedItemSubset
	RuntimeSnapshot RuntimeSnapshot
}

// RouteStageOnePlannerInput is the fully-resolved input for route-data planner
// execution.
type RouteStageOnePlannerInput struct {
	MatchResults              *nestedmatcher.Results
	Matches                   []*nestedmatcher.Match
	MatchedPatterns           []string
	Cached                    *CachedItemSubset
	RuntimeSnapshot           RuntimeSnapshot
	HasRootData               bool
	LoadersData               []any
	OutermostLoaderErrorIndex *int
	ClientLoaderErrorMessage  string
	ResponseProxies           []*response.Proxy
	MergedResponseProxy       *response.Proxy
}

type routeErrorCutPlan struct {
	cutIdx           int
	headRouteCount   int
	clientMessage    string
	depsForRouteData []string
}

// BuildRouteDataCacheKey builds the stable cache key used for per-snapshot
// route-data metadata caching.
func BuildRouteDataCacheKey(
	matches []*nestedmatcher.Match,
	isDev bool,
	buildID string,
	routeDataSnapshotVersion uint64,
) string {
	snapshotVersionString := strconv.FormatUint(routeDataSnapshotVersion, 10)
	var sb strings.Builder
	sb.Grow(len(buildID) + len(snapshotVersionString) + (len(matches) * 16) + 3)
	if isDev {
		sb.WriteByte('1')
	} else {
		sb.WriteByte('0')
	}
	sb.WriteByte('|')
	sb.WriteString(snapshotVersionString)
	sb.WriteByte('|')
	sb.WriteString(buildID)
	sb.WriteByte('|')
	for _, match := range matches {
		sb.WriteString(match.NormalizedPattern())
		sb.WriteByte(';')
	}
	return sb.String()
}

// CollectMatchedPatterns materializes original route patterns from nested
// match results in stable route order.
func CollectMatchedPatterns(matches []*nestedmatcher.Match) []string {
	matchedPatterns := make([]string, len(matches))
	for i, match := range matches {
		matchedPatterns[i] = match.OriginalPattern()
	}
	return matchedPatterns
}

// LoadOrBuildCachedItemSubset returns the cached metadata subset for the
// request path, rebuilding it if not already cached.
func LoadOrBuildCachedItemSubset(
	cacheKey string,
	matches []*nestedmatcher.Match,
	pathsSnapshot map[string]*PathData,
	clientEntryDepsSnapshot []string,
	isDev bool,
	expectedSnapshotVersion uint64,
	routeDataCacheSnapshot *sync.Map,
	isSnapshotVersionCurrent func(uint64) bool,
) *CachedItemSubset {
	if routeDataCacheSnapshot == nil {
		return BuildCachedItemSubset(
			matches,
			pathsSnapshot,
			clientEntryDepsSnapshot,
			isDev,
		)
	}

	if cachedValue, isCached := routeDataCacheSnapshot.Load(cacheKey); isCached {
		return cachedValue.(*CachedItemSubset)
	}

	cached := BuildCachedItemSubset(
		matches,
		pathsSnapshot,
		clientEntryDepsSnapshot,
		isDev,
	)
	if isSnapshotVersionCurrent != nil &&
		isSnapshotVersionCurrent(expectedSnapshotVersion) {
		routeDataCacheSnapshot.Store(cacheKey, cached)
	}
	return cached
}

// BuildCachedItemSubset constructs the route metadata needed to serialize the
// route-data payload.
func BuildCachedItemSubset(
	matches []*nestedmatcher.Match,
	pathsSnapshot map[string]*PathData,
	clientEntryDepsSnapshot []string,
	isDev bool,
) *CachedItemSubset {
	cached := &CachedItemSubset{
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
		cached.ErrorExportKeys = append(
			cached.ErrorExportKeys,
			foundPath.ErrorExportKey,
		)
	}

	cached.Deps = GetDepsFromData(
		matches,
		pathsSnapshot,
		clientEntryDepsSnapshot,
	)
	return cached
}

// ComputeHasRootData reports whether the first matched route is root and ran a
// task handler.
func ComputeHasRootData(
	matchResults *nestedmatcher.Results,
	tasksResults *nestedmux.TasksResults,
) bool {
	return len(matchResults.Matches) > 0 &&
		matchResults.Matches[0].NormalizedPattern() == "" &&
		tasksResults.HasTaskHandlerAt(0)
}

// CollectLoadersDataAndErrors normalizes nested task outputs into aligned data
// and error slices.
func CollectLoadersDataAndErrors(
	tasksResults *nestedmux.TasksResults,
) ([]any, []error) {
	numberOfLoaders := len(tasksResults.Slice)
	loadersData := make([]any, numberOfLoaders)
	loadersErrs := make([]error, numberOfLoaders)
	if numberOfLoaders == 0 {
		return loadersData, loadersErrs
	}

	for i := 0; i < numberOfLoaders; i++ {
		result := tasksResults.Slice[i]
		loadersData[i] = result.Data()
		loadersErrs[i] = result.Err()
	}
	return loadersData, loadersErrs
}

// FindFirstErrorIndex returns the first non-nil error index, if any.
func FindFirstErrorIndex(errs []error) *int {
	for i, err := range errs {
		if err != nil {
			out := i
			return &out
		}
	}
	return nil
}

// DetectTerminalStateFromMergedResponseProxy maps merged proxy response state
// into route-terminal state.
func DetectTerminalStateFromMergedResponseProxy(
	mergedResponseProxy *response.Proxy,
) RouteTerminalState {
	if mergedResponseProxy == nil {
		return RouteTerminalStateNone
	}

	if mergedResponseProxy.IsError() {
		return RouteTerminalStateError
	}
	if mergedResponseProxy.IsRedirect() {
		return RouteTerminalStateRedirect
	}

	return RouteTerminalStateNone
}

// PlanRouteResultFromResolvedTaskOutcomes plans the route result from resolved
// loader outcomes and merged response state.
func PlanRouteResultFromResolvedTaskOutcomes(
	input RouteStageOnePlannerInput,
) *RouteResult {
	terminalState := DetectTerminalStateFromMergedResponseProxy(
		input.MergedResponseProxy,
	)
	if terminalState != RouteTerminalStateNone {
		return &RouteResult{
			TerminalState:       terminalState,
			BuildID:             input.RuntimeSnapshot.BuildID,
			MergedResponseProxy: input.MergedResponseProxy,
		}
	}

	cached := input.Cached
	if cached == nil {
		cached = buildEmptyCachedItemSubset(len(input.Matches))
	}
	matchResults := input.MatchResults
	if matchResults == nil {
		matchResults = &nestedmatcher.Results{}
	}

	normalizedInput := input
	normalizedInput.Cached = cached
	normalizedInput.MatchResults = matchResults

	cutPlan := buildRouteErrorCutPlan(normalizedInput)
	core := BuildRouteDataCore(
		normalizedInput.MatchResults,
		normalizedInput.MatchedPatterns,
		normalizedInput.LoadersData,
		normalizedInput.Cached,
		normalizedInput.HasRootData,
		normalizedInput.OutermostLoaderErrorIndex,
		cutPlan.clientMessage,
		cutPlan.cutIdx,
		cutPlan.depsForRouteData,
	)
	cssBundles := GetCSSBundles(
		core.Deps,
		normalizedInput.RuntimeSnapshot.ClientEntryOut,
		normalizedInput.RuntimeSnapshot.DepToCSSBundleMap,
	)

	return &RouteResult{
		BuildID: normalizedInput.RuntimeSnapshot.BuildID,
		Core:    core,
		HeadElements: CollectFlattenedHeadElementsForPrefix(
			normalizedInput.ResponseProxies,
			cutPlan.headRouteCount,
		),
		CSSBundles:                cssBundles,
		IsDev:                     normalizedInput.RuntimeSnapshot.IsDev,
		HTMLRenderSnapshot:        normalizedInput.RuntimeSnapshot.HTMLRenderSnapshot,
		RouteManifestFileSnapshot: normalizedInput.RuntimeSnapshot.RouteManifestFile,
		MergedResponseProxy:       normalizedInput.MergedResponseProxy,
	}
}

func buildEmptyCachedItemSubset(routeCount int) *CachedItemSubset {
	return &CachedItemSubset{
		ImportURLs:      make([]string, routeCount),
		ExportKeys:      make([]string, routeCount),
		ErrorExportKeys: make([]string, routeCount),
	}
}

func buildRouteErrorCutPlan(input RouteStageOnePlannerInput) routeErrorCutPlan {
	plan := routeErrorCutPlan{
		cutIdx:         len(input.Matches),
		headRouteCount: len(input.Matches),
	}
	if input.Cached != nil {
		plan.depsForRouteData = input.Cached.Deps
	}
	if input.OutermostLoaderErrorIndex == nil {
		return plan
	}

	derefErrorIdx := *input.OutermostLoaderErrorIndex
	plan.clientMessage = input.ClientLoaderErrorMessage
	plan.cutIdx = derefErrorIdx + 1
	plan.headRouteCount = derefErrorIdx
	if plan.cutIdx < len(input.Matches) {
		plan.depsForRouteData = GetDepsFromData(
			input.Matches[:plan.cutIdx],
			input.RuntimeSnapshot.Paths,
			input.RuntimeSnapshot.ClientEntryDeps,
		)
	}
	return plan
}

// BuildRouteDataCore builds the serialized route-data payload core.
func BuildRouteDataCore(
	matchResults *nestedmatcher.Results,
	matchedPatterns []string,
	loadersData []any,
	cached *CachedItemSubset,
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

// CollectFlattenedHeadElementsForPrefix collects and flattens route head
// elements for a prefix route count.
func CollectFlattenedHeadElementsForPrefix(
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
		routeElements := responseProxies[routeIdx].HeadEls().Collect()
		headElsByRoute = append(headElsByRoute, routeElements)
		total += len(routeElements)
	}

	flattenedHeadEls := make([]*htmlutil.Element, 0, total)
	for _, routeElements := range headElsByRoute {
		flattenedHeadEls = append(flattenedHeadEls, routeElements...)
	}
	return flattenedHeadEls
}

// BuildRouteAssetsInput is the input contract for BuildRouteAssets.
type BuildRouteAssetsInput struct {
	RouteResult                    *RouteResult
	DefaultHeadElements            []*htmlutil.Element
	IsJSON                         bool
	PublicPathPrefix               string
	ToSortedAndPreEscapedHeadElsFn func([]*htmlutil.Element) *headels.SortedAndPreEscapedHeadEls
}

// BuildRouteAssets resolves final head elements and CSS bundles for response
// rendering.
func BuildRouteAssets(input BuildRouteAssetsInput) (*RouteAssets, error) {
	if input.RouteResult == nil {
		return nil, fmt.Errorf("route result is nil")
	}
	if input.RouteResult.Core == nil {
		return nil, fmt.Errorf("route result core is nil")
	}
	if input.ToSortedAndPreEscapedHeadElsFn == nil {
		return nil, fmt.Errorf("head elements sorting callback is nil")
	}

	cssBundles := input.RouteResult.CSSBundles
	combinedHeadEls := combineDefaultAndRouteHeadElements(
		input.DefaultHeadElements,
		input.RouteResult.HeadElements,
	)

	if shouldAppendProductionAssetLinks(input.RouteResult.IsDev, input.IsJSON) {
		combinedHeadEls = appendProductionAssetLinks(
			combinedHeadEls,
			input.PublicPathPrefix,
			input.RouteResult.Core.Deps,
			cssBundles,
		)
	}

	headEls := input.ToSortedAndPreEscapedHeadElsFn(combinedHeadEls)
	return &RouteAssets{
		SortedAndPreEscapedHeadEls: headEls,
		CSSBundles:                 cssBundles,
		ViteDevURL: GetViteDevURLForMode(
			input.RouteResult.IsDev,
		),
	}, nil
}

func combineDefaultAndRouteHeadElements(
	defaultHeadElsRaw []*htmlutil.Element,
	routeHeadEls []*htmlutil.Element,
) []*htmlutil.Element {
	out := make(
		[]*htmlutil.Element,
		0,
		len(defaultHeadElsRaw)+len(routeHeadEls),
	)
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
			Tag: "link",
			AttributesKnownSafe: map[string]string{
				"rel":  "modulepreload",
				"href": publicPathPrefix + dep,
			},
			SelfClosing: true,
		})
	}

	for _, cssBundle := range cssBundles {
		out = append(out, &htmlutil.Element{
			Tag: "link",
			AttributesKnownSafe: map[string]string{
				"rel":  "stylesheet",
				"href": publicPathPrefix + cssBundle,
			},
			Attributes: map[string]string{
				"data-vorma-css-bundle": cssBundle,
			},
			SelfClosing: true,
		})
	}

	return out
}

// GetViteDevURLForMode returns the Vite dev server URL for dev mode requests.
func GetViteDevURLForMode(isDevMode bool) string {
	if !isDevMode {
		return ""
	}
	return fmt.Sprintf("http://localhost:%s", viteutil.GetVitePortStr())
}

// GetDepsFromData computes request deps in stable order with de-duplication and
// client-entry deps first.
func GetDepsFromData(
	matches []*nestedmatcher.Match,
	paths map[string]*PathData,
	clientEntryDeps []string,
) []string {
	var deps []string
	seen := make(map[string]struct{}, len(matches))
	handleDeps := func(src []string) {
		for _, d := range src {
			if _, ok := seen[d]; !ok {
				deps = append(deps, d)
				seen[d] = struct{}{}
			}
		}
	}
	if clientEntryDeps != nil {
		handleDeps(clientEntryDeps)
	}
	for _, match := range matches {
		path := paths[match.OriginalPattern()]
		if path == nil {
			continue
		}
		handleDeps(path.Deps)
	}
	return deps
}

// GetCSSBundles resolves CSS bundles from deps, with client-entry bundles first
// and stable de-duplication.
func GetCSSBundles(
	deps []string,
	clientEntryOut string,
	depToCSSBundleMap map[string][]string,
) []string {
	clientEntryBundles := depToCSSBundleMap[clientEntryOut]
	seen := make(map[string]struct{})
	cssBundles := make([]string, 0, len(deps))

	addBundles := func(bundles []string) {
		for _, bundle := range bundles {
			if _, exists := seen[bundle]; !exists {
				seen[bundle] = struct{}{}
				cssBundles = append(cssBundles, bundle)
			}
		}
	}

	if len(clientEntryBundles) > 0 {
		addBundles(clientEntryBundles)
	}

	for _, dep := range deps {
		if bundles, exists := depToCSSBundleMap[dep]; exists {
			addBundles(bundles)
		}
	}

	return cssBundles
}
