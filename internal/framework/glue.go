package vorma

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/typed"
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

	// AdHocTypes and ExtraTSCode are used for TypeScript generation.
	// These are stored in the application state so they are available
	// to the server process during fast route rebuilds.
	AdHocTypes  []*AdHocType
	ExtraTSCode string
}

// temp config holder to parse just the Vorma section
type configWrapper struct {
	Vorma *VormaConfig `json:"Vorma,omitempty"`
}

func NewVormaApp(o VormaAppConfig) *Vorma {
	var v Vorma

	v.Wave = o.Wave
	if v.Wave == nil {
		panic("Wave instance is required")
	}

	// Parse Vorma config from Wave's raw JSON
	var wrapper configWrapper
	if err := json.Unmarshal(v.Wave.RawConfigJSON(), &wrapper); err != nil {
		panic(fmt.Sprintf("failed to parse Vorma config from wave.config.json: %v", err))
	}
	if wrapper.Vorma == nil {
		// Vorma config is optional in the schema, but we need defaults or panic if critical fields missing
		wrapper.Vorma = &VormaConfig{}
	}
	v.config = wrapper.Vorma
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

	// Store TS generation options
	v._adHocTypes = o.AdHocTypes
	v._extraTSCode = o.ExtraTSCode

	v.loadersRouter = newLoadersRouter(o.LoadersRouterOptions)
	v.actionsRouter = newActionsRouter(o.ActionsRouterOptions)

	v.gmpdCache = typed.NewSyncMap[string, *cachedItemSubset]()

	return &v
}

func (v *Vorma) validateConfig() {
	if v.config.UIVariant == "" {
		panic("config: Vorma.UIVariant is required when Vorma is configured")
	}
	if v.config.HTMLTemplateLocation == "" {
		panic("config: Vorma.HTMLTemplateLocation is required")
	}
	if v.config.ClientEntry == "" {
		panic("config: Vorma.ClientEntry is required")
	}
	if v.config.ClientRouteDefsFile == "" {
		panic("config: Vorma.ClientRouteDefsFile is required")
	}
	if v.config.TSGenOutDir == "" {
		panic("config: Vorma.TSGenOutDir is required")
	}
	if v.config.BuildtimePublicURLFuncName == "" {
		v.config.BuildtimePublicURLFuncName = "waveBuildtimeURL"
	}
}

type Loaders struct{ vorma *Vorma }
type Actions struct{ vorma *Vorma }

func (v *Vorma) ServeStatic() func(http.Handler) http.Handler {
	return v.Wave.ServeStatic(true)
}

func (v *Vorma) Loaders() *Loaders { return &Loaders{vorma: v} }
func (v *Vorma) Actions() *Actions { return &Actions{vorma: v} }

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

func (v *Vorma) Build() {
	// Inject Vorma's default watch patterns before starting the build/dev server.
	// This allows Wave to handle Vorma-specific file changes without having
	// Vorma-specific knowledge hardcoded into Wave.
	v.injectDefaultWatchPatterns()

	v.Wave.BuildWaveWithHook(func(isDev bool) error {
		return v.buildInner(&buildInnerOptions{
			isDev: isDev,
		})
	})
}

// injectDefaultWatchPatterns adds Vorma's default watch patterns to Wave.
// This is called before Build() starts the dev server.
func (v *Vorma) injectDefaultWatchPatterns() {
	// Check if Vorma defaults should be included
	includeDefaults := true
	if v.config.IncludeDefaults != nil {
		includeDefaults = *v.config.IncludeDefaults
	}

	if !includeDefaults {
		return
	}

	patterns := v.getDefaultWatchPatterns()
	v.Wave.AddFrameworkWatchPatterns(patterns)
}

// getDefaultWatchPatterns returns Vorma's default watch patterns.
// These use the Strategy system to handle file changes via HTTP endpoints
// instead of requiring Wave to have Vorma-specific knowledge.
func (v *Vorma) getDefaultWatchPatterns() []wave.WatchedFile {
	patterns := []wave.WatchedFile{}

	// Route definitions file - use fast rebuild via HTTP endpoint
	clientRouteDefsFile := v.config.ClientRouteDefsFile
	if clientRouteDefsFile != "" {
		patterns = append(patterns, wave.WatchedFile{
			Pattern: clientRouteDefsFile,
			OnChangeHooks: []wave.OnChangeHook{{
				Strategy: &wave.OnChangeStrategy{
					HttpEndpoint:   DevRebuildRoutesPath,
					SkipDevHook:    true,
					SkipGoCompile:  true,
					WaitForVite:    true,
					ReloadBrowser:  true,
					FallbackAction: wave.FallbackRestartNoGo,
				},
			}},
			SkipRebuildingNotification: true,
		})
	}

	// HTML template file - use fast reload via HTTP endpoint
	htmlTemplateLocation := v.config.HTMLTemplateLocation
	privateStaticDir := v.Wave.GetPrivateStaticDir()
	if htmlTemplateLocation != "" && privateStaticDir != "" {
		templatePath := filepath.Join(privateStaticDir, htmlTemplateLocation)
		patterns = append(patterns, wave.WatchedFile{
			Pattern: templatePath,
			OnChangeHooks: []wave.OnChangeHook{{
				Strategy: &wave.OnChangeStrategy{
					HttpEndpoint:   DevReloadTemplatePath,
					SkipDevHook:    true,
					SkipGoCompile:  true,
					WaitForApp:     true,
					WaitForVite:    true,
					ReloadBrowser:  true,
					FallbackAction: wave.FallbackRestartNoGo,
				},
			}},
		})
	}

	// Go files - run DevBuildHook concurrently with Go compilation
	patterns = append(patterns, wave.WatchedFile{
		Pattern: "**/*.go",
		OnChangeHooks: []wave.OnChangeHook{{
			Cmd:    "DevBuildHook",
			Timing: wave.OnChangeStrategyConcurrent,
		}},
	})

	// Public static files - we rely on Wave's internal handling + configured regeneration of filemap.ts
	// We do NOT need a strategy here because we used v.Wave.SetPublicFileMapOutDir in initInner.

	return patterns
}

type Route[I any, O any] = mux.Route[I, O]
type TaskHandler[I any, O any] = mux.TaskHandler[I, O]
