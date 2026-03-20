package vormabuild

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vormadev/vorma/vorma2/internal/constants"
	"github.com/vormadev/vorma/wave/wavebuild"
)

func (s *plugin_state) route_fast_path_hook(
	ctx *wavebuild.PluginCtx,
) (*wavebuild.PluginResult, error) {
	// skip on initial build — main build hook handles everything
	if ctx.IsInitialBuild() {
		return nil, nil
	}

	release := ctx.BlockAt(
		wavebuild.CheckpointOrder(
			wavebuild.Checkpoint_2_UserlandPublicFilemapReady,
		),
	)

	// skip if Go files also changed in this batch — main hook
	// handles the full rebuild and contributes the manifest
	for _, p := range ctx.EvtPaths() {
		if strings.HasSuffix(p.Str(), ".go") {
			return nil, nil
		}
	}

	cfg := s.cfg
	if cfg == nil {
		return nil, fmt.Errorf("vorma2 config not parsed")
	}

	build := s.app.ForBuild()

	// frontend route discovery
	routes, err := discover_routes(cfg.client_route_definition_patterns)
	if err != nil {
		return nil, fmt.Errorf("route discovery: %w", err)
	}

	// route manifest
	manifest := make(map[string]int, len(routes))
	for _, r := range routes {
		flag := 0
		if build.LoadersRouter().HasTaskHandler(r.pattern) {
			flag = 1
		}
		manifest[r.pattern] = flag
	}
	manifest_json, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshalling route manifest: %w", err)
	}
	if err := ctx.ContributePublicFiles(map[string][]byte{
		constants.PUBLIC_ROUTE_MANIFEST_FILENAME: manifest_json,
	}); err != nil {
		return nil, fmt.Errorf("contributing route manifest: %w", err)
	}

	release()

	// index.ts
	if err := write_index_ts(cfg, routes, s.app, ctx); err != nil {
		return nil, fmt.Errorf("writing index.ts: %w", err)
	}

	// wait for public filemap finalization (manifest gets hashed)
	if err := ctx.WaitFor(
		wavebuild.CheckpointOrder(wavebuild.Checkpoint_3_FullPublicFilemapFinalized),
	); err != nil {
		return nil, err
	}

	// rewrite runtime snapshot (dev only)
	snapshot, err := build_dev_snapshot(cfg, routes, ctx.VitePort())
	if err != nil {
		return nil, fmt.Errorf("building dev snapshot: %w", err)
	}
	if err := write_runtime_snapshot(ctx, s, snapshot, true); err != nil {
		return nil, fmt.Errorf("writing runtime snapshot: %w", err)
	}

	return nil, nil
}

func (s *plugin_state) filemap_refresh_hook(
	ctx *wavebuild.PluginCtx,
) (*wavebuild.PluginResult, error) {
	cfg := s.cfg
	if cfg == nil {
		return nil, fmt.Errorf("vorma2 config not parsed")
	}

	// wait for public filemap finalization
	if err := ctx.WaitFor(
		wavebuild.CheckpointOrder(wavebuild.Checkpoint_3_FullPublicFilemapFinalized),
	); err != nil {
		return nil, err
	}

	public_fm := ctx.ReadPublicFileMap()
	if err := write_gen_filemaps(cfg, public_fm.Filemap); err != nil {
		return nil, fmt.Errorf("writing filemap.ts: %w", err)
	}

	return nil, nil
}
