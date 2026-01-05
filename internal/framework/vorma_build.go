package vorma

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/stringsutil"
	"github.com/vormadev/vorma/kit/tsgen"
	"github.com/vormadev/vorma/kit/viteutil"
)

const (
	vormaOutPrefix                 = "vorma_out_"
	vormaVitePrehashedFilePrefix   = vormaOutPrefix + "vite_"
	vormaRouteManifestPrefix       = vormaOutPrefix + "vorma_internal_route_manifest_"
	VormaPathsStageOneJSONFileName = "vorma_paths_stage_1.json"
	VormaPathsStageTwoJSONFileName = "vorma_paths_stage_2.json"
)

type PathsFile struct {
	// both stages one and two
	Stage             string           `json:"stage"`
	BuildID           string           `json:"buildID,omitempty"`
	ClientEntrySrc    string           `json:"clientEntrySrc"`
	Paths             map[string]*Path `json:"paths"`
	RouteManifestFile string           `json:"routeManifestFile"`

	// stage two only
	ClientEntryOut    string            `json:"clientEntryOut,omitempty"`
	ClientEntryDeps   []string          `json:"clientEntryDeps,omitempty"`
	DepToCSSBundleMap map[string]string `json:"depToCSSBundleMap,omitempty"`
}

func (v *Vorma) writePathsToDisk_StageOne() error {
	pathsJSONOut_StageOne := filepath.Join(
		v.Wave.GetStaticPrivateOutDir(),
		"vorma_out",
		VormaPathsStageOneJSONFileName,
	)
	err := os.MkdirAll(filepath.Dir(pathsJSONOut_StageOne), os.ModePerm)
	if err != nil {
		return err
	}

	pathsAsJSON, err := json.MarshalIndent(PathsFile{
		Stage:             "one",
		Paths:             v._paths,
		ClientEntrySrc:    v.Wave.GetVormaClientEntry(),
		BuildID:           v._buildID,
		RouteManifestFile: v._routeManifestFile,
	}, "", "\t")
	if err != nil {
		return err
	}

	err = os.WriteFile(pathsJSONOut_StageOne, pathsAsJSON, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

var (
	reactDedupeList = []string{
		"react", "react-dom",
	}
	preactDedupeList = []string{
		"preact", "preact/hooks",
		"@preact/signals",
		"preact/jsx-runtime", "preact/compat", "preact/test-utils",
	}
	solidDedupeList = []string{
		"solid-js", "solid-js/web",
	}
)

const vitePluginTemplateStr = `
export const staticPublicAssetMap = {{.StaticPublicAssetMapJSON}} as const;

export type StaticPublicAsset = keyof typeof staticPublicAssetMap;

declare global {
	function {{.FuncName}}(
		staticPublicAsset: StaticPublicAsset,
	): string;
}

export const publicPathPrefix = "{{.PublicPathPrefix}}";

export function waveRuntimeURL(
	originalPublicURL: StaticPublicAsset,
) {
	const url = staticPublicAssetMap[originalPublicURL] ?? originalPublicURL;
	return publicPathPrefix + url;
}

export const vormaViteConfig = {
	rollupInput: [{{range $i, $e := .Entrypoints}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
	publicPathPrefix,
	staticPublicAssetMap,
	buildtimePublicURLFuncName: "{{.FuncName}}",
	ignoredPatterns: [{{range $i, $e := .IgnoredPatterns}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
	dedupeList: [{{range $i, $e := .DedupeList}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
} as const;
`

var vitePluginTemplate = template.Must(template.New("vitePlugin").Parse(vitePluginTemplateStr))

func (v *Vorma) toRollupOptions(entrypoints []string, fileMap map[string]string) (string, error) {
	var sb stringsutil.Builder

	sb.Return()
	sb.Write(tsgen.Comment("Vorma Vite Config:"))
	sb.Return()

	var dedupeList []string
	switch UIVariant(v.Wave.GetVormaUIVariant()) {
	case UIVariants.React:
		dedupeList = reactDedupeList
	case UIVariants.Preact:
		dedupeList = preactDedupeList
	case UIVariants.Solid:
		dedupeList = solidDedupeList
	}

	ignoredList := []string{
		"**/*.go",
		path.Join("**", v.Wave.GetDistDir()+"/**/*"),
		path.Join("**", v.Wave.GetPrivateStaticDir()+"/**/*"),
		path.Join("**", v.Wave.GetConfigFile()),
		path.Join("**", v.Wave.GetVormaTSGenOutPath()),
		path.Join("**", v.Wave.GetVormaClientRouteDefsFile()),
	}

	mapAsJSON, err := json.MarshalIndent(fileMap, "", "\t") // No initial indent
	if err != nil {
		return "", fmt.Errorf("error marshalling map to JSON: %w", err)
	}

	var buf bytes.Buffer
	err = vitePluginTemplate.Execute(&buf, map[string]any{
		"Entrypoints":              entrypoints,
		"PublicPathPrefix":         v.Wave.GetPublicPathPrefix(),
		"StaticPublicAssetMapJSON": template.HTML(mapAsJSON),
		"FuncName":                 v.Wave.GetVormaBuildtimePublicURLFuncName(),
		"IgnoredPatterns":          ignoredList,
		"DedupeList":               dedupeList,
	})
	if err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	sb.Write(buf.String())

	return sb.String(), nil
}

func (v *Vorma) handleViteConfigHelper(extraTS string) error {
	entrypoints := v.getEntrypoints()

	publicFileMap, err := v.Wave.GetSimplePublicFileMapBuildtime()
	if err != nil {
		Log.Error(fmt.Sprintf("HandleEntrypoints: error getting public file map: %s", err))
		return err
	}

	rollupOptions, err := v.toRollupOptions(entrypoints, publicFileMap)
	if err != nil {
		Log.Error(fmt.Sprintf("HandleEntrypoints: error converting entrypoints to rollup options: %s", err))
		return err
	}

	rollupOptions = extraTS + rollupOptions

	target := filepath.Join(".", v.Wave.GetVormaTSGenOutPath())

	err = os.MkdirAll(filepath.Dir(target), os.ModePerm)
	if err != nil {
		Log.Error(fmt.Sprintf("HandleEntrypoints: error creating directory: %s", err))
		return err
	}

	if err = os.WriteFile(target, []byte(rollupOptions), os.ModePerm); err != nil {
		Log.Error(fmt.Sprintf("HandleEntrypoints: error writing entrypoints to disk: %s", err))
		return err
	}

	return nil
}

type NodeScriptResultItem struct {
	Pattern  string `json:"p"`
	Module   string `json:"m"`
	Key      string `json:"k"`
	ErrorKey string `json:"ek,omitempty"`
}

type NodeScriptResult []NodeScriptResultItem

type buildInnerOptions struct {
	isDev        bool
	buildOptions *BuildOptions
}

// Finds `import("./path")` and captures just the path string `"./path"`.
// Handles single quotes, double quotes, and backticks.
// Intended to be run post-minification to ensure consistent formatting.
var importRegex = regexp.MustCompile(`import\((` +
	"`" + `[^` + "`" + `]+` + "`" +
	`|'[^']+'|"[^"]+")\)`)

// RouteCall represents a parsed route() function call.
type RouteCall struct {
	Pattern  string
	Module   string
	Key      string
	ErrorKey string
}

// importTracker tracks variable assignments that contain import() calls
type importTracker struct {
	imports map[string]string // varName -> importPath
}

// routeCallVisitor is a stateful struct to find route() calls while walking the AST.
type routeCallVisitor struct {
	routeFuncNames map[string]bool
	routes         *[]RouteCall
	importTracker  *importTracker
}

// Enter is called for each node when descending into the AST.
func (v *routeCallVisitor) Enter(n js.INode) js.IVisitor {
	call, isCall := n.(*js.CallExpr)
	if !isCall {
		return v
	}

	ident, isIdent := call.X.(*js.Var)
	if !isIdent {
		return v
	}

	if _, isRouteFunc := v.routeFuncNames[string(ident.Data)]; isRouteFunc {
		route := RouteCall{Key: "default"}
		argsList := call.Args.List

		extractStringArg := func(idx int) (string, bool) {
			if idx < len(argsList) {
				if strLit, ok := argsList[idx].Value.(*js.LiteralExpr); ok && strLit.TokenType == js.StringToken {
					unquoted, err := strconv.Unquote(string(strLit.Data))
					if err == nil {
						return unquoted, true
					}
				}
			}
			return "", false
		}

		// Extract pattern (first argument)
		val, ok := extractStringArg(0)
		if !ok {
			return v
		}
		route.Pattern = val

		// Extract module (second argument) -- could be a variable or direct import
		if len(argsList) > 1 {
			arg := argsList[1]

			// Check if it's a variable reference
			if varRef, ok := arg.Value.(*js.Var); ok {
				if importPath, exists := v.importTracker.imports[string(varRef.Data)]; exists {
					route.Module = importPath
				} else {
					return v // Skip if we can't resolve the variable
				}
			} else if call, ok := arg.Value.(*js.CallExpr); ok {
				// Direct import() call
				if ident, ok := call.X.(*js.Var); ok && string(ident.Data) == "import" {
					if len(call.Args.List) > 0 {
						if strLit, ok := call.Args.List[0].Value.(*js.LiteralExpr); ok && strLit.TokenType == js.StringToken {
							unquoted, err := strconv.Unquote(string(strLit.Data))
							if err == nil {
								route.Module = unquoted
							} else {
								return v
							}
						}
					}
				}
			} else {
				// Try to extract as string (shouldn't happen with imports, but just in case)
				val, ok := extractStringArg(1)
				if !ok {
					return v
				}
				route.Module = val
			}
		}

		// Extract remaining arguments
		if val, ok = extractStringArg(2); ok {
			route.Key = val
		}

		if val, ok = extractStringArg(3); ok {
			route.ErrorKey = val
		}

		*v.routes = append(*v.routes, route)
	}
	return v
}

// Exit is called when ascending from a node.
func (v *routeCallVisitor) Exit(n js.INode) {}

// extractRouteCalls uses an AST parser to find all `route()` calls.
func extractRouteCalls(code string) ([]RouteCall, error) {
	parsedAST, err := js.Parse(parse.NewInputString(code), js.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse JS/TS code: %w", err)
	}

	routeFuncNames := make(map[string]bool)
	tracker := &importTracker{
		imports: make(map[string]string),
	}

	// First pass: collect route function names and import assignments
	for _, stmt := range parsedAST.BlockStmt.List {
		switch s := stmt.(type) {
		case *js.ImportStmt:
			// Get the import path from the Module field
			importPath := ""
			if s.Module != nil {
				importPath = string(s.Module)
				// Remove quotes if present
				importPath = strings.Trim(importPath, `"'`+"`")
			}

			// Only process route imports from vorma/client
			if importPath == "vorma/client" {
				for _, alias := range s.List {
					if string(alias.Name) == "route" ||
						(string(alias.Name) == "" && string(alias.Binding) == "route") {
						if len(alias.Binding) > 0 {
							routeFuncNames[string(alias.Binding)] = true
						} else {
							routeFuncNames[string(alias.Name)] = true
						}
					}
				}
			}
		case *js.VarDecl:
			// Look for const/let/var declarations with string literals (transformed imports)
			for _, binding := range s.List {
				if varBinding, ok := binding.Binding.(*js.Var); ok {
					varName := string(varBinding.Data)

					// Since imports were transformed to string literals, check for those
					if strLit, ok := binding.Default.(*js.LiteralExpr); ok && strLit.TokenType == js.StringToken {
						unquoted, err := strconv.Unquote(string(strLit.Data))
						if err == nil {
							tracker.imports[varName] = unquoted
						}
					}
				}
			}
		}
	}

	var routes []RouteCall
	visitor := &routeCallVisitor{
		routeFuncNames: routeFuncNames,
		routes:         &routes,
		importTracker:  tracker,
	}
	js.Walk(visitor, parsedAST)

	return routes, nil
}

func (v *Vorma) buildInner(opts *buildInnerOptions) error {
	a := time.Now()

	v.mu.Lock()
	defer v.mu.Unlock()

	v._isDev = opts.isDev

	if v._isDev {
		buildID, err := id.New(16)
		if err != nil {
			Log.Error(fmt.Sprintf("error generating random ID: %s", err))
			return err
		}
		v._buildID = "dev_" + buildID
		Log.Info("START building Vorma (DEV)")
	} else {
		Log.Info("START building Vorma (PROD)")
	}

	clientRouteDefsFile := v.Wave.GetVormaClientRouteDefsFile()

	code, err := os.ReadFile(clientRouteDefsFile)
	if err != nil {
		Log.Error(fmt.Sprintf("error reading client route defs file: %s", err))
		return err
	}

	// First, transpile and minify the routes file to ensure consistent import format
	minifyResult := esbuild.Transform(string(code), esbuild.TransformOptions{
		Format:            esbuild.FormatESModule,
		Platform:          esbuild.PlatformNode,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		MinifyIdentifiers: false,
		Loader:            esbuild.LoaderTSX,
		Target:            esbuild.ES2020,
	})
	if len(minifyResult.Errors) > 0 {
		for _, msg := range minifyResult.Errors {
			Log.Error(fmt.Sprintf("esbuild error: %s", msg.Text))
		}
		return fmt.Errorf("esbuild errors occurred during transform")
	}
	minifiedCode := string(minifyResult.Code)

	// Apply the import transformation to the minified code
	transformedCode := importRegex.ReplaceAllString(minifiedCode, "$1")

	// Extract route calls from the transformed code
	routeCalls, err := extractRouteCalls(transformedCode)
	if err != nil {
		Log.Error(fmt.Sprintf("error extracting route calls: %s", err))
		return err
	}

	v._paths = make(map[string]*Path)

	routesDir := filepath.Dir(clientRouteDefsFile)
	for _, routeCall := range routeCalls {
		// The module path is now a raw string literal, so we need to unquote it if needed
		unquotedModule := routeCall.Module

		resolvedModulePath, err := filepath.Rel(".", filepath.Join(routesDir, unquotedModule))
		if err != nil {
			Log.Warn(fmt.Sprintf("could not make module path relative: %s", err))
			resolvedModulePath = unquotedModule
		}
		modulePath := filepath.ToSlash(resolvedModulePath)

		// Check if the module file exists on disk
		if _, err := os.Stat(modulePath); err != nil {
			if os.IsNotExist(err) {
				errMsg := fmt.Sprintf("Component module does not exist: %s (pattern: %s). Did you specify the correct file extension?", modulePath, routeCall.Pattern)
				Log.Error(errMsg)
				return errors.New(errMsg)
			}
			errMsg := fmt.Sprintf("Error accessing component module %s: %v", modulePath, err)
			Log.Error(errMsg)
			return errors.New(errMsg)
		}

		v._paths[routeCall.Pattern] = &Path{
			OriginalPattern: routeCall.Pattern,
			SrcPath:         modulePath,
			ExportKey:       routeCall.Key,
			ErrorExportKey:  routeCall.ErrorKey,
		}
	}

	allServerRoutes := v.LoadersRouter().NestedRouter.AllRoutes()
	for pattern := range allServerRoutes {
		if _, hasClientRoute := v._paths[pattern]; !hasClientRoute {
			// Create a pass-through path entry
			v._paths[pattern] = &Path{
				OriginalPattern: pattern,
				SrcPath:         "", // Empty indicates pass-through
				ExportKey:       "default",
				ErrorExportKey:  "",
			}
		}
	}

	// Remove all files in StaticPublicOutDir starting with vormaChunkPrefix or vormaEntryPrefix.
	err = cleanStaticPublicOutDir(v.Wave.GetStaticPublicOutDir())
	if err != nil {
		Log.Error(fmt.Sprintf("error cleaning static public out dir: %s", err))
		return err
	}

	manifest := v.generateRouteManifest(v.LoadersRouter().NestedRouter)
	manifestFile, err := v.writeRouteManifestToDisk(manifest)
	if err != nil {
		Log.Error(fmt.Sprintf("error writing route manifest: %s", err))
		return err
	}
	v._routeManifestFile = manifestFile

	if err = v.writePathsToDisk_StageOne(); err != nil {
		Log.Error(fmt.Sprintf("error writing paths to disk: %s", err))
		return err
	}

	tsgenOutput, err := v.generateTypeScript(&tsGenOptions{
		LoadersRouter: v.LoadersRouter().NestedRouter,
		ActionsRouter: v.ActionsRouter().Router,
		AdHocTypes:    opts.buildOptions.AdHocTypes,
		ExtraTSCode:   opts.buildOptions.ExtraTSCode,
	})
	if err != nil {
		Log.Error(fmt.Sprintf("error generating TypeScript: %s", err))
		return err
	}

	if err = v.handleViteConfigHelper(tsgenOutput); err != nil {
		// already logged internally in handleViteConfigHelper
		return err
	}

	if !v._isDev {
		if err := v.Wave.ViteProdBuildWave(); err != nil {
			Log.Error(fmt.Sprintf("error running vite prod build: %s", err))
			return err
		}

		if err := v.postViteProdBuild(); err != nil {
			Log.Error(fmt.Sprintf("error running post vite prod build: %s", err))
			return err
		}
	}

	Log.Info("DONE building Vorma",
		"buildID", v._buildID,
		"routes found", len(routeCalls),
		"duration", time.Since(a),
	)

	return nil
}

func (v *Vorma) getViteDevURL() string {
	if !v._isDev {
		return ""
	}
	return fmt.Sprintf("http://localhost:%s", viteutil.GetVitePortStr())
}

/////////////////////////////////////////////////////////////////////
/////// CLEAN STATIC PUBLIC OUT DIR
/////////////////////////////////////////////////////////////////////

func cleanStaticPublicOutDir(staticPublicOutDir string) error {
	fileInfo, err := os.Stat(staticPublicOutDir)
	if err != nil {
		if os.IsNotExist(err) {
			Log.Warn(fmt.Sprintf("static public out dir does not exist: %s", staticPublicOutDir))
			return nil
		}
		return err
	}

	if !fileInfo.IsDir() {
		wrapped := fmt.Errorf("%s is not a directory", staticPublicOutDir)
		Log.Error(wrapped.Error())
		return wrapped
	}

	// delete the ".vite" directory
	err = os.RemoveAll(filepath.Join(staticPublicOutDir, ".vite"))
	if err != nil {
		wrapped := fmt.Errorf("error removing .vite directory: %s", err)
		Log.Error(wrapped.Error())
		return wrapped
	}

	// delete all files starting with vormaPrehashedFilePrefix
	err = filepath.Walk(staticPublicOutDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.Base(path), vormaVitePrehashedFilePrefix) ||
			strings.HasPrefix(filepath.Base(path), vormaRouteManifestPrefix) {
			err = os.Remove(path)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

/////////////////////////////////////////////////////////////////////
/////// GET ENTRYPOINTS
/////////////////////////////////////////////////////////////////////

func (v *Vorma) getEntrypoints() []string {
	entryPoints := make(map[string]struct{}, len(v._paths)+1)
	entryPoints[path.Clean(v.Wave.GetVormaClientEntry())] = struct{}{}
	for _, path := range v._paths {
		if path.SrcPath != "" {
			entryPoints[path.SrcPath] = struct{}{}
		}
	}
	keys := make([]string, 0, len(entryPoints))
	for key := range entryPoints {
		keys = append(keys, key)
	}
	slices.SortStableFunc(keys, strings.Compare)
	return keys
}

/////////////////////////////////////////////////////////////////////
/////// TO PATHS FILE -- STAGE TWO
/////////////////////////////////////////////////////////////////////

func (v *Vorma) toPathsFile_StageTwo() (*PathsFile, error) {
	vormaClientEntryOut := ""
	vormaClientEntryDeps := []string{}
	depToCSSBundleMap := make(map[string]string)

	viteManifest, err := viteutil.ReadManifest(v.Wave.GetViteManifestLocation())
	if err != nil {
		Log.Error(fmt.Sprintf("error reading vite manifest: %s", err))
		return nil, err
	}

	cleanClientEntry := filepath.Clean(v.Wave.GetVormaClientEntry())

	// Assuming manifestJSON is your Vite manifest
	for key, chunk := range viteManifest {
		cleanKey := filepath.Base(chunk.File)

		// Handle CSS bundles
		// In Vite, CSS is handled through the CSS array
		if len(chunk.CSS) > 0 {
			for _, cssFile := range chunk.CSS {
				depToCSSBundleMap[cleanKey] = filepath.Base(cssFile)
			}
		}

		// Get dependencies
		deps := viteutil.FindAllDependencies(viteManifest, key)

		// Handle client entry
		if chunk.IsEntry && cleanClientEntry == chunk.Src {
			vormaClientEntryOut = cleanKey
			depsWithoutClientEntry := make([]string, 0, len(deps)-1)
			for _, dep := range deps {
				if dep != vormaClientEntryOut {
					depsWithoutClientEntry = append(depsWithoutClientEntry, dep)
				}
			}
			vormaClientEntryDeps = depsWithoutClientEntry
		} else {
			// Handle other paths
			for i, path := range v._paths {
				// Compare with source path instead of entryPoint
				if path.SrcPath == chunk.Src {
					v._paths[i].OutPath = cleanKey
					v._paths[i].Deps = deps
				}
			}
		}
	}

	htmlTemplateContent, err := os.ReadFile(path.Join(v.Wave.GetPrivateStaticDir(), v.Wave.GetVormaHTMLTemplateLocation()))
	if err != nil {
		Log.Error(fmt.Sprintf("error reading HTML template file: %s", err))
		return nil, err
	}
	htmlContentHash := cryptoutil.Sha256Hash(htmlTemplateContent)

	pf := &PathsFile{
		Stage:             "two",
		DepToCSSBundleMap: depToCSSBundleMap,
		Paths:             v._paths,
		ClientEntrySrc:    v.Wave.GetVormaClientEntry(),
		ClientEntryOut:    vormaClientEntryOut,
		ClientEntryDeps:   vormaClientEntryDeps,
		RouteManifestFile: v._routeManifestFile,
	}

	asJSON, err := json.Marshal(pf)
	if err != nil {
		Log.Error(fmt.Sprintf("error marshalling paths file to JSON: %s", err))
		return nil, err
	}
	pfJSONHash := cryptoutil.Sha256Hash(asJSON)

	publicFSSummaryHash, err := getFSSummaryHash(os.DirFS(v.Wave.GetStaticPublicOutDir()))
	if err != nil {
		Log.Error(fmt.Sprintf("error getting FS summary hash: %s", err))
		return nil, err
	}

	fullHash := sha256.New()
	fullHash.Write(htmlContentHash)
	fullHash.Write(pfJSONHash)
	fullHash.Write(publicFSSummaryHash)
	buildID := base64.RawURLEncoding.EncodeToString(fullHash.Sum(nil)[:16])

	v._buildID = buildID
	pf.BuildID = buildID

	return pf, nil
}

func (v *Vorma) writeRouteManifestToDisk(manifest map[string]int) (string, error) {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("error marshalling route manifest: %w", err)
	}

	// Hash the content to create a stable filename
	hash := cryptoutil.Sha256Hash(manifestJSON)
	hashStr := base64.RawURLEncoding.EncodeToString(hash[:8])
	filename := fmt.Sprintf(vormaRouteManifestPrefix+"%s.json", hashStr)

	// Write to static public dir so it's served automatically
	outPath := filepath.Join(v.Wave.GetStaticPublicOutDir(), filename)
	if err := os.WriteFile(outPath, manifestJSON, 0644); err != nil {
		return "", fmt.Errorf("error writing route manifest: %w", err)
	}

	return filename, nil
}

func (v *Vorma) generateRouteManifest(nestedRouter *mux.NestedRouter) map[string]int {
	manifest := make(map[string]int)

	for _, v := range v._paths {
		hasServerLoader := 0
		if nestedRouter.HasTaskHandler(v.OriginalPattern) {
			hasServerLoader = 1
		}
		manifest[v.OriginalPattern] = hasServerLoader
	}

	return manifest
}
