package vormaruntime

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/htmlutil"
)

type SSRInnerHTMLInput struct {
	VormaSymbolStr   string
	IsDev            bool
	ViteDevURL       string
	BuildID          string
	PublicPathPrefix string
	DeploymentID     string
	RouteManifestURL string
	*RouteDataCore
	CSSBundles []string
}

const ssrInnerHTMLTmplStr = `<script>
globalThis[Symbol.for("{{.VormaSymbolStr}}")] = {};
const x = globalThis[Symbol.for("{{.VormaSymbolStr}}")];
x.patternToWaitFnMap = {};
x.clientLoadersData = [];
x.isDev = {{.IsDev}};
x.viteDevURL = {{.ViteDevURL}};
x.buildID = {{.BuildID}};
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

var ssrInnerTmpl = template.Must(template.New("ssr").Parse(ssrInnerHTMLTmplStr))

type GetSSRInnerHTMLOutput struct {
	Script     *template.HTML
	Sha256Hash string
}

type ssrRuntimeSnapshot struct {
	isDev             bool
	buildID           string
	routeManifestFile string
}

func (v *Vorma) getSSRInnerHTML(routeData *RouteDataFinal) (*GetSSRInnerHTMLOutput, error) {
	v.mu.RLock()
	snapshot := ssrRuntimeSnapshot{
		isDev:             v._isDev,
		buildID:           v._buildID,
		routeManifestFile: v._routeManifestFile,
	}
	v.mu.RUnlock()

	return v.getSSRInnerHTMLFromSnapshot(routeData, snapshot)
}

func (v *Vorma) getSSRInnerHTMLFromSnapshot(
	routeData *RouteDataFinal,
	snapshot ssrRuntimeSnapshot,
) (*GetSSRInnerHTMLOutput, error) {
	if routeData == nil {
		return nil, fmt.Errorf("routeData cannot be nil")
	}
	if routeData.RouteDataCore == nil {
		return nil, fmt.Errorf("routeData.RouteDataCore cannot be nil")
	}
	for i, loaderData := range routeData.RouteDataCore.LoadersData {
		if err := json.NewEncoder(io.Discard).Encode(loaderData); err != nil {
			return nil, fmt.Errorf("routeData.LoadersData[%d] must be JSON-serializable: %w", i, err)
		}
	}

	var htmlBuilder strings.Builder
	publicPathPrefix := v.Wave.GetPublicPathPrefix()

	dto := SSRInnerHTMLInput{
		VormaSymbolStr:   VormaSymbolStr,
		IsDev:            snapshot.isDev,
		ViteDevURL:       routeData.ViteDevURL,
		BuildID:          snapshot.buildID,
		PublicPathPrefix: publicPathPrefix,
		RouteManifestURL: path.Join(publicPathPrefix, snapshot.routeManifestFile),
		RouteDataCore:    routeData.RouteDataCore,
		CSSBundles:       routeData.CSSBundles,
	}

	if envutil.GetBool("VERCEL_SKEW_PROTECTION_ENABLED", false) {
		dto.DeploymentID = envutil.GetStr("VERCEL_DEPLOYMENT_ID", "")
	}

	if err := ssrInnerTmpl.Execute(&htmlBuilder, dto); err != nil {
		return nil, fmt.Errorf("could not execute SSR inner HTML template: %w", err)
	}

	innerHTML := htmlBuilder.String()
	innerHTML = strings.TrimPrefix(innerHTML, "<script>")
	innerHTML = strings.TrimSuffix(innerHTML, "</script>")

	el := htmlutil.Element{
		Tag:                 "script",
		AttributesKnownSafe: map[string]string{"type": "module"},
		DangerousInnerHTML:  innerHTML,
	}

	sha256Hash, err := htmlutil.ComputeContentSha256(&el)
	if err != nil {
		return nil, fmt.Errorf("could not compute CSP hash: %w", err)
	}

	renderedEl, err := htmlutil.RenderElement(&el)
	if err != nil {
		return nil, fmt.Errorf("could not render SSR inner HTML: %w", err)
	}

	return &GetSSRInnerHTMLOutput{Script: &renderedEl, Sha256Hash: sha256Hash}, nil
}
