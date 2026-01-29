package vormaruntime

import (
	"fmt"
	"html/template"
	"path/filepath"
)

// ReloadRoutesFromDisk reloads route configuration from JSON files on disk.
// Called by Process B when Process A has regenerated route artifacts.
// This does NOT regenerate TypeScript - that's done by Process A.
func (v *Vorma) ReloadRoutesFromDisk() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	pathsFile, err := v.getBasePaths_StageOneOrTwo(true)
	if err != nil {
		return fmt.Errorf("load paths from disk: %w", err)
	}

	v._paths = pathsFile.Paths
	v._buildID = pathsFile.BuildID
	v._routeManifestFile = pathsFile.RouteManifestFile

	v.routes().Sync(v._paths)

	v.Log.Info("Routes reloaded from disk", "buildID", v._buildID)
	return nil
}

// ReloadTemplateFromDisk re-parses the HTML template from disk.
func (v *Vorma) ReloadTemplateFromDisk() error {
	srcPath := filepath.Join(v.Wave.GetPrivateStaticDir(), v.Config.HTMLTemplateLocation)
	tmpl, err := template.ParseFiles(srcPath)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v._rootTemplate = tmpl

	v.Log.Info("HTML template reloaded")
	return nil
}
