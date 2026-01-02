package vorma

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"path"

	"github.com/vormadev/vorma/kit/mux"
)

func (h *Vorma) Init() *Vorma {
	isDev := h.GetIsDev()
	if err := h.initInner(isDev); err != nil {
		wrapped := fmt.Errorf("error initializing Vorma: %w", err)
		if isDev {
			Log.Error(wrapped.Error())
		} else {
			panic(wrapped)
		}
	} else {
		Log.Info("Vorma initialized", "build id", h._buildID)
	}
	return h
}

// RUNTIME! Gets called from the handler maker, which gets called by the user's router init function.
func (h *Vorma) validateAndDecorateNestedRouter(nestedRouter *mux.NestedRouter) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if nestedRouter == nil {
		panic("nestedRouter is nil")
	}
	for _, p := range h._paths {
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

func (h *Vorma) initInner(isDev bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h._isDev = isDev
	privateFS, err := h.Wave.GetPrivateFS()
	if err != nil {
		wrapped := fmt.Errorf("could not get private fs: %w", err)
		Log.Error(wrapped.Error())
		return wrapped
	}
	h._privateFS = privateFS
	pathsFile, err := h.getBasePaths_StageOneOrTwo(isDev)
	if err != nil {
		wrapped := fmt.Errorf("could not get base paths: %w", err)
		Log.Error(wrapped.Error())
		return wrapped
	}
	h._buildID = pathsFile.BuildID
	if h._paths == nil {
		h._paths = make(map[string]*Path, len(pathsFile.Paths))
	}
	for _, p := range pathsFile.Paths {
		h._paths[p.OriginalPattern] = p
	}
	h._clientEntrySrc = pathsFile.ClientEntrySrc
	h._clientEntryOut = pathsFile.ClientEntryOut
	h._clientEntryDeps = pathsFile.ClientEntryDeps
	h._depToCSSBundleMap = pathsFile.DepToCSSBundleMap
	if h._depToCSSBundleMap == nil {
		h._depToCSSBundleMap = make(map[string]string)
	}
	h._routeManifestFile = pathsFile.RouteManifestFile
	tmpl, err := template.ParseFS(h._privateFS, h.Wave.GetVormaHTMLTemplateLocation())
	if err != nil {
		return fmt.Errorf("error parsing root template: %w", err)
	}
	h._rootTemplate = tmpl
	if h.getHeadElUniqueRules != nil {
		headElsInstance.InitUniqueRules(h.getHeadElUniqueRules())
	} else {
		headElsInstance.InitUniqueRules(nil)
	}
	h._serverAddr = fmt.Sprintf(":%d", h.MustGetPort())
	return nil
}

func (h *Vorma) getBasePaths_StageOneOrTwo(isDev bool) (*PathsFile, error) {
	fileToUse := VormaPathsStageOneJSONFileName
	if !isDev {
		fileToUse = VormaPathsStageTwoJSONFileName
	}
	pathsFile := PathsFile{}
	file, err := h._privateFS.Open(path.Join("vorma_out", fileToUse))
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
