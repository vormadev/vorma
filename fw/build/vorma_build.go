package build

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/vormadev/vorma/fw/runtime"
	"github.com/vormadev/vorma/fw/types"
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave"
	wavebuild "github.com/vormadev/vorma/wave/tooling"
)

func registerVormaSchema(v *runtime.Vorma) {
	cfg := v.Wave.GetParsedConfig()
	if cfg.FrameworkSchemaExtensions == nil {
		cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	cfg.FrameworkSchemaExtensions["Vorma"] = Vorma_Schema
}

// RunBuildCLI parses flags and runs the build or dev server.
func RunBuildCLI(v *runtime.Vorma) error {
	dev := flag.Bool("dev", false, "run in development mode")
	hook := flag.Bool("hook", false, "run build hook only (internal use)")
	_ = flag.Bool("no-binary", false, "skip go binary compilation (internal use)")
	flag.Parse()

	if *hook {
		registerVormaSchema(v)
		injectDefaultWatchPatterns(v)

		if err := buildInner(v, &buildInnerOptions{isDev: *dev}); err != nil {
			return err
		}

		// In prod hook mode, also run Vite and post-processing.
		// This matches the old architecture where everything ran inside
		// the hook subprocess, keeping all state in one process.
		if !*dev {
			wb := wavebuild.NewBuilder(v.Wave.GetParsedConfig(), v.Wave.Logger())
			defer wb.Close()

			if err := wb.ViteProdBuild(); err != nil {
				return err
			}

			if err := postViteProdBuild(v); err != nil {
				return err
			}
		}

		return nil
	}

	return Build(v, *dev)
}

// Build performs a full Vorma build.
func Build(v *runtime.Vorma, isDev bool) error {
	registerVormaSchema(v)
	injectDefaultWatchPatterns(v)

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

func injectDefaultWatchPatterns(v *runtime.Vorma) {
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
		)
	}
}

func getDefaultWatchPatterns(v *runtime.Vorma) []wave.WatchedFile {
	var patterns []wave.WatchedFile

	// Route definitions file
	if clientRouteDefsFile := v.Config.ClientRouteDefsFile; clientRouteDefsFile != "" {
		patterns = append(patterns, wave.WatchedFile{
			Pattern: clientRouteDefsFile,
			OnChangeHooks: []wave.OnChangeHook{{
				Callback: func(string) error {
					return rebuildRoutesOnly(v)
				},
				Strategy: &wave.OnChangeStrategy{
					HttpEndpoint:   runtime.DevReloadRoutesPath,
					WaitForApp:     true,
					WaitForVite:    true,
					ReloadBrowser:  true,
					FallbackAction: wave.FallbackRestartNoGo,
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
			Pattern: templatePath,
			OnChangeHooks: []wave.OnChangeHook{{
				Strategy: &wave.OnChangeStrategy{
					HttpEndpoint:   runtime.DevReloadTemplatePath,
					WaitForApp:     true,
					WaitForVite:    true,
					ReloadBrowser:  true,
					FallbackAction: wave.FallbackRestartNoGo,
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

type buildInnerOptions struct {
	isDev bool
}

func buildInner(v *runtime.Vorma, opts *buildInnerOptions) error {
	start := time.Now()

	v.SetIsDev(opts.isDev)

	if opts.isDev {
		buildID, err := id.New(16)
		if err != nil {
			return fmt.Errorf("generate build ID: %w", err)
		}
		v.Lock()
		v.UnsafeSetBuildID("dev_" + buildID)
		v.Unlock()
		runtime.Log.Info("START building Vorma (DEV)")
	} else {
		runtime.Log.Info("START building Vorma (PROD)")
	}

	// Parse client routes
	paths, err := parseClientRoutes(v.Config)
	if err != nil {
		return fmt.Errorf("parse client routes: %w", err)
	}

	// Sync routes
	v.Lock()
	v.Routes().Sync(paths)
	v.Unlock()

	// Clean Vorma-generated files
	if err := cleanStaticPublicOutDir(v.Wave.GetStaticPublicOutDir()); err != nil {
		return fmt.Errorf("clean static public out dir: %w", err)
	}

	// Write filemap.ts
	wb := wavebuild.NewBuilder(v.Wave.GetParsedConfig(), v.Wave.Logger())
	defer wb.Close()

	if err := wb.WritePublicFileMapTS(v.Config.TSGenOutDir); err != nil {
		return fmt.Errorf("write public file map TS: %w", err)
	}

	// Write all route artifacts
	v.Lock()
	err = writeRouteArtifacts(v)
	v.Unlock()
	if err != nil {
		return fmt.Errorf("write route artifacts: %w", err)
	}

	runtime.Log.Info("DONE building Vorma",
		"buildID", v.GetBuildID(),
		"routes found", len(v.GetPathsSnapshot()),
		"duration", time.Since(start),
	)

	return nil
}

func cleanStaticPublicOutDir(staticPublicOutDir string) error {
	fileInfo, err := os.Stat(staticPublicOutDir)
	if err != nil {
		if os.IsNotExist(err) {
			runtime.Log.Warn(fmt.Sprintf("static public out dir does not exist: %s", staticPublicOutDir))
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
		if strings.HasPrefix(baseName, types.VormaVitePrehashedFilePrefix) ||
			strings.HasPrefix(baseName, types.VormaRouteManifestPrefix) {
			return os.Remove(path)
		}
		return nil
	})
}

func writePathsToDisk_StageOne(v *runtime.Vorma) error {
	pathsJSONOut := filepath.Join(
		v.Wave.GetStaticPrivateOutDir(),
		types.VormaOutDirname,
		types.VormaPathsStageOneJSONFileName,
	)
	if err := os.MkdirAll(filepath.Dir(pathsJSONOut), os.ModePerm); err != nil {
		return err
	}

	pathsAsJSON, err := json.MarshalIndent(types.PathsFile{
		Stage:             "one",
		Paths:             v.UnsafeGetPaths(),
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           v.UnsafeGetBuildID(),
		RouteManifestFile: v.UnsafeGetRouteManifestFile(),
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

type importTracker struct {
	imports map[string]string
}

type routeCallVisitor struct {
	routeFuncNames map[string]bool
	routes         *[]RouteCall
	importTracker  *importTracker
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
			if varRef, ok := arg.Value.(*js.Var); ok {
				if importPath, exists := rv.importTracker.imports[string(varRef.Data)]; exists {
					route.Module = importPath
				} else {
					return rv
				}
			} else if innerCall, ok := arg.Value.(*js.CallExpr); ok {
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
				val, ok := extractStringArg(1)
				if !ok {
					return rv
				}
				route.Module = val
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

func extractRouteCalls(code string) ([]RouteCall, error) {
	parsedAST, err := js.Parse(parse.NewInputString(code), js.Options{})
	if err != nil {
		return nil, fmt.Errorf("parse JS/TS: %w", err)
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
	visitor := &routeCallVisitor{
		routeFuncNames: routeFuncNames,
		routes:         &routes,
		importTracker:  tracker,
	}
	js.Walk(visitor, parsedAST)

	return routes, nil
}

func parseClientRoutes(config *types.VormaConfig) (map[string]*types.Path, error) {
	code, err := os.ReadFile(config.ClientRouteDefsFile)
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
			runtime.Log.Error(fmt.Sprintf("esbuild error: %s", msg.Text))
		}
		return nil, errors.New("esbuild transform failed")
	}

	transformedCode := importRegex.ReplaceAllString(string(minifyResult.Code), "$1")

	routeCalls, err := extractRouteCalls(transformedCode)
	if err != nil {
		return nil, fmt.Errorf("extract route calls: %w", err)
	}

	paths := make(map[string]*types.Path, len(routeCalls))
	routesDir := filepath.Dir(config.ClientRouteDefsFile)

	for _, rc := range routeCalls {
		resolvedModulePath, err := filepath.Rel(".", filepath.Join(routesDir, rc.Module))
		if err != nil {
			runtime.Log.Warn(fmt.Sprintf("could not make module path relative: %s", err))
			resolvedModulePath = rc.Module
		}
		modulePath := filepath.ToSlash(resolvedModulePath)

		if _, err := os.Stat(modulePath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("component module does not exist: %s (pattern: %s)", modulePath, rc.Pattern)
			}
			return nil, fmt.Errorf("access component module %s: %w", modulePath, err)
		}

		paths[rc.Pattern] = &types.Path{
			OriginalPattern: rc.Pattern,
			SrcPath:         modulePath,
			ExportKey:       rc.Key,
			ErrorExportKey:  rc.ErrorKey,
		}
	}

	return paths, nil
}
