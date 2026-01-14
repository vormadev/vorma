package vormaruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/validate"
	"github.com/vormadev/vorma/lab/tsgen"
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
	DynamicParamPrefix     rune
	SplatSegmentIdentifier rune
	IndexSegmentIdentifier string
}

type ActionsRouterOptions struct {
	DynamicParamPrefix     rune
	SplatSegmentIdentifier rune
	MountRoot              string
	SupportedMethods       []string
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

// FormData is used as the input type for actions that accept form data.
type FormData struct{}

func (m FormData) TSTypeRaw() string { return "FormData" }

type VormaAppConfig struct {
	Wave                 *wave.Wave
	GetDefaultHeadEls    GetDefaultHeadElsFunc
	GetHeadElUniqueRules GetHeadElUniqueRulesFunc
	GetRootTemplateData  GetRootTemplateDataFunc
	LoadersRouterOptions LoadersRouterOptions
	ActionsRouterOptions ActionsRouterOptions
	AdHocTypes           []*tsgen.AdHocType
	ExtraTSCode          string
}

type configWrapper struct {
	Vorma *VormaConfig `json:"Vorma,omitempty"`
}

func NewVormaApp(o VormaAppConfig) *Vorma {
	var v Vorma

	v.Wave = o.Wave
	if v.Wave == nil {
		panic("Wave instance is required")
	}

	var wrapper configWrapper
	if err := json.Unmarshal(v.Wave.RawConfigJSON(), &wrapper); err != nil {
		panic(fmt.Sprintf("failed to parse Vorma config: %v", err))
	}
	if wrapper.Vorma == nil {
		wrapper.Vorma = &VormaConfig{}
	}
	v.Config = wrapper.Vorma
	v.validateConfig()

	v.getDefaultHeadEls = o.GetDefaultHeadEls
	if v.getDefaultHeadEls == nil {
		v.getDefaultHeadEls = func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
			return nil
		}
	}
	v.getHeadElUniqueRules = o.GetHeadElUniqueRules
	if v.getHeadElUniqueRules == nil {
		v.getHeadElUniqueRules = func(h *headels.HeadEls) {}
	}
	v.getRootTemplateData = o.GetRootTemplateData
	if v.getRootTemplateData == nil {
		v.getRootTemplateData = func(r *http.Request) (map[string]any, error) {
			return map[string]any{}, nil
		}
	}

	v._adHocTypes = o.AdHocTypes
	v._extraTSCode = o.ExtraTSCode
	v.loadersRouter = newLoadersRouter(o.LoadersRouterOptions)
	v.actionsRouter = newActionsRouter(o.ActionsRouterOptions)

	return &v
}

func (v *Vorma) validateConfig() {
	if v.Config.UIVariant == "" {
		panic("config: Vorma.UIVariant is required")
	}
	if v.Config.HTMLTemplateLocation == "" {
		panic("config: Vorma.HTMLTemplateLocation is required")
	}
	if v.Config.ClientEntry == "" {
		panic("config: Vorma.ClientEntry is required")
	}
	if v.Config.ClientRouteDefsFile == "" {
		panic("config: Vorma.ClientRouteDefsFile is required")
	}
	if v.Config.TSGenOutDir == "" {
		panic("config: Vorma.TSGenOutDir is required")
	}
	if v.Config.BuildtimePublicURLFuncName == "" {
		v.Config.BuildtimePublicURLFuncName = "waveBuildtimeURL"
	}
}

type Loaders struct{ vorma *Vorma }
type Actions struct{ vorma *Vorma }

func (v *Vorma) ServeStatic() func(http.Handler) http.Handler {
	return v.Wave.ServeStatic(true)
}

func (v *Vorma) Loaders() *Loaders { return &Loaders{vorma: v} }
func (v *Vorma) Actions() *Actions { return &Actions{vorma: v} }

func (h *Loaders) HandlerMountPattern() string { return "/*" }
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

type Route[I any, O any] = mux.Route[I, O]
type TaskHandler[I any, O any] = mux.TaskHandler[I, O]
