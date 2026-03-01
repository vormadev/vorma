// Package muxcore provides shared internals used by mux and nestedmux.
//
// It centralizes matcher option normalization and request-scoped transport data
// plumbing so mux and nestedmux do not duplicate these mechanics.
package muxcore

import (
	"net/http"

	"github.com/vormadev/vorma/kit/contextutil"
	"github.com/vormadev/vorma/kit/genericsutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/nestedmatcher"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/tasks"
)

var (
	requestStore = contextutil.NewStore[*RequestData](
		"__vorma_kit_mux_request_data",
	)
	emptySplatValues = []string{}
)

// RequestData stores request-scoped route and task context metadata.
type RequestData struct {
	params        matcher.Params
	splatValues   []string
	tasksCtx      *tasks.Ctx
	request       *http.Request
	responseProxy *response.Proxy
}

// NewRequestData constructs request-scoped transport data.
func NewRequestData(
	params matcher.Params,
	splatValues []string,
	tasksCtx *tasks.Ctx,
	request *http.Request,
	responseProxy *response.Proxy,
) *RequestData {
	return &RequestData{
		params:        params,
		splatValues:   splatValues,
		tasksCtx:      tasksCtx,
		request:       request,
		responseProxy: responseProxy,
	}
}

// NewRequestDataWithTasksCtxOnly constructs request data that only carries a
// tasks context for nested task execution outside mux route matching.
func NewRequestDataWithTasksCtxOnly(
	request *http.Request,
	tasksCtx *tasks.Ctx,
) *RequestData {
	return &RequestData{
		params:        nil,
		splatValues:   emptySplatValues,
		tasksCtx:      tasksCtx,
		request:       request,
		responseProxy: nil,
	}
}

// Params returns route params.
func (rd *RequestData) Params() matcher.Params {
	if rd == nil {
		return nil
	}
	return rd.params
}

// SplatValues returns route splat values.
func (rd *RequestData) SplatValues() []string {
	if rd == nil {
		return nil
	}
	return rd.splatValues
}

// TasksCtx returns the request task context.
func (rd *RequestData) TasksCtx() *tasks.Ctx {
	if rd == nil {
		return nil
	}
	return rd.tasksCtx
}

// Request returns the backing request.
func (rd *RequestData) Request() *http.Request {
	if rd == nil {
		return nil
	}
	return rd.request
}

// ResponseProxy returns the response proxy.
func (rd *RequestData) ResponseProxy() *response.Proxy {
	if rd == nil {
		return nil
	}
	return rd.responseProxy
}

// RequestWithData returns a request carrying request-scoped transport data.
func RequestWithData(
	request *http.Request,
	requestData *RequestData,
) *http.Request {
	if request == nil {
		return nil
	}
	if requestData == nil {
		return request
	}
	return requestStore.RequestWithContextValue(request, requestData)
}

// DataFromRequest returns request-scoped transport data when present.
func DataFromRequest(request *http.Request) *RequestData {
	if request == nil {
		return nil
	}
	return requestStore.Value(request.Context())
}

// RequestWithTasksCtx returns a request carrying only tasks context metadata.
func RequestWithTasksCtx(
	request *http.Request,
	tasksCtx *tasks.Ctx,
) *http.Request {
	if request == nil {
		return nil
	}
	return RequestWithData(
		request,
		NewRequestDataWithTasksCtxOnly(request, tasksCtx),
	)
}

// GetTasksCtx returns request tasks context when present.
func GetTasksCtx(request *http.Request) *tasks.Ctx {
	requestData := DataFromRequest(request)
	if requestData == nil {
		return nil
	}
	return requestData.TasksCtx()
}

// GetParams returns route params when present.
func GetParams(request *http.Request) matcher.Params {
	requestData := DataFromRequest(request)
	if requestData == nil {
		return nil
	}
	if requestData.Params() == nil {
		return nil
	}
	return requestData.Params()
}

// GetParam returns one route param by key.
func GetParam(request *http.Request, key string) string {
	params := GetParams(request)
	if params == nil {
		return ""
	}
	return params[key]
}

// GetSplatValues returns route splat values when present.
func GetSplatValues(request *http.Request) []string {
	requestData := DataFromRequest(request)
	if requestData == nil {
		return emptySplatValues
	}
	splatValues := requestData.SplatValues()
	if splatValues == nil {
		return emptySplatValues
	}
	return splatValues
}

// MatcherOptionsInput configures plain matcher options used by mux.
type MatcherOptionsInput struct {
	DynamicParamPrefix     rune
	SplatSegmentIdentifier rune
	Quiet                  bool
}

// BuildMatcherOptions normalizes plain matcher option defaults.
func BuildMatcherOptions(input MatcherOptionsInput) *matcher.Options {
	return &matcher.Options{
		DynamicParamPrefix: genericsutil.OrDefault(
			input.DynamicParamPrefix,
			':',
		),
		SplatSegmentIdentifier: genericsutil.OrDefault(
			input.SplatSegmentIdentifier,
			'*',
		),
		Quiet: input.Quiet,
	}
}

// NestedMatcherOptionsInput configures nested matcher options used by nestedmux.
type NestedMatcherOptionsInput struct {
	DynamicParamPrefix             rune
	SplatSegmentIdentifier         rune
	ExplicitIndexSegmentIdentifier string
	Quiet                          bool
}

// BuildNestedMatcherOptions normalizes nested matcher option defaults.
func BuildNestedMatcherOptions(
	input NestedMatcherOptionsInput,
) *nestedmatcher.Options {
	return &nestedmatcher.Options{
		DynamicParamPrefix: genericsutil.OrDefault(
			input.DynamicParamPrefix,
			':',
		),
		SplatSegmentIdentifier: genericsutil.OrDefault(
			input.SplatSegmentIdentifier,
			'*',
		),
		ExplicitIndexSegmentIdentifier: genericsutil.OrDefault(
			input.ExplicitIndexSegmentIdentifier,
			"",
		),
		Quiet: input.Quiet,
	}
}

// NormalizeMountRoot canonicalizes mux mount roots.
func NormalizeMountRoot(mountRoot string) string {
	if mountRoot == "" {
		return ""
	}
	if len(mountRoot) == 1 && mountRoot[0] == '/' {
		return ""
	}
	if len(mountRoot) > 1 && mountRoot[0] != '/' {
		mountRoot = "/" + mountRoot
	}
	if len(mountRoot) > 0 && mountRoot[len(mountRoot)-1] != '/' {
		mountRoot = mountRoot + "/"
	}
	return mountRoot
}
