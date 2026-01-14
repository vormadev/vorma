package runtime

import (
	"fmt"
	"net/http"

	"github.com/vormadev/vorma/fw/types"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
	"golang.org/x/sync/errgroup"
)

type SplatValues []string

type ui_data_core struct {
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

type ui_data_stage_2 struct {
	SortedAndPreEscapedHeadEls *headels.SortedAndPreEscapedHeadEls
	CSSBundles                 []string
	ViteDevURL                 string
}

type ui_data_all struct {
	notFound         bool
	didRedirect      bool
	didErr           bool
	ui_data_core     *ui_data_core
	stage_1_head_els []*htmlutil.Element
	state_2_final    *ui_data_stage_2
}

type final_ui_data struct {
	*ui_data_core
	Title      *htmlutil.Element   `json:"title,omitempty"`
	Meta       []*htmlutil.Element `json:"metaHeadEls,omitempty"`
	Rest       []*htmlutil.Element `json:"restHeadEls,omitempty"`
	CSSBundles []string            `json:"cssBundles,omitempty"`
	ViteDevURL string              `json:"viteDevURL,omitempty"`
}

func (v *Vorma) get_ui_data_stage_1(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *mux.NestedRouter,
) *ui_data_all {
	realPath := matcher.StripTrailingSlash(r.URL.Path)
	if realPath == "" {
		realPath = "/"
	}

	_match_results, found := mux.FindNestedMatches(nestedRouter, r)
	if !found {
		return &ui_data_all{notFound: true}
	}

	_matches := _match_results.Matches
	matchedPatterns := make([]string, len(_matches))
	for i, match := range _matches {
		matchedPatterns[i] = match.OriginalPattern()
	}

	paths := v.GetPathsSnapshot()
	isDev := v.GetIsDevMode()

	importURLs := make([]string, 0, len(_matches))
	exportKeys := make([]string, 0, len(_matches))
	errorExportKeys := make([]string, 0, len(_matches))

	for _, path := range _matches {
		foundPath := paths[path.OriginalPattern()]
		if foundPath == nil || foundPath.SrcPath == "" {
			importURLs = append(importURLs, "")
			exportKeys = append(exportKeys, "")
			errorExportKeys = append(errorExportKeys, "")
			continue
		}
		pathToUse := foundPath.OutPath
		if isDev {
			pathToUse = foundPath.SrcPath
		}
		importURLs = append(importURLs, "/"+pathToUse)
		exportKeys = append(exportKeys, foundPath.ExportKey)
		errorExportKeys = append(errorExportKeys, foundPath.ErrorExportKey)
	}
	deps := v.getDepsFromSnapshot(_matches, paths)

	_tasks_results := mux.RunNestedTasks(nestedRouter, r, _match_results)

	var hasRootData bool
	if len(_match_results.Matches) > 0 &&
		_match_results.Matches[0].NormalizedPattern() == "" &&
		_tasks_results.GetHasTaskHandler(0) {
		hasRootData = true
	}

	_merged_response_proxy := response.MergeProxyResponses(_tasks_results.ResponseProxies...)
	if _merged_response_proxy != nil {
		_merged_response_proxy.ApplyToResponseWriter(w, r)
		if _merged_response_proxy.IsError() {
			return &ui_data_all{didErr: true}
		}
		if _merged_response_proxy.IsRedirect() {
			return &ui_data_all{didRedirect: true}
		}
	}

	var numberOfLoaders int
	if _match_results != nil {
		numberOfLoaders = len(_match_results.Matches)
	}

	loadersData := make([]any, numberOfLoaders)
	loadersErrs := make([]error, numberOfLoaders)

	if numberOfLoaders > 0 {
		for i, result := range _tasks_results.Slice {
			if result != nil {
				loadersData[i] = result.Data()
				loadersErrs[i] = result.Err()

				if result.RanTask() && loadersErrs[i] == nil {
					shouldWarn := reflectutil.ExcludingNoneGetIsNilOrUltimatelyPointsToNil(loadersData[i])
					if shouldWarn {
						Log.Warn("Do not return nil values from loaders unless the underlying type is an empty struct or you are returning an error.",
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

	loadersHeadEls := make([][]*htmlutil.Element, 0, numberOfLoaders)
	for _, _response_proxy := range _tasks_results.ResponseProxies {
		if _response_proxy != nil {
			loadersHeadEls = append(loadersHeadEls, _response_proxy.GetHeadEls().Collect())
		}
	}

	if outermostErrorIdx != nil {
		derefIdx := *outermostErrorIdx
		err := loadersErrs[derefIdx]
		pattern := matchedPatterns[derefIdx]

		var clientMsg string
		var errToLog error

		if loaderErr, ok := err.(types.LoaderErrorMarker); ok {
			clientMsg = loaderErr.ClientMessage()
			errToLog = loaderErr.ServerError()
		} else {
			clientMsg = "An error occurred"
			errToLog = err
			Log.Warn("Sending generic error to client. Use vorma.LoaderError for custom client messages.")
		}

		if errToLog != nil {
			Log.Error("loader error", "pattern", pattern, "error", errToLog)
		}

		headElsSlice := loadersHeadEls[:derefIdx]
		headEls := make([]*htmlutil.Element, 0)
		for _, slice := range headElsSlice {
			headEls = append(headEls, slice...)
		}

		cutIdx := derefIdx + 1
		return &ui_data_all{
			ui_data_core: &ui_data_core{
				OutermostServerError:    clientMsg,
				OutermostServerErrorIdx: outermostErrorIdx,
				ErrorExportKeys:         errorExportKeys[:cutIdx],
				MatchedPatterns:         matchedPatterns[:cutIdx],
				LoadersData:             loadersData[:cutIdx],
				ImportURLs:              importURLs[:cutIdx],
				ExportKeys:              exportKeys[:cutIdx],
				HasRootData:             hasRootData,
				Params:                  _match_results.Params,
				SplatValues:             _match_results.SplatValues,
				Deps:                    deps,
			},
			stage_1_head_els: headEls,
		}
	}

	headEls := make([]*htmlutil.Element, 0, len(loadersHeadEls))
	for _, slice := range loadersHeadEls {
		headEls = append(headEls, slice...)
	}

	return &ui_data_all{
		ui_data_core: &ui_data_core{
			OutermostServerError:    "",
			OutermostServerErrorIdx: nil,
			ErrorExportKeys:         errorExportKeys,
			MatchedPatterns:         matchedPatterns,
			LoadersData:             loadersData,
			ImportURLs:              importURLs,
			ExportKeys:              exportKeys,
			HasRootData:             hasRootData,
			Params:                  _match_results.Params,
			SplatValues:             _match_results.SplatValues,
			Deps:                    deps,
		},
		stage_1_head_els: headEls,
	}
}

func (v *Vorma) getUIRouteData(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *mux.NestedRouter,
	isJSON bool,
) *ui_data_all {
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

	uiRoutesData := v.get_ui_data_stage_1(w, r, nestedRouter)
	egErr = eg.Wait()

	if egErr != nil {
		Log.Error("Error in getUIRouteData", "error", egErr.Error())
		res.InternalServerError()
		return &ui_data_all{didErr: true}
	}

	if uiRoutesData.notFound || uiRoutesData.didRedirect || uiRoutesData.didErr {
		return uiRoutesData
	}

	cssBundles := v.getCSSBundles(uiRoutesData.ui_data_core.Deps)
	defaultHeadElsRaw := defaultHeadEls.Collect()

	hb := make([]*htmlutil.Element, 0, len(uiRoutesData.stage_1_head_els)+len(defaultHeadElsRaw))
	hb = append(hb, defaultHeadElsRaw...)
	hb = append(hb, uiRoutesData.stage_1_head_els...)

	publicPathPrefix := v.Wave.GetPublicPathPrefix()
	isDev := v.GetIsDevMode()

	if !isDev && !isJSON {
		if uiRoutesData.ui_data_core.Deps != nil {
			for _, dep := range uiRoutesData.ui_data_core.Deps {
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

	return &ui_data_all{
		ui_data_core: uiRoutesData.ui_data_core,
		state_2_final: &ui_data_stage_2{
			SortedAndPreEscapedHeadEls: headEls,
			CSSBundles:                 cssBundles,
			ViteDevURL:                 v.getViteDevURL(),
		},
	}
}
