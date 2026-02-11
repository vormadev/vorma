package vormaruntime

import (
	"fmt"
	"html/template"
	"path/filepath"
)

func (v *Vorma) guardDevOnlyReload(op string) error {
	if !v.GetIsDevMode() {
		return fmt.Errorf("%s is dev-only and cannot run outside dev mode", op)
	}
	return nil
}

// devReloadRoutesFromDisk reloads route configuration from JSON files on disk.
// Called by Process B when Process A has regenerated route artifacts.
// This does NOT regenerate TypeScript - that's done by Process A.
func (v *Vorma) devReloadRoutesFromDisk() error {
	if err := v.guardDevOnlyReload("route reload"); err != nil {
		return err
	}

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

// devReloadTemplateFromDisk re-parses the HTML template from disk.
func (v *Vorma) devReloadTemplateFromDisk() error {
	if err := v.guardDevOnlyReload("template reload"); err != nil {
		return err
	}

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
