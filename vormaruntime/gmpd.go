package vormaruntime

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
	"golang.org/x/sync/errgroup"
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
	notFound    bool
	didRedirect bool
	didErr      bool

	core         *RouteDataCore
	headElements []*htmlutil.Element
	assets       *RouteAssets
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

func (v *Vorma) getRouteDataStage1(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *mux.NestedRouter,
) *RouteResult {
	realPath := matcher.StripTrailingSlash(r.URL.Path)
	if realPath == "" {
		realPath = "/"
	}

	matchResults, found := mux.FindNestedMatches(nestedRouter, r)
	if !found {
		return &RouteResult{notFound: true}
	}

	matches := matchResults.Matches
	matchedPatterns := make([]string, len(matches))
	for i, match := range matches {
		matchedPatterns[i] = match.OriginalPattern()
	}

	// Cache key generation based on normalized patterns
	var sb strings.Builder
	var growSize int
	for _, match := range matches {
		growSize += len(match.NormalizedPattern())
	}
	sb.Grow(growSize)
	for _, match := range matches {
		sb.WriteString(match.NormalizedPattern())
	}
	cacheKey := sb.String()

	var cached *cachedItemSubset
	cachedValue, isCached := gmpdCache.Load(cacheKey)

	if isCached {
		cached = cachedValue.(*cachedItemSubset)
	} else {
		// Cache Miss: Perform expensive path lookups and dependency graph traversal
		paths := v.GetPathsSnapshot()
		isDev := v.GetIsDevMode()

		cached = &cachedItemSubset{
			ImportURLs:      make([]string, 0, len(matches)),
			ExportKeys:      make([]string, 0, len(matches)),
			ErrorExportKeys: make([]string, 0, len(matches)),
		}

		for _, path := range matches {
			foundPath := paths[path.OriginalPattern()]
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

		// Expensive dependency graph traversal
		cached.Deps = v.getDepsFromSnapshot(matches, paths)

		gmpdCache.Store(cacheKey, cached)
	}

	tasksResults := mux.RunNestedTasks(nestedRouter, r, matchResults)

	var hasRootData bool
	if len(matchResults.Matches) > 0 &&
		matchResults.Matches[0].NormalizedPattern() == "" &&
		tasksResults.GetHasTaskHandler(0) {
		hasRootData = true
	}

	mergedResponseProxy := response.MergeProxyResponses(tasksResults.ResponseProxies...)
	if mergedResponseProxy != nil {
		mergedResponseProxy.ApplyToResponseWriter(w, r)
		if mergedResponseProxy.IsError() {
			return &RouteResult{didErr: true}
		}
		if mergedResponseProxy.IsRedirect() {
			return &RouteResult{didRedirect: true}
		}
	}

	var numberOfLoaders int
	if matchResults != nil {
		numberOfLoaders = len(matchResults.Matches)
	}

	loadersData := make([]any, numberOfLoaders)
	loadersErrs := make([]error, numberOfLoaders)

	if numberOfLoaders > 0 {
		for i, result := range tasksResults.Slice {
			if result != nil {
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
		}
	}

	var outermostErrorIdx *int
	for i, err := range loadersErrs {
		if err != nil {
			outermostErrorIdx = &i
			break
		}
	}

	// Collect head elements from each response proxy, maintaining index alignment
	// with matches. If a proxy is nil, append nil to preserve indices.
	loadersHeadEls := make([][]*htmlutil.Element, 0, numberOfLoaders)
	for _, responseProxy := range tasksResults.ResponseProxies {
		if responseProxy != nil {
			loadersHeadEls = append(loadersHeadEls, responseProxy.GetHeadEls().Collect())
		} else {
			loadersHeadEls = append(loadersHeadEls, nil)
		}
	}

	if outermostErrorIdx != nil {
		derefIdx := *outermostErrorIdx
		err := loadersErrs[derefIdx]
		pattern := matchedPatterns[derefIdx]

		var clientMsg string
		var errToLog error

		if loaderErr, ok := err.(LoaderErrorMarker); ok {
			clientMsg = loaderErr.ClientMessage()
			errToLog = loaderErr.ServerError()
		} else {
			clientMsg = "An error occurred"
			errToLog = err
			v.Log.Warn("Sending generic error to client. Use vorma.LoaderError for custom client messages.")
		}

		if errToLog != nil {
			v.Log.Error("loader error", "pattern", pattern, "error", errToLog)
		}

		headElsSlice := loadersHeadEls[:derefIdx]
		headEls := make([]*htmlutil.Element, 0)
		for _, slice := range headElsSlice {
			headEls = append(headEls, slice...)
		}

		cutIdx := derefIdx + 1
		return &RouteResult{
			core: &RouteDataCore{
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
				Deps:                    cached.Deps,
			},
			headElements: headEls,
		}
	}

	headEls := make([]*htmlutil.Element, 0, len(loadersHeadEls))
	for _, slice := range loadersHeadEls {
		headEls = append(headEls, slice...)
	}

	return &RouteResult{
		core: &RouteDataCore{
			OutermostServerError:    "",
			OutermostServerErrorIdx: nil,
			ErrorExportKeys:         cached.ErrorExportKeys,
			MatchedPatterns:         matchedPatterns,
			LoadersData:             loadersData,
			ImportURLs:              cached.ImportURLs,
			ExportKeys:              cached.ExportKeys,
			HasRootData:             hasRootData,
			Params:                  matchResults.Params,
			SplatValues:             matchResults.SplatValues,
			Deps:                    cached.Deps,
		},
		headElements: headEls,
	}
}

func (v *Vorma) getUIRouteData(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *mux.NestedRouter,
	isJSON bool,
) *RouteResult {
	res := response.New(w)
	eg := errgroup.Group{}
	defaultHeadEls := headels.New()
	var egErr error

	eg.Go(func() error {
		if v.getDefaultHeadEls != nil {
			if err := v.getDefaultHeadEls(r, v, defaultHeadEls); err != nil {
				return fmt.Errorf("GetDefaultHeadEls error: %w", err)
			}
		}
		return nil
	})

	routeResult := v.getRouteDataStage1(w, r, nestedRouter)
	egErr = eg.Wait()

	if egErr != nil {
		v.Log.Error("Error in getUIRouteData", "error", egErr.Error())
		res.InternalServerError()
		return &RouteResult{didErr: true}
	}

	if routeResult.notFound || routeResult.didRedirect || routeResult.didErr {
		return routeResult
	}

	cssBundles := v.getCSSBundles(routeResult.core.Deps)
	defaultHeadElsRaw := defaultHeadEls.Collect()

	hb := make([]*htmlutil.Element, 0, len(routeResult.headElements)+len(defaultHeadElsRaw))
	hb = append(hb, defaultHeadElsRaw...)
	hb = append(hb, routeResult.headElements...)

	publicPathPrefix := v.Wave.GetPublicPathPrefix()
	isDev := v.GetIsDevMode()

	if !isDev && !isJSON {
		if routeResult.core.Deps != nil {
			for _, dep := range routeResult.core.Deps {
				el := &htmlutil.Element{
					Tag:                 "link",
					AttributesKnownSafe: map[string]string{"rel": "modulepreload", "href": publicPathPrefix + dep},
					SelfClosing:         true,
				}
				hb = append(hb, el)
			}
		}
		for _, cssBundle := range cssBundles {
			el := &htmlutil.Element{
				Tag:                 "link",
				AttributesKnownSafe: map[string]string{"rel": "stylesheet", "href": publicPathPrefix + cssBundle},
				Attributes:          map[string]string{"data-vorma-css-bundle": cssBundle},
				SelfClosing:         true,
			}
			hb = append(hb, el)
		}
	}

	headEls := headElsInstance.ToSortedAndPreEscapedHeadEls(hb)

	return &RouteResult{
		core: routeResult.core,
		assets: &RouteAssets{
			SortedAndPreEscapedHeadEls: headEls,
			CSSBundles:                 cssBundles,
			ViteDevURL:                 v.getViteDevURL(),
		},
	}
}
