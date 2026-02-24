// Package rendering holds SSR script and template-rendering helpers used by
// vormaruntime.
//
// Keeping these rendering details separate from runtime orchestration keeps the
// core runtime package focused on state and route flow, while this package owns
// the HTML/script generation details that are tested as one rendering unit.
package rendering

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/lab/viteutil"
	"golang.org/x/sync/errgroup"
)

// SSRInnerHTMLInput is the DTO used to render the runtime bootstrap script.
type SSRInnerHTMLInput struct {
	VormaSymbolStr   string
	IsDev            bool
	ViteDevURL       string
	BuildID          string
	RootElementID    string
	PublicPathPrefix string
	DeploymentID     string
	RouteManifestURL string

	OutermostServerError    string
	OutermostServerErrorIdx *int
	ErrorExportKeys         []string
	MatchedPatterns         []string
	LoadersData             []any
	ImportURLs              []string
	ExportKeys              []string
	HasRootData             bool
	Params                  any
	SplatValues             any
	Deps                    []string
	CSSBundles              []string
}

const ssrInnerHTMLTemplateString = `<script>
globalThis[Symbol.for("{{.VormaSymbolStr}}")] = {};
const x = globalThis[Symbol.for("{{.VormaSymbolStr}}")];
x.patternToWaitFnMap = {};
x.clientLoadersData = [];
x.isDev = {{.IsDev}};
x.viteDevURL = {{.ViteDevURL}};
x.buildID = {{.BuildID}};
x.rootElementID = "{{.RootElementID}}";
x.publicPathPrefix = "{{.PublicPathPrefix}}";
x.outermostServerError = {{.OutermostServerError}};
x.outermostServerErrorIdx = {{.OutermostServerErrorIdx}};
x.errorExportKeys = {{.ErrorExportKeys}};
x.matchedPatterns = {{.MatchedPatterns}};
x.loadersData = {{.LoadersData}};
x.importURLs = {{.ImportURLs}};
x.exportKeys = {{.ExportKeys}};
x.hasRootData = {{.HasRootData}};
x.params = {{.Params}};
x.splatValues = {{.SplatValues}};
x.deps = {{.Deps}};
x.cssBundles = {{.CSSBundles}};
x.deploymentID = {{.DeploymentID}};
x.routeManifestURL = {{.RouteManifestURL}};
</script>`

var ssrInnerTemplate = template.Must(
	template.New("ssr").Parse(ssrInnerHTMLTemplateString),
)

// BuildSSRInnerHTMLOutput contains rendered script HTML and CSP hash.
type BuildSSRInnerHTMLOutput struct {
	Script     *template.HTML
	Sha256Hash string
}

// LoadersHTMLRenderSnapshot captures template/render metadata for one response.
type LoadersHTMLRenderSnapshot struct {
	IsDevMode      bool
	ClientEntryOut string
	RootTemplate   *template.Template
}

// BodyScriptsInput configures body script rendering for loaders HTML output.
type BodyScriptsInput struct {
	RenderSnapshot   LoadersHTMLRenderSnapshot
	PublicPathPrefix string
	RefreshScript    template.HTML
	ClientEntry      string
	UseReactVariant  bool
}

// SSRRuntimeState captures runtime values used to build SSR bootstrap script
// content.
type SSRRuntimeState struct {
	VormaSymbolStr    string
	IsDev             bool
	BuildID           string
	RootElementID     string
	PublicPathPrefix  string
	RouteManifestFile string
}

// SSRRouteData captures request-specific route payload values serialized into
// the SSR bootstrap script.
type SSRRouteData struct {
	ViteDevURL string
	CSSBundles []string

	OutermostServerError    string
	OutermostServerErrorIdx *int
	ErrorExportKeys         []string
	MatchedPatterns         []string
	LoadersData             []any
	ImportURLs              []string
	ExportKeys              []string
	HasRootData             bool
	Params                  any
	SplatValues             any
	Deps                    []string
}

// BuildLoadersHTMLResponseInput captures dependencies and values needed to
// render one loaders HTML response body.
type BuildLoadersHTMLResponseInput struct {
	RenderHeadElements      func() (template.HTML, error)
	CriticalCSSStyleElement template.HTML
	StyleSheetLinkElement   template.HTML

	SSRRuntimeState SSRRuntimeState
	SSRRouteData    SSRRouteData

	RootTemplateData map[string]any

	TemplateDataKeyHeadElements  string
	TemplateDataKeyBodyScripts   string
	TemplateDataKeySSRScript     string
	TemplateDataKeySSRScriptHash string
	TemplateDataKeyRootElementID string
	ClientRootElementID          string

	BodyScriptsInput BodyScriptsInput
}

// BuildBodyScriptsForTemplate renders dev or production body scripts.
func BuildBodyScriptsForTemplate(
	input BodyScriptsInput,
) (template.HTML, error) {
	if !input.RenderSnapshot.IsDevMode {
		return template.HTML(
			fmt.Sprintf(
				`<script type="module" src="%s%s"></script>`,
				input.PublicPathPrefix,
				input.RenderSnapshot.ClientEntryOut,
			),
		), nil
	}

	devScriptOptions := viteutil.ToDevScriptsOptions{
		ClientEntry: input.ClientEntry,
	}
	if input.UseReactVariant {
		devScriptOptions.Variant = viteutil.VariantReact
	} else {
		devScriptOptions.Variant = viteutil.VariantOther
	}

	devScripts, devScriptsError := viteutil.ToDevScripts(devScriptOptions)
	if devScriptsError != nil {
		return "", devScriptsError
	}
	return devScripts + "\n" + input.RefreshScript, nil
}

// CloneTemplateDataMap returns a shallow copy of template data.
func CloneTemplateDataMap(input map[string]any) map[string]any {
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

// InjectVormaTemplateFields injects vorma-generated template values.
func InjectVormaTemplateFields(
	rootTemplateData map[string]any,
	headElements template.HTML,
	ssrScript *template.HTML,
	ssrScriptSha256Hash string,
	headElementsKey string,
	ssrScriptKey string,
	ssrScriptHashKey string,
	rootElementIDKey string,
	rootElementID string,
) {
	rootTemplateData[headElementsKey] = headElements
	rootTemplateData[ssrScriptKey] = ssrScript
	rootTemplateData[ssrScriptHashKey] = ssrScriptSha256Hash
	rootTemplateData[rootElementIDKey] = rootElementID
}

// ExecuteRootTemplate renders the final root template to bytes.
func ExecuteRootTemplate(
	rootTemplate *template.Template,
	rootTemplateData map[string]any,
) ([]byte, error) {
	if rootTemplate == nil {
		return nil, fmt.Errorf("root template is nil")
	}
	var output bytes.Buffer
	if executeError := rootTemplate.Execute(&output, rootTemplateData); executeError != nil {
		return nil, executeError
	}
	return output.Bytes(), nil
}

// BuildSSRInnerHTMLFromRuntimeState validates route payload serialization and
// builds SSR script output from runtime + route data state.
func BuildSSRInnerHTMLFromRuntimeState(
	runtimeState SSRRuntimeState,
	routeData SSRRouteData,
) (*BuildSSRInnerHTMLOutput, error) {
	for i, loaderData := range routeData.LoadersData {
		if err := json.NewEncoder(io.Discard).Encode(loaderData); err != nil {
			return nil, fmt.Errorf(
				"routeData.LoadersData[%d] must be JSON-serializable: %w",
				i,
				err,
			)
		}
	}

	input := SSRInnerHTMLInput{
		VormaSymbolStr:   runtimeState.VormaSymbolStr,
		IsDev:            runtimeState.IsDev,
		ViteDevURL:       routeData.ViteDevURL,
		BuildID:          runtimeState.BuildID,
		RootElementID:    runtimeState.RootElementID,
		PublicPathPrefix: runtimeState.PublicPathPrefix,
		RouteManifestURL: path.Join(
			runtimeState.PublicPathPrefix,
			runtimeState.RouteManifestFile,
		),
		OutermostServerError:    routeData.OutermostServerError,
		OutermostServerErrorIdx: routeData.OutermostServerErrorIdx,
		ErrorExportKeys:         routeData.ErrorExportKeys,
		MatchedPatterns:         routeData.MatchedPatterns,
		LoadersData:             routeData.LoadersData,
		ImportURLs:              routeData.ImportURLs,
		ExportKeys:              routeData.ExportKeys,
		HasRootData:             routeData.HasRootData,
		Params:                  routeData.Params,
		SplatValues:             routeData.SplatValues,
		Deps:                    routeData.Deps,
		CSSBundles:              routeData.CSSBundles,
	}

	return BuildSSRInnerHTML(input)
}

// BuildLoadersHTMLResponseBytes renders one full loaders HTML response body.
func BuildLoadersHTMLResponseBytes(
	input BuildLoadersHTMLResponseInput,
) ([]byte, error) {
	if input.RenderHeadElements == nil {
		return nil, fmt.Errorf("render head elements callback is nil")
	}
	if input.RootTemplateData == nil {
		return nil, fmt.Errorf("root template data is nil")
	}

	var renderGroup errgroup.Group
	var headElements template.HTML
	var ssrOutput *BuildSSRInnerHTMLOutput

	renderGroup.Go(func() error {
		renderedHeadElements, err := input.RenderHeadElements()
		if err != nil {
			return fmt.Errorf("render head elements: %w", err)
		}
		headElements = renderedHeadElements
		headElements += "\n" + input.CriticalCSSStyleElement
		headElements += "\n" + input.StyleSheetLinkElement
		return nil
	})

	renderGroup.Go(func() error {
		renderedSSROutput, err := BuildSSRInnerHTMLFromRuntimeState(
			input.SSRRuntimeState,
			input.SSRRouteData,
		)
		if err != nil {
			return fmt.Errorf("build SSR inner HTML: %w", err)
		}
		ssrOutput = renderedSSROutput
		return nil
	})

	if err := renderGroup.Wait(); err != nil {
		return nil, err
	}
	if ssrOutput == nil {
		return nil, fmt.Errorf("SSR render output is nil")
	}

	InjectVormaTemplateFields(
		input.RootTemplateData,
		headElements,
		ssrOutput.Script,
		ssrOutput.Sha256Hash,
		input.TemplateDataKeyHeadElements,
		input.TemplateDataKeySSRScript,
		input.TemplateDataKeySSRScriptHash,
		input.TemplateDataKeyRootElementID,
		input.ClientRootElementID,
	)

	bodyScripts, bodyScriptsBuildError := BuildBodyScriptsForTemplate(
		input.BodyScriptsInput,
	)
	if bodyScriptsBuildError != nil {
		return nil, fmt.Errorf(
			"build body scripts for template: %w",
			bodyScriptsBuildError,
		)
	}
	input.RootTemplateData[input.TemplateDataKeyBodyScripts] = bodyScripts

	return ExecuteRootTemplate(
		input.BodyScriptsInput.RenderSnapshot.RootTemplate,
		input.RootTemplateData,
	)
}

// BuildSSRInnerHTML renders the runtime bootstrap script and computes its CSP
// SHA-256 hash.
func BuildSSRInnerHTML(
	input SSRInnerHTMLInput,
) (*BuildSSRInnerHTMLOutput, error) {
	inputForRender := input
	if envutil.GetBool("VERCEL_SKEW_PROTECTION_ENABLED", false) {
		inputForRender.DeploymentID = envutil.GetStr(
			"VERCEL_DEPLOYMENT_ID",
			"",
		)
	}

	var htmlBuilder strings.Builder
	if executeError := ssrInnerTemplate.Execute(
		&htmlBuilder,
		inputForRender,
	); executeError != nil {
		return nil, fmt.Errorf(
			"could not execute SSR inner HTML template: %w",
			executeError,
		)
	}

	innerHTML := htmlBuilder.String()
	innerHTML = strings.TrimPrefix(innerHTML, "<script>")
	innerHTML = strings.TrimSuffix(innerHTML, "</script>")

	scriptElement := htmlutil.Element{
		Tag:                 "script",
		AttributesKnownSafe: map[string]string{"type": "module"},
		DangerousInnerHTML:  innerHTML,
	}

	sha256Hash, hashError := htmlutil.ComputeContentSha256(&scriptElement)
	if hashError != nil {
		return nil, fmt.Errorf("could not compute CSP hash: %w", hashError)
	}

	renderedElement, renderError := htmlutil.RenderElement(&scriptElement)
	if renderError != nil {
		return nil, fmt.Errorf(
			"could not render SSR inner HTML: %w",
			renderError,
		)
	}

	return &BuildSSRInnerHTMLOutput{
		Script:     &renderedElement,
		Sha256Hash: sha256Hash,
	}, nil
}
