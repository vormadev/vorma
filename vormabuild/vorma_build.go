package vormabuild

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
	wavebuild "github.com/vormadev/vorma/wave/tooling"
)

func registerVormaSchema(v *vormaruntime.Vorma) {
	cfg := v.Wave.GetParsedConfig()
	if cfg.FrameworkSchemaExtensions == nil {
		cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	cfg.FrameworkSchemaExtensions["Vorma"] = Vorma_Schema
}

func injectFrameworkBuildHooks(v *vormaruntime.Vorma) {
	cfg := v.Wave.GetParsedConfig()
	cfg.FrameworkDevBuildHook = fmt.Sprintf("go run ./%s --dev --hook", v.Config.MainBuildEntry)
	cfg.FrameworkProdBuildHook = fmt.Sprintf("go run ./%s --hook", v.Config.MainBuildEntry)
}

// Build parses flags and runs the build or dev server.
func Build(v *vormaruntime.Vorma) {
	dev := flag.Bool("dev", false, "run in development mode")
	hook := flag.Bool("hook", false, "run build hook only (internal use)")
	noBinary := flag.Bool("no-binary", false, "skip go binary compilation")
	flag.Parse()

	if *hook {
		configureBuildEnvironment(v)
		if err := runBuildHook(v, *dev); err != nil {
			log.Fatalf("build hook failed: %v", err)
		}
		if !*dev {
			if err := runProdHookPostProcessing(v); err != nil {
				log.Fatalf("%v", err)
			}
		}
		return
	}

	if err := build(v, *dev, *noBinary); err != nil {
		log.Fatalf("build failed: %v", err)
	}
}

// build performs a full Vorma build.
func build(v *vormaruntime.Vorma, isDev bool, noBinary bool) error {
	configureBuildEnvironment(v)

	if isDev {
		return runDevBuild(v)
	}

	return runProductionBuild(v, noBinary)
}

func configureBuildEnvironment(v *vormaruntime.Vorma) {
	registerVormaSchema(v)
	injectDefaultWatchPatterns(v)
	injectFrameworkBuildHooks(v)
}

func runBuildHook(v *vormaruntime.Vorma, isDev bool) error {
	return buildInner(v, &buildInnerOptions{isDev: isDev})
}

func runProdHookPostProcessing(v *vormaruntime.Vorma) error {
	builder := wavebuild.NewBuilder(v.Wave.GetParsedConfig(), v.Wave.Logger())
	defer builder.Close()

	if err := builder.ViteProdBuild(); err != nil {
		return fmt.Errorf("Vite production build failed: %w", err)
	}

	if err := postViteProdBuild(v); err != nil {
		return fmt.Errorf("post Vite production build failed: %w", err)
	}

	return nil
}

func runDevBuild(v *vormaruntime.Vorma) error {
	wave.SetModeToDev()
	// Set isDev on this process's Vorma instance so callbacks
	// (like rebuildRoutesOnly) can run in the dev server process.
	v.SetIsDev(true)
	return wavebuild.RunDev(v.Wave.GetParsedConfig(), v.Wave.Logger())
}

func runProductionBuild(v *vormaruntime.Vorma, noBinary bool) error {
	// Production Build
	//
	// The build flow is:
	// 1. wb.Build() sets up dist directory, then runs ProdBuildHook
	// 2. ProdBuildHook (e.g., "go run ./cmd/build --hook") runs in subprocess:
	//    a. buildInner() - parses routes, writes artifacts
	//    b. ViteProdBuild() - runs Vite
	//    c. postViteProdBuild() - processes Vite output
	// 3. wb.Build() compiles the Go binary
	//
	// All Vorma logic runs in the subprocess (via --hook) so state is shared.
	wb := wavebuild.NewBuilder(v.Wave.GetParsedConfig(), v.Wave.Logger())
	defer wb.Close()

	return wb.Build(wavebuild.BuildOpts{
		CompileGo: !noBinary,
		IsDev:     false,
		IsRebuild: false,
	})
}

func injectDefaultWatchPatterns(v *vormaruntime.Vorma) {
	includeDefaults := true
	if v.Config.IncludeDefaults != nil {
		includeDefaults = *v.Config.IncludeDefaults
	}
	if !includeDefaults {
		return
	}

	cfg := v.Wave.GetParsedConfig()
	patterns := getDefaultWatchPatterns(v)
	cfg.FrameworkWatchPatterns = append(cfg.FrameworkWatchPatterns, patterns...)

	if v.Config.TSGenOutDir != "" {
		cfg.FrameworkPublicFileMapOutDir = v.Config.TSGenOutDir
		cfg.FrameworkIgnoredPatterns = append(cfg.FrameworkIgnoredPatterns,
			filepath.Join(v.Config.TSGenOutDir, wave.GeneratedTSFileName),
			filepath.Join(v.Config.TSGenOutDir, wave.PublicFileMapTSName),
			filepath.Join(v.Config.TSGenOutDir, wave.PublicFileMapJSONName),
		)
	}
}

func getDefaultWatchPatterns(v *vormaruntime.Vorma) []wave.WatchedFile {
	var patterns []wave.WatchedFile

	// Route definitions file
	if clientRouteDefsFile := v.Config.ClientRouteDefsFile; clientRouteDefsFile != "" {
		patterns = append(patterns, wave.WatchedFile{
			Pattern:         clientRouteDefsFile,
			RunOnChangeOnly: true, // Skip standard build - callback handles everything
			OnChangeHooks: []wave.OnChangeHook{{
				Callback: func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
					// 1. Process A work: Rebuild artifacts
					if err := rebuildRoutesOnly(v); err != nil {
						return nil, err
					}

					// 2. If the app was stopped (e.g., Go file changed in same batch),
					// the batch restart will handle everything. Don't try to call endpoints.
					if ctx.AppStoppedForBatch {
						return nil, nil
					}

					// 3. Process A talks to Process B: Call reload endpoint
					return getReloadActionForEndpointWithFallback(
						v,
						vormaruntime.Dev_ReloadRoutesPath,
						"route reload endpoint failed, falling back to restart",
					), nil
				},
			}},
			SkipRebuildingNotification: true,
		})
	}

	// HTML template file
	htmlTemplateLocation := v.Config.HTMLTemplateLocation
	privateStaticDir := v.Wave.GetPrivateStaticDir()
	if htmlTemplateLocation != "" && privateStaticDir != "" {
		templatePath := filepath.Join(privateStaticDir, htmlTemplateLocation)
		patterns = append(patterns, wave.WatchedFile{
			Pattern:         templatePath,
			RunOnChangeOnly: true, // Skip standard build - callback handles everything
			OnChangeHooks: []wave.OnChangeHook{{
				Callback: func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
					if ctx.AppStoppedForBatch {
						return nil, nil
					}

					return getReloadActionForEndpointWithFallback(
						v,
						vormaruntime.Dev_ReloadTemplatePath,
						"template reload endpoint failed, falling back to restart",
					), nil
				},
			}},
		})
	}

	// Go files
	patterns = append(patterns, wave.WatchedFile{
		Pattern: "**/*.go",
		OnChangeHooks: []wave.OnChangeHook{{
			Cmd:    "DevBuildHook",
			Timing: wave.OnChangeStrategyConcurrent,
		}},
	})

	return patterns
}

func getReloadActionForEndpointWithFallback(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
) *wave.RefreshAction {
	if err := callReloadEndpoint(v, endpoint); err != nil {
		v.Log.Warn(warnMessage, "error", err)
		return newRestartWithoutRecompileAction()
	}
	return newReloadBrowserAndWaitAction()
}

func newReloadBrowserAndWaitAction() *wave.RefreshAction {
	return &wave.RefreshAction{
		ReloadBrowser: true,
		WaitForApp:    true,
		WaitForVite:   true,
	}
}

func newRestartWithoutRecompileAction() *wave.RefreshAction {
	return &wave.RefreshAction{
		TriggerRestart: true,
		RecompileGo:    false,
	}
}

// callReloadEndpoint makes an HTTP GET request to the running app's reload endpoint.
func callReloadEndpoint(v *vormaruntime.Vorma, endpoint string) error {
	port := v.MustGetPort()
	url := fmt.Sprintf("http://localhost:%d%s", port, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("endpoint returned %d", resp.StatusCode)
	}

	return nil
}

type buildInnerOptions struct {
	isDev bool
}

func buildInner(v *vormaruntime.Vorma, opts *buildInnerOptions) error {
	start := time.Now()

	if err := initializeBuildInnerState(v, opts); err != nil {
		return err
	}

	if err := parseAndSyncClientRoutes(v); err != nil {
		return fmt.Errorf("parse client routes: %w", err)
	}

	if err := cleanStaticPublicOutDir(v); err != nil {
		return fmt.Errorf("clean static public out dir: %w", err)
	}

	if err := writePublicFileMapTypeScript(v); err != nil {
		return fmt.Errorf("write public file map TS: %w", err)
	}

	if err := writeRouteArtifactsWithLock(v); err != nil {
		return fmt.Errorf("write route artifacts: %w", err)
	}

	logBuildInnerCompletion(v, start)
	return nil
}

func initializeBuildInnerState(v *vormaruntime.Vorma, opts *buildInnerOptions) error {
	v.SetIsDev(opts.isDev)

	if !opts.isDev {
		v.Log.Info("START building Vorma (PROD)")
		return nil
	}

	buildID, err := id.New(16)
	if err != nil {
		return fmt.Errorf("generate build ID: %w", err)
	}

	v.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("dev_" + buildID)
	})
	v.Log.Info("START building Vorma (DEV)")
	return nil
}

func parseAndSyncClientRoutes(v *vormaruntime.Vorma) error {
	paths, err := parseClientRoutes(v)
	if err != nil {
		return err
	}
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		l.Routes().Sync(paths)
	})
	return nil
}

func writePublicFileMapTypeScript(v *vormaruntime.Vorma) error {
	builder := wavebuild.NewBuilder(v.Wave.GetParsedConfig(), v.Wave.Logger())
	defer builder.Close()

	return builder.WritePublicFileMapTS(v.Config.TSGenOutDir)
}

func writeRouteArtifactsWithLock(v *vormaruntime.Vorma) error {
	var writeRouteArtifactsErr error
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		writeRouteArtifactsErr = writeRouteArtifacts(l)
	})
	return writeRouteArtifactsErr
}

func logBuildInnerCompletion(v *vormaruntime.Vorma, start time.Time) {
	v.Log.Info("DONE building Vorma",
		"buildID", v.GetBuildID(),
		"routes found", len(v.GetPathsSnapshot()),
		"duration", time.Since(start),
	)
}

func cleanStaticPublicOutDir(v *vormaruntime.Vorma) error {
	staticPublicOutDir := v.Wave.GetStaticPublicOutDir()

	fileInfo, err := os.Stat(staticPublicOutDir)
	if err != nil {
		if os.IsNotExist(err) {
			v.Log.Warn(fmt.Sprintf("static public out dir does not exist: %s", staticPublicOutDir))
			return nil
		}
		return err
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", staticPublicOutDir)
	}

	return filepath.Walk(staticPublicOutDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		baseName := filepath.Base(path)
		if shouldRemoveGeneratedStaticPublicFile(baseName) {
			return os.Remove(path)
		}
		return nil
	})
}

func shouldRemoveGeneratedStaticPublicFile(fileBaseName string) bool {
	return strings.HasPrefix(fileBaseName, vormaruntime.VormaVitePrehashedFilePrefix) ||
		strings.HasPrefix(fileBaseName, vormaruntime.VormaRouteManifestPrefix)
}

func writePathsToDisk_StageOne(l *vormaruntime.LockedVorma) error {
	v := l.Vorma()
	pathsJSONOut := filepath.Join(
		v.Wave.GetStaticPrivateOutDir(),
		vormaruntime.VormaOutDirname,
		vormaruntime.VormaPathsStageOneJSONFileName,
	)
	if err := os.MkdirAll(filepath.Dir(pathsJSONOut), os.ModePerm); err != nil {
		return err
	}

	pathsAsJSON, err := json.MarshalIndent(vormaruntime.PathsFile{
		Stage:             "one",
		Paths:             l.GetPaths(),
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           l.GetBuildID(),
		RouteManifestFile: l.GetRouteManifestFile(),
	}, "", "\t")
	if err != nil {
		return err
	}

	return os.WriteFile(pathsJSONOut, pathsAsJSON, os.ModePerm)
}

// --- Route Parsing (esbuild) ---

var importRegex = regexp.MustCompile(`import\((` + "`" + `[^` + "`" + `]+` + "`" + `|'[^']+'|"[^"]+")\)`)

type RouteCall struct {
	Pattern  string
	Module   string
	Key      string
	ErrorKey string
}

// UnresolvedRouteCall represents a route() call where the module path could not
// be statically determined. This happens when the module argument is a variable,
// function call, or other dynamic expression.
type UnresolvedRouteCall struct {
	Pattern       string
	RawModuleExpr string
	Reason        string
}

type routeCallVisitor struct {
	routeFuncNames    map[string]bool
	trackedModuleVars map[string]string
	routes            *[]RouteCall
	unresolvedRoutes  *[]UnresolvedRouteCall
}

func (rv *routeCallVisitor) Enter(n js.INode) js.IVisitor {
	call, isCall := n.(*js.CallExpr)
	if !isCall {
		return rv
	}

	ident, isIdent := call.X.(*js.Var)
	if !isIdent {
		return rv
	}

	if _, isRouteFunc := rv.routeFuncNames[string(ident.Data)]; !isRouteFunc {
		return rv
	}

	route, unresolved, resolved := rv.extractRouteCall(call.Args.List)
	if unresolved != nil {
		*rv.unresolvedRoutes = append(*rv.unresolvedRoutes, *unresolved)
		return rv
	}
	if !resolved {
		return rv
	}

	*rv.routes = append(*rv.routes, route)
	return rv
}

func (rv *routeCallVisitor) Exit(n js.INode) {}

func (rv *routeCallVisitor) extractRouteCall(argsList []js.Arg) (RouteCall, *UnresolvedRouteCall, bool) {
	route := RouteCall{Key: "default"}

	pattern, ok := extractStaticStringArg(argsList, 0)
	if !ok {
		return RouteCall{}, nil, false
	}
	route.Pattern = pattern

	if len(argsList) > 1 {
		modulePath, unresolvedRoute, resolved := rv.resolveModuleArgument(route.Pattern, argsList[1].Value)
		if unresolvedRoute != nil {
			return RouteCall{}, unresolvedRoute, false
		}
		if !resolved {
			return RouteCall{}, nil, false
		}
		route.Module = modulePath
	}

	if key, ok := extractStaticStringArg(argsList, 2); ok {
		route.Key = key
	}
	if errorKey, ok := extractStaticStringArg(argsList, 3); ok {
		route.ErrorKey = errorKey
	}

	return route, nil, true
}

func (rv *routeCallVisitor) resolveModuleArgument(
	routePattern string,
	moduleExpr js.IExpr,
) (string, *UnresolvedRouteCall, bool) {
	if varRef, ok := moduleExpr.(*js.Var); ok {
		varName := string(varRef.Data)
		if trackedModulePath, exists := rv.trackedModuleVars[varName]; exists {
			return trackedModulePath, nil, true
		}
		return "", &UnresolvedRouteCall{
			Pattern:       routePattern,
			RawModuleExpr: varName,
			Reason:        fmt.Sprintf("variable '%s' is not a tracked import or const string", varName),
		}, false
	}

	if functionCall, ok := moduleExpr.(*js.CallExpr); ok {
		return resolveModuleArgumentFromFunctionCall(routePattern, functionCall)
	}

	modulePath, ok := extractStaticStringLiteral(moduleExpr)
	if !ok {
		return "", &UnresolvedRouteCall{
			Pattern:       routePattern,
			RawModuleExpr: "<expression>",
			Reason:        "module argument is not a static string, variable, or import() call",
		}, false
	}
	return modulePath, nil, true
}

func resolveModuleArgumentFromFunctionCall(
	routePattern string,
	functionCall *js.CallExpr,
) (string, *UnresolvedRouteCall, bool) {
	if functionCallTargetsJSImport(functionCall) {
		if modulePath, ok := extractStaticStringArg(functionCall.Args.List, 0); ok {
			return modulePath, nil, true
		}
		return "", &UnresolvedRouteCall{
			Pattern:       routePattern,
			RawModuleExpr: "import(...)",
			Reason:        "dynamic import() argument is not a static string",
		}, false
	}

	functionName := "<unknown>"
	if functionIdent, ok := functionCall.X.(*js.Var); ok {
		functionName = string(functionIdent.Data)
	}
	return "", &UnresolvedRouteCall{
		Pattern:       routePattern,
		RawModuleExpr: functionName + "(...)",
		Reason:        "module argument is a function call, which cannot be statically analyzed",
	}, false
}

func functionCallTargetsJSImport(functionCall *js.CallExpr) bool {
	functionIdent, ok := functionCall.X.(*js.Var)
	return ok && string(functionIdent.Data) == "import"
}

func extractStaticStringArg(args []js.Arg, idx int) (string, bool) {
	if idx >= len(args) {
		return "", false
	}
	return extractStaticStringLiteral(args[idx].Value)
}

func extractStaticStringLiteral(expr js.IExpr) (string, bool) {
	strLit, ok := expr.(*js.LiteralExpr)
	if !ok || strLit.TokenType != js.StringToken {
		return "", false
	}
	unquoted, err := strconv.Unquote(string(strLit.Data))
	if err != nil {
		return "", false
	}
	return unquoted, true
}

func extractRouteCalls(code string) ([]RouteCall, []UnresolvedRouteCall, error) {
	parsedAST, err := js.Parse(parse.NewInputString(code), js.Options{})
	if err != nil {
		return nil, nil, fmt.Errorf("parse JS/TS: %w", err)
	}

	routeFuncNames := collectBuildtimeRouteFunctionNames(parsedAST)
	trackedModuleVars := collectStaticStringVariableAssignments(parsedAST)

	var routes []RouteCall
	var unresolvedRoutes []UnresolvedRouteCall

	visitor := &routeCallVisitor{
		routeFuncNames:    routeFuncNames,
		trackedModuleVars: trackedModuleVars,
		routes:            &routes,
		unresolvedRoutes:  &unresolvedRoutes,
	}
	js.Walk(visitor, parsedAST)

	return routes, unresolvedRoutes, nil
}

func collectBuildtimeRouteFunctionNames(parsedAST *js.AST) map[string]bool {
	routeFuncNames := make(map[string]bool)
	for _, statement := range parsedAST.BlockStmt.List {
		importStmt, isImportStmt := statement.(*js.ImportStmt)
		if !isImportStmt {
			continue
		}
		if !isBuildtimeImportStatement(importStmt) {
			continue
		}
		for _, alias := range importStmt.List {
			if !isRouteImportAlias(alias) {
				continue
			}
			routeFuncName := routeImportAliasBinding(alias)
			if routeFuncName != "" {
				routeFuncNames[routeFuncName] = true
			}
		}
	}
	return routeFuncNames
}

func isBuildtimeImportStatement(importStmt *js.ImportStmt) bool {
	return strings.Trim(string(importStmt.Module), `"'`+"`") == "vorma/buildtime"
}

func isRouteImportAlias(alias js.Alias) bool {
	return string(alias.Name) == "route" || (string(alias.Name) == "" && string(alias.Binding) == "route")
}

func routeImportAliasBinding(alias js.Alias) string {
	if len(alias.Binding) > 0 {
		return string(alias.Binding)
	}
	return string(alias.Name)
}

func collectStaticStringVariableAssignments(parsedAST *js.AST) map[string]string {
	trackedModuleVars := make(map[string]string)
	for _, statement := range parsedAST.BlockStmt.List {
		varDecl, isVarDecl := statement.(*js.VarDecl)
		if !isVarDecl {
			continue
		}
		for _, binding := range varDecl.List {
			varBinding, ok := binding.Binding.(*js.Var)
			if !ok {
				continue
			}
			modulePath, ok := extractStaticStringLiteral(binding.Default)
			if !ok {
				continue
			}
			trackedModuleVars[string(varBinding.Data)] = modulePath
		}
	}
	return trackedModuleVars
}

func parseClientRoutes(v *vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
	code, err := os.ReadFile(v.Config.ClientRouteDefsFile)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	transformedCode, err := transformRouteDefinitionsCode(v, code)
	if err != nil {
		return nil, err
	}

	routeCalls, unresolvedRoutes, err := extractRouteCalls(transformedCode)
	if err != nil {
		return nil, fmt.Errorf("extract route calls: %w", err)
	}

	warnUnresolvedRouteCalls(v, unresolvedRoutes)

	return buildPathsFromRouteCalls(v, routeCalls)
}

func transformRouteDefinitionsCode(v *vormaruntime.Vorma, code []byte) (string, error) {
	transformResult := esbuild.Transform(string(code), esbuild.TransformOptions{
		Format:            esbuild.FormatESModule,
		Platform:          esbuild.PlatformNode,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		MinifyIdentifiers: false,
		Loader:            esbuild.LoaderTSX,
		Target:            esbuild.ES2020,
	})

	if len(transformResult.Errors) > 0 {
		logEsbuildTransformErrors(v, transformResult.Errors)
		return "", errors.New("esbuild transform failed")
	}

	return importRegex.ReplaceAllString(string(transformResult.Code), "$1"), nil
}

func logEsbuildTransformErrors(v *vormaruntime.Vorma, messages []esbuild.Message) {
	for _, message := range messages {
		v.Log.Error(fmt.Sprintf("esbuild error: %s", message.Text))
	}
}

func warnUnresolvedRouteCalls(v *vormaruntime.Vorma, unresolvedRoutes []UnresolvedRouteCall) {
	for _, unresolved := range unresolvedRoutes {
		v.Log.Warn(
			fmt.Sprintf("Route pattern %q has a module path that cannot be statically resolved", unresolved.Pattern),
			"file", v.Config.ClientRouteDefsFile,
			"expression", unresolved.RawModuleExpr,
			"reason", unresolved.Reason,
		)
		v.Log.Warn(
			"This route will be ignored. Use a static string path or a const variable assigned to a string literal.",
		)
	}
}

func buildPathsFromRouteCalls(v *vormaruntime.Vorma, routeCalls []RouteCall) (map[string]*vormaruntime.Path, error) {
	paths := make(map[string]*vormaruntime.Path, len(routeCalls))
	routesDir := filepath.Dir(v.Config.ClientRouteDefsFile)

	for _, routeCall := range routeCalls {
		modulePath := resolveRouteModulePath(v, routesDir, routeCall)
		if err := ensureRouteModuleExists(modulePath, routeCall.Pattern); err != nil {
			return nil, err
		}

		paths[routeCall.Pattern] = &vormaruntime.Path{
			OriginalPattern: routeCall.Pattern,
			SrcPath:         modulePath,
			ExportKey:       routeCall.Key,
			ErrorExportKey:  routeCall.ErrorKey,
		}
	}

	return paths, nil
}

func resolveRouteModulePath(v *vormaruntime.Vorma, routesDir string, routeCall RouteCall) string {
	resolvedModulePath, err := filepath.Rel(".", filepath.Join(routesDir, routeCall.Module))
	if err != nil {
		v.Log.Warn(fmt.Sprintf("could not make module path relative: %s", err))
		resolvedModulePath = routeCall.Module
	}
	return filepath.ToSlash(resolvedModulePath)
}

func ensureRouteModuleExists(modulePath string, pattern string) error {
	if _, err := os.Stat(modulePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("component module does not exist: %s (pattern: %s)", modulePath, pattern)
		}
		return fmt.Errorf("access component module %s: %w", modulePath, err)
	}
	return nil
}
