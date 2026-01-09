package vorma

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"path/filepath"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/wave"
)

// Inits Vorma.
// Panics on error in production; logs error in dev mode.
func (v *Vorma) Init() {
	isDev := v.GetIsDev()
	if err := v.initInner(isDev); err != nil {
		wrapped := fmt.Errorf("error initializing Vorma: %w", err)
		if isDev {
			Log.Error(wrapped.Error())
		} else {
			panic(wrapped)
		}
	} else {
		Log.Info("Vorma initialized", "build id", v._buildID)
	}
}

// Inits Vorma and returns a default mux.Router with all loaders and actions registered.
// To use a different router, call Vorma.Init() and then register the handlers manually
// with whatever third-party router you may want to use (see the implementation of this
// function for reference on how to do that).
func (v *Vorma) InitWithDefaultRouter() *mux.Router {
	v.Init()
	r := mux.NewRouter()
	loaders, actions := v.Loaders(), v.Actions()
	r.RegisterHandler("GET", loaders.HandlerMountPattern(), loaders.Handler())
	for m := range actions.SupportedMethods() {
		r.RegisterHandler(m, actions.HandlerMountPattern(), actions.Handler())
	}
	return r
}

// RUNTIME!
// Gets called from the handler maker, which gets called by the user's router init function.
func (v *Vorma) validateAndDecorateNestedRouter(nestedRouter *mux.NestedRouter) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if nestedRouter == nil {
		panic("nestedRouter is nil")
	}
	for _, p := range v._paths {
		_is_already_registered := nestedRouter.IsRegistered(p.OriginalPattern)
		if !_is_already_registered {
			mux.RegisterNestedPatternWithoutHandler(nestedRouter, p.OriginalPattern)
		}
	}
}

func PrettyPrintFS(fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			fmt.Println(path)
		} else {
			fmt.Printf("%s (%s)\n", path, d.Type())
		}
		return nil
	})
}

// Internal__GetCurrentNPMVersion returns the version of the Vorma NPM package
// that should be installed. Used by bootstrap.
func Internal__GetCurrentNPMVersion() string {
	return "0.0.1" // Placeholder: This would typically be dynamic or hardcoded to release version
}

func (v *Vorma) initInner(isDev bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v._isDev = isDev

	// Inject runtime configuration into Wave
	if v.config.TSGenOutDir != "" {
		v.Wave.SetPublicFileMapOutDir(v.config.TSGenOutDir)
		// Ignore the generated filemap.ts to prevent infinite loops
		// Use public constant from wave package
		v.Wave.AddIgnoredPatterns([]string{
			filepath.Join(v.config.TSGenOutDir, wave.PublicFileMapTSName),
		})
	}

	privateFS, err := v.Wave.GetPrivateFS()
	if err != nil {
		wrapped := fmt.Errorf("could not get private fs: %w", err)
		Log.Error(wrapped.Error())
		return wrapped
	}
	v._privateFS = privateFS
	pathsFile, err := v.getBasePaths_StageOneOrTwo(isDev)
	if err != nil {
		wrapped := fmt.Errorf("could not get base paths: %w", err)
		Log.Error(wrapped.Error())
		return wrapped
	}
	v._buildID = pathsFile.BuildID
	if v._paths == nil {
		v._paths = make(map[string]*Path, len(pathsFile.Paths))
	}
	for _, p := range pathsFile.Paths {
		v._paths[p.OriginalPattern] = p
	}
	v._clientEntrySrc = pathsFile.ClientEntrySrc
	v._clientEntryOut = pathsFile.ClientEntryOut
	v._clientEntryDeps = pathsFile.ClientEntryDeps
	v._depToCSSBundleMap = pathsFile.DepToCSSBundleMap
	if v._depToCSSBundleMap == nil {
		v._depToCSSBundleMap = make(map[string]string)
	}
	v._routeManifestFile = pathsFile.RouteManifestFile
	tmpl, err := template.ParseFS(v._privateFS, v.config.HTMLTemplateLocation)
	if err != nil {
		return fmt.Errorf("error parsing root template: %w", err)
	}
	v._rootTemplate = tmpl
	if v.getHeadElUniqueRules != nil {
		headEls := headels.New()
		v.getHeadElUniqueRules(headEls)
		headElsInstance.InitUniqueRules(headEls)
	} else {
		headElsInstance.InitUniqueRules(nil)
	}
	v._serverAddr = fmt.Sprintf(":%d", v.MustGetPort())
	return nil
}

func (v *Vorma) getBasePaths_StageOneOrTwo(isDev bool) (*PathsFile, error) {
	fileToUse := VormaPathsStageOneJSONFileName
	if !isDev {
		fileToUse = VormaPathsStageTwoJSONFileName
	}
	pathsFile := PathsFile{}
	file, err := v._privateFS.Open(path.Join("vorma_out", fileToUse))
	if err != nil {
		wrapped := fmt.Errorf("could not open %s: %v", fileToUse, err)
		Log.Error(wrapped.Error())
		return nil, wrapped
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&pathsFile)
	if err != nil {
		wrapped := fmt.Errorf("could not decode %s: %v", fileToUse, err)
		Log.Error(wrapped.Error())
		return nil, wrapped
	}
	return &pathsFile, nil
}
