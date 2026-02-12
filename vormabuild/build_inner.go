package vormabuild

import (
	"fmt"
	"time"

	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/vormaruntime"
	wavebuild "github.com/vormadev/vorma/wave/tooling"
)

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
