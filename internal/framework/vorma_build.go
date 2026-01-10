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
		VormaOutDirname,
		VormaPathsStageOneJSONFileName,
	)
	err := os.MkdirAll(filepath.Dir(pathsJSONOut_StageOne), os.ModePerm)
	if err != nil {
		return err
	}

	pathsAsJSON, err := json.MarshalIndent(PathsFile{
		Stage:             "one",
		Paths:             v._paths,
		ClientEntrySrc:    v.config.ClientEntry,
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
import { staticPublicAssetMap } from "./filemap";
export { staticPublicAssetMap };
export type StaticPublicAsset = keyof typeof staticPublicAssetMap;

declare global {
	function {{.FuncName}}(staticPublicAsset: StaticPublicAsset): string;
}

export const publicPathPrefix = "{{.PublicPathPrefix}}";

export function waveRuntimeURL(originalPublicURL: StaticPublicAsset) {
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

func (v *Vorma) toRollupOptions(entrypoints []string) (string, error) {
	var sb stringsutil.Builder

	sb.Return()
	sb.Write(tsgen.Comment("Vorma Vite Config:"))
	sb.Return()

	var dedupeList []string
	switch UIVariant(v.config.UIVariant) {
	case UIVariants.React:
		dedupeList = reactDedupeList
	case UIVariants.Preact:
		dedupeList = preactDedupeList
	case UIVariants.Solid:
		dedupeList = solidDedupeList
	}

	// Ignore the entire TSGenOutDir (contains filemap.ts and index.ts)
	ignoredList := []string{
		"**/*.go",
		path.Join("**", v.Wave.GetDistDir()+"/**/*"),
		path.Join("**", v.Wave.GetPrivateStaticDir()+"/**/*"),
		path.Join("**", v.Wave.GetConfigFile()),
		path.Join("**", v.config.TSGenOutDir+"/**/*"),
		path.Join("**", v.config.ClientRouteDefsFile),
	}

	var buf bytes.Buffer
	err := vitePluginTemplate.Execute(&buf, map[string]any{
		"Entrypoints":      entrypoints,
		"PublicPathPrefix": v.Wave.GetPublicPathPrefix(),
		"FuncName":         v.config.BuildtimePublicURLFuncName,
		"IgnoredPatterns":  ignoredList,
		"DedupeList":       dedupeList,
	})
	if err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	sb.Write(buf.String())

	return sb.String(), nil
}

// writeGeneratedTS generates and writes the complete TypeScript output file.
// This includes both the route type definitions and the Vite config.
func (v *Vorma) writeGeneratedTS() error {
	tsOutput, err := v.generateTypeScript(&tsGenOptions{
		LoadersRouter: v.LoadersRouter().NestedRouter,
		ActionsRouter: v.ActionsRouter().Router,
		// Use stored configuration which is available in both build and server processes
		AdHocTypes:  v._adHocTypes,
		ExtraTSCode: v._extraTSCode,
	})
	if err != nil {
		return fmt.Errorf("generate TypeScript: %w", err)
	}

	rollupOptions, err := v.toRollupOptions(v.getEntrypoints())
	if err != nil {
		return fmt.Errorf("generate rollup options: %w", err)
	}

	content := tsOutput + rollupOptions

	// Hash check to skip write if unchanged (prevents unnecessary Vite HMR)
	newHash := sha256.Sum256([]byte(content))
	if v._lastConfigHash == newHash {
		Log.Info("Generated config unchanged, skipping write")
		return nil
	}
	v._lastConfigHash = newHash

	tsGenOutDir := v.config.TSGenOutDir
	target := filepath.Join(".", tsGenOutDir, "index.ts")

	if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
		Log.Error(fmt.Sprintf("writeGeneratedTS: error creating directory: %s", err))
		return err
	}

	if err := os.WriteFile(target, []byte(content), os.ModePerm); err != nil {
		Log.Error(fmt.Sprintf("writeGeneratedTS: error writing file: %s", err))
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
	isDev bool
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
func (rv *routeCallVisitor) Enter(n js.INode) js.IVisitor {
	call, isCall := n.(*js.CallExpr)
	if !isCall {
		return rv
	}

	ident, isIdent := call.X.(*js.Var)
	if !isIdent {
		return rv
	}

	if _, isRouteFunc := rv.routeFuncNames[string(ident.Data)]; isRouteFunc {
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
			return rv
		}
		route.Pattern = val

		// Extract module (second argument) -- could be a variable or direct import
		if len(argsList) > 1 {
			arg := argsList[1]

			// Check if it's a variable reference
			if varRef, ok := arg.Value.(*js.Var); ok {
				if importPath, exists := rv.importTracker.imports[string(varRef.Data)]; exists {
					route.Module = importPath
				} else {
					return rv // Skip if we can't resolve the variable
				}
			} else if innerCall, ok := arg.Value.(*js.CallExpr); ok {
				// Direct import() call
				if innerIdent, ok := innerCall.X.(*js.Var); ok && string(innerIdent.Data) == "import" {
					if len(innerCall.Args.List) > 0 {
						if strLit, ok := innerCall.Args.List[0].Value.(*js.LiteralExpr); ok && strLit.TokenType == js.StringToken {
							unquoted, err := strconv.Unquote(string(strLit.Data))
							if err == nil {
								route.Module = unquoted
							} else {
								return rv
							}
						}
					}
				}
			} else {
				// Try to extract as string (shouldn't happen with imports, but just in case)
				val, ok := extractStringArg(1)
				if !ok {
					return rv
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

		*rv.routes = append(*rv.routes, route)
	}
	return rv
}

// Exit is called when ascending from a node.
func (rv *routeCallVisitor) Exit(n js.INode) {}

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

// parseClientRoutes reads and parses vorma.routes.ts, returning the paths map.
// This is the single source of truth for route parsing, used by both
// buildInner() and rebuildRoutesOnly().
func (v *Vorma) parseClientRoutes() (map[string]*Path, error) {
	clientRouteDefsFile := v.config.ClientRouteDefsFile

	code, err := os.ReadFile(clientRouteDefsFile)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
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
		return nil, errors.New("esbuild transform failed")
	}

	// Apply the import transformation to the minified code
	transformedCode := importRegex.ReplaceAllString(string(minifyResult.Code), "$1")

	// Extract route calls from the transformed code
	routeCalls, err := extractRouteCalls(transformedCode)
	if err != nil {
		return nil, fmt.Errorf("extract route calls: %w", err)
	}

	paths := make(map[string]*Path, len(routeCalls))
	routesDir := filepath.Dir(clientRouteDefsFile)

	for _, rc := range routeCalls {
		resolvedModulePath, err := filepath.Rel(".", filepath.Join(routesDir, rc.Module))
		if err != nil {
			Log.Warn(fmt.Sprintf("could not make module path relative: %s", err))
			resolvedModulePath = rc.Module
		}
		modulePath := filepath.ToSlash(resolvedModulePath)

		// Check if the module file exists on disk
		if _, err := os.Stat(modulePath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("component module does not exist: %s (pattern: %s). Did you specify the correct file extension?", modulePath, rc.Pattern)
			}
			return nil, fmt.Errorf("access component module %s: %w", modulePath, err)
		}

		paths[rc.Pattern] = &Path{
			OriginalPattern: rc.Pattern,
			SrcPath:         modulePath,
			ExportKey:       rc.Key,
			ErrorExportKey:  rc.ErrorKey,
		}
	}

	return paths, nil
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

	// Parse client routes (single source of truth)
	paths, err := v.parseClientRoutes()
	if err != nil {
		Log.Error(fmt.Sprintf("error parsing client routes: %s", err))
		return err
	}

	// Sync routes via registry (merges server routes, rebuilds router, clears cache)
	v.routes().Sync(paths)

	// Remove Vorma-generated files
	err = cleanStaticPublicOutDir(v.Wave.GetStaticPublicOutDir())
	if err != nil {
		Log.Error(fmt.Sprintf("error cleaning static public out dir: %s", err))
		return err
	}

	// Ensure Wave writes the filemap.ts before we generate index.ts
	if err = v.Wave.WritePublicFileMapTS(v.config.TSGenOutDir); err != nil {
		Log.Error(fmt.Sprintf("error writing public file map TS: %s", err))
		return err
	}

	// Write all route artifacts (manifest, paths JSON, TypeScript)
	if err = v.routes().WriteArtifacts(); err != nil {
		Log.Error(fmt.Sprintf("error writing route artifacts: %s", err))
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
		"routes found", len(v._paths),
		"duration", time.Since(a),
	)

	return nil
}

func (v *Vorma) getViteDevURL() string {
	if !v.getIsDev() {
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

	err = filepath.Walk(staticPublicOutDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		baseName := filepath.Base(path)
		if strings.HasPrefix(baseName, vormaVitePrehashedFilePrefix) ||
			strings.HasPrefix(baseName, vormaRouteManifestPrefix) {
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
	entryPoints[path.Clean(v.config.ClientEntry)] = struct{}{}
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

	cleanClientEntry := filepath.Clean(v.config.ClientEntry)

	for key, chunk := range viteManifest {
		cleanKey := filepath.Base(chunk.File)

		// Handle CSS bundles
		// In Vite, CSS is handled through the CSS array
		if len(chunk.CSS) > 0 {
			for _, cssFile := range chunk.CSS {
				depToCSSBundleMap[cleanKey] = filepath.Base(cssFile)
			}
		}

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

	htmlTemplateContent, err := os.ReadFile(path.Join(v.Wave.GetPrivateStaticDir(), v.config.HTMLTemplateLocation))
	if err != nil {
		Log.Error(fmt.Sprintf("error reading HTML template file: %s", err))
		return nil, err
	}
	htmlContentHash := cryptoutil.Sha256Hash(htmlTemplateContent)

	pf := &PathsFile{
		Stage:             "two",
		DepToCSSBundleMap: depToCSSBundleMap,
		Paths:             v._paths,
		ClientEntrySrc:    v.config.ClientEntry,
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
	filename := fmt.Sprintf("%s%s.json", vormaRouteManifestPrefix, hashStr)

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
