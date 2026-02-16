package vormaruntime

import (
	"fmt"
	"html/template"
)

func (v *Vorma) guardDevOnlyReload(op string) error {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v._isDev {
		return fmt.Errorf("%s is dev-only and cannot run outside dev mode", op)
	}
	return nil
}

func (v *Vorma) guardDevOnlyReloadLocked(op string) error {
	if !v._isDev {
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

	v.mu.RLock()
	privateFS := v._privateFS
	v.mu.RUnlock()

	pathsFile, err := v.getBasePathsFromFS(privateFS, true)
	if err != nil {
		return fmt.Errorf("load paths from disk: %w", err)
	}
	runtimeArtifacts, err := buildRuntimeRouteArtifacts(pathsFile)
	if err != nil {
		return fmt.Errorf("build runtime route artifacts: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.guardDevOnlyReloadLocked("route reload"); err != nil {
		return err
	}

	v.transitionLifecycleStateLocked(
		runtimeLifecycleStateReloadingRoutes,
		"dev route artifacts commit start",
		"",
	)
	v.commitRouteArtifactsLocked(runtimeArtifacts, true, routeArtifactCommitModeDevReload)
	v.transitionLifecycleStateLocked(runtimeLifecycleStateReady, "dev route artifacts commit complete", "")

	v.Log.Info("Routes reloaded from disk", "buildID", v._buildID)
	return nil
}

// devReloadTemplateFromDisk re-parses the HTML template from disk.
func (v *Vorma) devReloadTemplateFromDisk() error {
	if err := v.guardDevOnlyReload("template reload"); err != nil {
		return err
	}

	v.mu.RLock()
	privateFS := v._privateFS
	rootTemplateLocation := v.Config.HTMLTemplateLocation
	v.mu.RUnlock()
	if privateFS == nil {
		return fmt.Errorf("private fs is nil")
	}

	tmpl, err := template.ParseFS(privateFS, rootTemplateLocation)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.guardDevOnlyReloadLocked("template reload"); err != nil {
		return err
	}
	v.transitionLifecycleStateLocked(
		runtimeLifecycleStateReloadingHTML,
		"dev html template commit start",
		"",
	)
	v.commitRootTemplateLocked(tmpl)
	v.transitionLifecycleStateLocked(runtimeLifecycleStateReady, "dev html template commit complete", "")

	v.Log.Info("HTML template reloaded")
	return nil
}
