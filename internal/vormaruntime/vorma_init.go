package vormaruntime

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/wave"
)

// MustInit initializes Vorma. Panics on error.
func (v *Vorma) MustInit() {
	isDev := wave.GetIsDev()
	if err := v.initInner(isDev); err != nil {
		panic(fmt.Errorf("error initializing Vorma: %w", err))
	}
	v.Log.Info("Vorma initialized", "build_id", v._buildID)
}

// MustInitWithDefaultRouter initializes Vorma and returns a configured mux.Router.
func (v *Vorma) MustInitWithDefaultRouter() *mux.Router {
	v.MustInit()
	r := mux.NewRouter()
	loaders, actions := v.Loaders(), v.Actions()
	r.AddHTTPHandler("GET", loaders.HandlerMountPattern(), loaders.Handler())
	for m := range actions.SupportedMethods() {
		r.AddHTTPHandler(m, actions.HandlerMountPattern(), actions.Handler())
	}
	if v.IsDevMode() {
		r.AddHTTPHandler(
			http.MethodPost,
			v.DevReloadRoutesEndpointPath(),
			actions.Handler(),
		)
		r.AddHTTPHandler(
			http.MethodPost,
			v.DevReloadTemplateEndpointPath(),
			actions.Handler(),
		)
	}
	return r
}

func (v *Vorma) validateAndDecorateNestedRouter(
	nestedRouter *mux.NestedRouter,
) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if nestedRouter == nil {
		panic("nestedRouter is nil")
	}
	for mapKeyPattern, pathEntry := range v._paths {
		if pathEntry == nil {
			panic(
				fmt.Sprintf(
					"paths entry for pattern %q is nil",
					mapKeyPattern,
				),
			)
		}
		nestedRouter.AddNestedPatternWithoutHandlerIfMissing(
			pathEntry.OriginalPattern,
		)
	}
}

func (v *Vorma) ensureLoaderPatternsRegisteredForHandler() {
	v.validateAndDecorateNestedRouter(v.LoadersRouter().NestedRouter)
}

func (v *Vorma) initInner(isDev bool) error {
	privateFS, err := v.Wave.PrivateFS()
	if err != nil {
		return fmt.Errorf("could not get private fs: %w", err)
	}

	pathsFile, err := v.getBasePathsFromFS(privateFS, isDev)
	if err != nil {
		return fmt.Errorf("could not get base paths: %w", err)
	}
	runtimeArtifacts, err := buildRuntimeRouteArtifacts(pathsFile)
	if err != nil {
		return fmt.Errorf("could not build runtime route artifacts: %w", err)
	}

	tmpl, err := template.ParseFS(privateFS, v.Config.HTMLTemplateLocation)
	if err != nil {
		return fmt.Errorf("error parsing root template: %w", err)
	}

	var headElsUniqueRules *headels.HeadEls
	if v.getHeadDedupeKeys != nil {
		headEls := headels.New()
		v.getHeadDedupeKeys(headEls)
		headElsUniqueRules = headEls
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	wasInitialized := v._paths != nil

	v.setIsDevModeLocked(isDev)
	v._privateFS = privateFS
	v.transitionLifecycleStateLocked(
		v.lifecycleStateForRouteCommitLocked(),
		"init route artifacts commit start",
		"",
	)
	v.commitRouteArtifactsLocked(
		runtimeArtifacts,
		wasInitialized,
		routeArtifactCommitModeInit,
	)

	v.commitRootTemplateLocked(tmpl)
	if v.headElsInst == nil {
		v.headElsInst = headels.NewInstance("vorma")
	}

	if headElsUniqueRules != nil {
		v.headElsInst.InitUniqueRules(headElsUniqueRules)
	} else {
		v.headElsInst.InitUniqueRules(nil)
	}

	v._serverAddr = fmt.Sprintf(":%d", v.MustGetPort())
	v.transitionLifecycleStateLocked(
		runtimeLifecycleStateReady,
		"init commit complete",
		"",
	)
	return nil
}

func (v *Vorma) getBasePaths_StageOneOrTwo(isDev bool) (*PathsFile, error) {
	return v.getBasePathsFromFS(v._privateFS, isDev)
}

func (v *Vorma) getBasePathsFromFS(
	privateFS fs.FS,
	isDev bool,
) (*PathsFile, error) {
	if privateFS == nil {
		return nil, fmt.Errorf("private fs is nil")
	}

	fileToUse := VormaPathsStageOneJSONFileName
	if !isDev {
		fileToUse = VormaPathsStageTwoJSONFileName
	}

	file, err := privateFS.Open(path.Join("vorma_out", fileToUse))
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", fileToUse, err)
	}
	defer file.Close()

	var pathsFile PathsFile
	if err := json.NewDecoder(file).Decode(&pathsFile); err != nil {
		return nil, fmt.Errorf("could not decode %s: %w", fileToUse, err)
	}
	if err := validatePathsFileStructuralIntegrity(&pathsFile); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", fileToUse, err)
	}
	if err := validatePathsFileSemanticIntegrity(&pathsFile, isDev); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", fileToUse, err)
	}
	return &pathsFile, nil
}

func validatePathsFileStructuralIntegrity(pathsFile *PathsFile) error {
	if pathsFile == nil {
		return fmt.Errorf("paths file is nil")
	}
	if pathsFile.Paths == nil {
		return nil
	}
	for mapKeyPattern, pathEntry := range pathsFile.Paths {
		if pathEntry == nil {
			return fmt.Errorf("paths[%q] cannot be null", mapKeyPattern)
		}
		if mapKeyPattern != "" && pathEntry.OriginalPattern == "" {
			return fmt.Errorf(
				"paths[%q].originalPattern is required",
				mapKeyPattern,
			)
		}
		if pathEntry.OriginalPattern != mapKeyPattern {
			return fmt.Errorf(
				"paths[%q].originalPattern=%q does not match key",
				mapKeyPattern,
				pathEntry.OriginalPattern,
			)
		}
	}
	return nil
}

func validatePathsFileSemanticIntegrity(
	pathsFile *PathsFile,
	isDev bool,
) error {
	if pathsFile == nil {
		return fmt.Errorf("paths file is nil")
	}
	if strings.TrimSpace(pathsFile.RouteManifestFile) == "" {
		return fmt.Errorf("routeManifestFile is required")
	}
	if !isDev && strings.TrimSpace(pathsFile.ClientEntryOut) == "" {
		return fmt.Errorf("clientEntryOut is required")
	}

	for mapKeyPattern, pathEntry := range pathsFile.Paths {
		if pathEntry == nil {
			continue
		}
		if pathEntry.SrcPath != "" &&
			strings.TrimSpace(pathEntry.ExportKey) == "" {
			return fmt.Errorf(
				"paths[%q].exportKey is required when srcPath is set",
				mapKeyPattern,
			)
		}
		if !isDev && pathEntry.SrcPath != "" &&
			strings.TrimSpace(pathEntry.OutPath) == "" {
			return fmt.Errorf(
				"paths[%q].outPath is required in production mode",
				mapKeyPattern,
			)
		}
	}

	return nil
}

// PrettyPrintFS is a debug utility for fs.FS instances.
func PrettyPrintFS(fsys fs.FS) error {
	return fs.WalkDir(
		fsys,
		".",
		func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				fmt.Println(p)
			} else {
				fmt.Printf("%s (%s)\n", p, d.Type())
			}
			return nil
		},
	)
}
