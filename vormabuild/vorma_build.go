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
	_ = flag.Bool("no-binary", false, "skip go binary compilation (internal use)")
	flag.Parse()

	if *hook {
		registerVormaSchema(v)
		injectDefaultWatchPatterns(v)
		injectFrameworkBuildHooks(v)

		if err := buildInner(v, &buildInnerOptions{isDev: *dev}); err != nil {
			log.Fatalf("build hook failed: %v", err)
		}

		// In prod hook mode, also run Vite and post-processing.
		// This matches the old architecture where everything ran inside
		// the hook subprocess, keeping all state in one process.
		if !*dev {
			wb := wavebuild.NewBuilder(v.Wave.GetParsedConfig(), v.Wave.Logger())
			defer wb.Close()

			if err := wb.ViteProdBuild(); err != nil {
				log.Fatalf("Vite production build failed: %v", err)
			}

			if err := postViteProdBuild(v); err != nil {
				log.Fatalf("post Vite production build failed: %v", err)
			}
		}
		return
	}

	if err := build(v, *dev); err != nil {
		log.Fatalf("build failed: %v", err)
	}
}

// build performs a full Vorma build.
func build(v *vormaruntime.Vorma, isDev bool) error {
	registerVormaSchema(v)
	injectDefaultWatchPatterns(v)
	injectFrameworkBuildHooks(v)

	if isDev {
		wave.SetModeToDev()
		// Set isDev on this process's Vorma instance so callbacks
		// (like rebuildRoutesOnly) can run in the dev server process.
		v.SetIsDev(true)
		return wavebuild.RunDev(v.Wave.GetParsedConfig(), v.Wave.Logger())
	}

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
		CompileGo: true,
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
					if err := callReloadEndpoint(v, vormaruntime.DevReloadRoutesPath); err != nil {
						// Fallback: restart without Go recompile
						v.Log.Warn("route reload endpoint failed, falling back to restart", "error", err)
						return &wave.RefreshAction{TriggerRestart: true, RecompileGo: false}, nil
					}

					// 4. Tell Wave to reload browser after waiting for app/vite
					return &wave.RefreshAction{
						ReloadBrowser: true,
						WaitForApp:    true,
						WaitForVite:   true,
					}, nil
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

					if err := callReloadEndpoint(v, vormaruntime.DevReloadTemplatePath); err != nil {
						v.Log.Warn("template reload endpoint failed, falling back to restart", "error", err)
						return &wave.RefreshAction{TriggerRestart: true, RecompileGo: false}, nil
					}

					return &wave.RefreshAction{
						ReloadBrowser: true,
						WaitForApp:    true,
						WaitForVite:   true,
					}, nil
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

	v.SetIsDev(opts.isDev)

	if opts.isDev {
		buildID, err := id.New(16)
		if err != nil {
			return fmt.Errorf("generate build ID: %w", err)
		}
		v.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID("dev_" + buildID)
		})
		v.Log.Info("START building Vorma (DEV)")
	} else {
		v.Log.Info("START building Vorma (PROD)")
	}

	// Parse client routes
	paths, err := parseClientRoutes(v)
	if err != nil {
		return fmt.Errorf("parse client routes: %w", err)
	}

	// Sync routes
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		l.Routes().Sync(paths)
	})

	// Clean Vorma-generated files
	if err := cleanStaticPublicOutDir(v); err != nil {
		return fmt.Errorf("clean static public out dir: %w", err)
	}

	// Write filemap.ts
	wb := wavebuild.NewBuilder(v.Wave.GetParsedConfig(), v.Wave.Logger())
	defer wb.Close()

	if err := wb.WritePublicFileMapTS(v.Config.TSGenOutDir); err != nil {
		return fmt.Errorf("write public file map TS: %w", err)
	}

	// Write all route artifacts
	var writeErr error
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		writeErr = writeRouteArtifacts(l)
	})
	if writeErr != nil {
		return fmt.Errorf("write route artifacts: %w", writeErr)
	}

	v.Log.Info("DONE building Vorma",
		"buildID", v.GetBuildID(),
		"routes found", len(v.GetPathsSnapshot()),
		"duration", time.Since(start),
	)

	return nil
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
		if strings.HasPrefix(baseName, vormaruntime.VormaVitePrehashedFilePrefix) ||
			strings.HasPrefix(baseName, vormaruntime.VormaRouteManifestPrefix) {
			return os.Remove(path)
		}
		return nil
	})
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

type importTracker struct {
	imports map[string]string
}

type routeCallVisitor struct {
	routeFuncNames   map[string]bool
	routes           *[]RouteCall
	unresolvedRoutes *[]UnresolvedRouteCall
	importTracker    *importTracker
	sourceFile       string
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

		val, ok := extractStringArg(0)
		if !ok {
			return rv
		}
		route.Pattern = val

		if len(argsList) > 1 {
			arg := argsList[1]
			resolved := false

			if varRef, ok := arg.Value.(*js.Var); ok {
				if importPath, exists := rv.importTracker.imports[string(varRef.Data)]; exists {
					route.Module = importPath
					resolved = true
				} else {
					// Variable reference that we can't resolve
					*rv.unresolvedRoutes = append(*rv.unresolvedRoutes, UnresolvedRouteCall{
						Pattern:       route.Pattern,
						RawModuleExpr: string(varRef.Data),
						Reason:        fmt.Sprintf("variable '%s' is not a tracked import or const string", string(varRef.Data)),
					})
					return rv
				}
			} else if innerCall, ok := arg.Value.(*js.CallExpr); ok {
				if innerIdent, ok := innerCall.X.(*js.Var); ok && string(innerIdent.Data) == "import" {
					if len(innerCall.Args.List) > 0 {
						if strLit, ok := innerCall.Args.List[0].Value.(*js.LiteralExpr); ok && strLit.TokenType == js.StringToken {
							unquoted, err := strconv.Unquote(string(strLit.Data))
							if err == nil {
								route.Module = unquoted
								resolved = true
							}
						}
					}
					if !resolved {
						// Dynamic import with non-static argument
						*rv.unresolvedRoutes = append(*rv.unresolvedRoutes, UnresolvedRouteCall{
							Pattern:       route.Pattern,
							RawModuleExpr: "import(...)",
							Reason:        "dynamic import() argument is not a static string",
						})
						return rv
					}
				} else {
					// Some other function call as the module argument
					funcName := "<unknown>"
					if innerIdent, ok := innerCall.X.(*js.Var); ok {
						funcName = string(innerIdent.Data)
					}
					*rv.unresolvedRoutes = append(*rv.unresolvedRoutes, UnresolvedRouteCall{
						Pattern:       route.Pattern,
						RawModuleExpr: funcName + "(...)",
						Reason:        "module argument is a function call, which cannot be statically analyzed",
					})
					return rv
				}
			} else {
				val, ok := extractStringArg(1)
				if !ok {
					// Not a string, not a var, not a call - some other expression
					*rv.unresolvedRoutes = append(*rv.unresolvedRoutes, UnresolvedRouteCall{
						Pattern:       route.Pattern,
						RawModuleExpr: "<expression>",
						Reason:        "module argument is not a static string, variable, or import() call",
					})
					return rv
				}
				route.Module = val
				resolved = true
			}

			if !resolved {
				return rv
			}
		}

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

func (rv *routeCallVisitor) Exit(n js.INode) {}

func extractRouteCalls(code string, sourceFile string) ([]RouteCall, []UnresolvedRouteCall, error) {
	parsedAST, err := js.Parse(parse.NewInputString(code), js.Options{})
	if err != nil {
		return nil, nil, fmt.Errorf("parse JS/TS: %w", err)
	}

	routeFuncNames := make(map[string]bool)
	tracker := &importTracker{imports: make(map[string]string)}

	for _, stmt := range parsedAST.BlockStmt.List {
		switch s := stmt.(type) {
		case *js.ImportStmt:
			importPath := ""
			if s.Module != nil {
				importPath = strings.Trim(string(s.Module), `"'`+"`")
			}
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
			for _, binding := range s.List {
				if varBinding, ok := binding.Binding.(*js.Var); ok {
					varName := string(varBinding.Data)
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
	var unresolvedRoutes []UnresolvedRouteCall

	visitor := &routeCallVisitor{
		routeFuncNames:   routeFuncNames,
		routes:           &routes,
		unresolvedRoutes: &unresolvedRoutes,
		importTracker:    tracker,
		sourceFile:       sourceFile,
	}
	js.Walk(visitor, parsedAST)

	return routes, unresolvedRoutes, nil
}

func parseClientRoutes(v *vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
	code, err := os.ReadFile(v.Config.ClientRouteDefsFile)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

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
			v.Log.Error(fmt.Sprintf("esbuild error: %s", msg.Text))
		}
		return nil, errors.New("esbuild transform failed")
	}

	transformedCode := importRegex.ReplaceAllString(string(minifyResult.Code), "$1")

	routeCalls, unresolvedRoutes, err := extractRouteCalls(transformedCode, v.Config.ClientRouteDefsFile)
	if err != nil {
		return nil, fmt.Errorf("extract route calls: %w", err)
	}

	// Warn about unresolved routes
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

	paths := make(map[string]*vormaruntime.Path, len(routeCalls))
	routesDir := filepath.Dir(v.Config.ClientRouteDefsFile)

	for _, rc := range routeCalls {
		resolvedModulePath, err := filepath.Rel(".", filepath.Join(routesDir, rc.Module))
		if err != nil {
			v.Log.Warn(fmt.Sprintf("could not make module path relative: %s", err))
			resolvedModulePath = rc.Module
		}
		modulePath := filepath.ToSlash(resolvedModulePath)

		if _, err := os.Stat(modulePath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("component module does not exist: %s (pattern: %s)", modulePath, rc.Pattern)
			}
			return nil, fmt.Errorf("access component module %s: %w", modulePath, err)
		}

		paths[rc.Pattern] = &vormaruntime.Path{
			OriginalPattern: rc.Pattern,
			SrcPath:         modulePath,
			ExportKey:       rc.Key,
			ErrorExportKey:  rc.ErrorKey,
		}
	}

	return paths, nil
}
