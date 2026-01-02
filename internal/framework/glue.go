package vorma

import (
	"errors"
	"mime"
	"net/http"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/validate"
	"github.com/vormadev/vorma/wave"
)

type LoadersRouter struct {
	*mux.NestedRouter
}
type ActionsRouter struct {
	*mux.Router
	supportedMethods map[string]bool
}
type LoaderReqData = mux.NestedReqData
type ActionReqData[I any] = mux.ReqData[I]

type LoadersRouterOptions struct {
	// Default: ':' (e.g., /user/:id)
	DynamicParamPrefix rune
	// Default: '*' (e.g., /files/*)
	SplatSegmentIdentifier rune
	// Default: "_index" (e.g., /blog/_index)
	IndexSegmentIdentifier string
}

type ActionsRouterOptions struct {
	// Default: ':' (e.g., /user/:id)
	DynamicParamPrefix rune
	// Default: '*' (e.g., /files/*)
	SplatSegmentIdentifier rune
	// Default: "/api/"
	MountRoot string
	// Default: []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	SupportedMethods []string
}

func newLoadersRouter(options ...LoadersRouterOptions) *LoadersRouter {
	var o LoadersRouterOptions
	if len(options) > 0 {
		o = options[0]
	}
	explicitIndexSegment := o.IndexSegmentIdentifier
	if explicitIndexSegment == "" {
		explicitIndexSegment = "_index"
	}

	return &LoadersRouter{
		NestedRouter: mux.NewNestedRouter(&mux.NestedOptions{
			DynamicParamPrefixRune: o.DynamicParamPrefix,
			SplatSegmentRune:       o.SplatSegmentIdentifier,
			ExplicitIndexSegment:   explicitIndexSegment,
		}),
	}
}

func newActionsRouter(options ...ActionsRouterOptions) *ActionsRouter {
	var o ActionsRouterOptions
	if len(options) > 0 {
		o = options[0]
	}

	mountRoot := o.MountRoot
	if mountRoot == "" {
		mountRoot = "/api/"
	}

	supportedMethods := make(map[string]bool, len(o.SupportedMethods))
	if len(o.SupportedMethods) == 0 {
		supportedMethods["GET"] = true
		supportedMethods["POST"] = true
		supportedMethods["PUT"] = true
		supportedMethods["DELETE"] = true
		supportedMethods["PATCH"] = true
	} else {
		for _, m := range o.SupportedMethods {
			supportedMethods[m] = true
		}
	}

	return &ActionsRouter{
		Router: mux.NewRouter(&mux.Options{
			DynamicParamPrefixRune: o.DynamicParamPrefix,
			SplatSegmentRune:       o.SplatSegmentIdentifier,
			MountRoot:              mountRoot,
			ParseInput: func(r *http.Request, iPtr any) error {
				if r.Method == http.MethodGet {
					return validate.URLSearchParamsInto(r, iPtr)
				}
				if supportedMethods[r.Method] {
					contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
					if contentType == "application/x-www-form-urlencoded" ||
						contentType == "multipart/form-data" {
						return nil
					}
					return validate.JSONBodyInto(r, iPtr)
				}
				return errors.New("unsupported method")
			},
		}),
		supportedMethods: supportedMethods,
	}
}

type FormData struct{}

func (m FormData) TSTypeRaw() string { return "FormData" }

type VormaAppConfig struct {
	Wave *wave.Wave

	GetDefaultHeadEls    GetDefaultHeadElsFunc
	GetHeadElUniqueRules GetHeadElUniqueRulesFunc
	GetRootTemplateData  GetRootTemplateDataFunc

	LoadersRouterOptions LoadersRouterOptions
	ActionsRouterOptions ActionsRouterOptions
}

func NewVormaApp(o VormaAppConfig) *Vorma {
	var v Vorma

	v.Wave = o.Wave
	if v.Wave == nil {
		panic("Wave instance is required")
	}

	v.getDefaultHeadEls = o.GetDefaultHeadEls
	if v.getDefaultHeadEls == nil {
		v.getDefaultHeadEls = func(r *http.Request, app *Vorma) (*headels.HeadEls, error) {
			return headels.New(), nil
		}
	}

	v.getHeadElUniqueRules = o.GetHeadElUniqueRules
	if v.getHeadElUniqueRules == nil {
		v.getHeadElUniqueRules = func() *headels.HeadEls {
			return headels.New()
		}
	}

	v.getRootTemplateData = o.GetRootTemplateData
	if v.getRootTemplateData == nil {
		v.getRootTemplateData = func(r *http.Request) (map[string]any, error) {
			return map[string]any{}, nil
		}
	}

	v.loadersRouter = newLoadersRouter(o.LoadersRouterOptions)
	v.actionsRouter = newActionsRouter(o.ActionsRouterOptions)

	return &v
}

type Loaders struct{ vorma *Vorma }
type Actions struct{ vorma *Vorma }

func (h *Vorma) ServeStatic() func(http.Handler) http.Handler {
	return h.Wave.ServeStatic(true)
}

func (h *Vorma) Loaders() *Loaders { return &Loaders{vorma: h} }
func (h *Vorma) Actions() *Actions { return &Actions{vorma: h} }

func (h *Loaders) HandlerMountPattern() string {
	return "/*"
}
func (h *Loaders) Handler() http.Handler {
	return h.vorma.GetLoadersHandler(h.vorma.LoadersRouter().NestedRouter)
}

func (h *Actions) HandlerMountPattern() string {
	return h.vorma.ActionsRouter().MountRoot("*")
}
func (h *Actions) Handler() http.Handler {
	return h.vorma.GetActionsHandler(h.vorma.ActionsRouter().Router)
}
func (h *Actions) SupportedMethods() map[string]bool {
	return h.vorma.ActionsRouter().supportedMethods
}

type BuildOptions struct {
	AdHocTypes  []*AdHocType
	ExtraTSCode string
}

func (h *Vorma) Build(o ...BuildOptions) {
	var opts BuildOptions
	if len(o) > 0 {
		opts = o[0]
	}
	h.Wave.BuildWaveWithHook(func(isDev bool) error {
		return h.buildInner(&buildInnerOptions{
			isDev:        isDev,
			buildOptions: &opts,
		})
	})
}

type Route[I any, O any] = mux.Route[I, O]
type TaskHandler[I any, O any] = mux.TaskHandler[I, O]
