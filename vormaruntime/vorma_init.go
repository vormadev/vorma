package vormaruntime

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"path"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/wave"
)

// Init initializes Vorma. Panics on error in production; logs in dev.
func (v *Vorma) Init() {
	isDev := wave.GetIsDev()
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

// InitWithDefaultRouter initializes Vorma and returns a configured mux.Router.
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

func (v *Vorma) validateAndDecorateNestedRouter(nestedRouter *mux.NestedRouter) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if nestedRouter == nil {
		panic("nestedRouter is nil")
	}
	for _, p := range v._paths {
		if !nestedRouter.IsRegistered(p.OriginalPattern) {
			mux.RegisterNestedPatternWithoutHandler(nestedRouter, p.OriginalPattern)
		}
	}
}

func (v *Vorma) initInner(isDev bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v._isDev = isDev

	privateFS, err := v.Wave.GetPrivateFS()
	if err != nil {
		return fmt.Errorf("could not get private fs: %w", err)
	}
	v._privateFS = privateFS

	pathsFile, err := v.getBasePaths_StageOneOrTwo(isDev)
	if err != nil {
		return fmt.Errorf("could not get base paths: %w", err)
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

	tmpl, err := template.ParseFS(v._privateFS, v.Config.HTMLTemplateLocation)
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

	file, err := v._privateFS.Open(path.Join("vorma_out", fileToUse))
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", fileToUse, err)
	}
	defer file.Close()

	var pathsFile PathsFile
	if err := json.NewDecoder(file).Decode(&pathsFile); err != nil {
		return nil, fmt.Errorf("could not decode %s: %w", fileToUse, err)
	}
	return &pathsFile, nil
}

// PrettyPrintFS is a debug utility for fs.FS instances.
func PrettyPrintFS(fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			fmt.Println(p)
		} else {
			fmt.Printf("%s (%s)\n", p, d.Type())
		}
		return nil
	})
}
