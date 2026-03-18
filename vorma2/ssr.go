package vorma2

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/envutil"
)

const vorma_symbol_str = "__vorma_internal__"

const ssr_template_str = `<script type="module">
globalThis[Symbol.for("{{.VormaSymbol}}")] = {};
const x = globalThis[Symbol.for("{{.VormaSymbol}}")];
x.patternToWaitFnMap = {};
x.isDev = {{.IsDev}};
x.viteDevURL = {{.ViteDevURL}};
x.publicPathPrefix = "{{.PublicPathPrefix}}";
x.deploymentID = {{.DeploymentID}};
x.routeManifestURL = {{.RouteManifestURL}};
x.runtimeRouteSnapshot = {
	outermostServerError: {{.OutermostServerError}},
	outermostServerErrorIdx: {{.OutermostServerErrorIdx}},
	matchedPatterns: {{.MatchedPatterns}},
	loadersData: {{.LoadersDataJSON}},
	importURLs: {{.ImportURLs}},
	exportKeys: {{.ExportKeys}},
	errorExportKeys: {{.ErrorExportKeys}},
	hasRootData: {{.HasRootData}},
	params: {{.Params}},
	splatValues: {{.SplatValues}},
	outermostClientError: undefined,
	outermostClientErrorIdx: undefined,
	outermostError: {{.OutermostServerError}},
	outermostErrorIdx: {{.OutermostServerErrorIdx}},
	buildID: {{.BuildID}},
	rootElementID: "{{.RootElementID}}",
	activeComponents: null,
	activeErrorBoundary: undefined,
	clientLoadersData: [],
};
</script>`

var ssr_template = template.Must(
	template.New("ssr").Parse(ssr_template_str),
)

type ssr_input struct {
	VormaSymbol             string
	IsDev                   bool
	ViteDevURL              string
	PublicPathPrefix        string
	DeploymentID            string
	RouteManifestURL        string
	BuildID                 string
	RootElementID           string
	OutermostServerError    string
	OutermostServerErrorIdx *int
	ErrorExportKeys         []string
	MatchedPatterns         []string
	LoadersDataJSON         template.JS
	ImportURLs              []string
	ExportKeys              []string
	HasRootData             bool
	Params                  any
	SplatValues             any
}

type ssr_output struct {
	script_html template.HTML
	csp_hash    string
}

func build_ssr_script(input ssr_input) (*ssr_output, error) {
	if input.Params == nil {
		input.Params = map[string]string{}
	}
	if input.SplatValues == nil {
		input.SplatValues = []string{}
	}

	if envutil.GetBool("VERCEL_SKEW_PROTECTION_ENABLED", false) {
		input.DeploymentID = envutil.GetStr("VERCEL_DEPLOYMENT_ID", "")
	}

	var buf bytes.Buffer
	if err := ssr_template.Execute(&buf, input); err != nil {
		return nil, fmt.Errorf("executing SSR template: %w", err)
	}

	// extract inner HTML (strip wrapping <script> tags) for CSP hash
	raw := buf.String()
	inner := raw
	inner = strings.TrimPrefix(inner, `<script type="module">`)
	inner = strings.TrimSuffix(inner, "</script>")

	hash := bytesutil.ToBase64(cryptoutil.Sha256Hash([]byte(inner)))
	return &ssr_output{
		script_html: template.HTML(raw),
		csp_hash:    hash,
	}, nil
}
