package vormabuild

import (
	"encoding/json"
	"fmt"

	"github.com/vormadev/vorma/vorma2/internal/constants"
	"github.com/vormadev/vorma/wave/wavebuild"
)

func (s *plugin_state) route_fast_path_hook(
	ctx *wavebuild.PluginCtx,
) (*wavebuild.PluginResult, error) {
	if s.build_hook_active.Load() {
		return nil, nil
	}

	release := ctx.BlockAt(
		wavebuild.CheckpointOrder(
			wavebuild.Checkpoint_2_UserlandPublicFilemapReady,
		),
	)

	cfg := s.cfg
	if cfg == nil {
		return nil, fmt.Errorf("vorma2 config not parsed")
	}

	build := s.app.ForBuild()
	var routes []discovered_route

	err := func() error {
		defer release()

		discovered_routes, err := discover_client_routes(
			cfg.client_route_definition_patterns,
		)
		if err != nil {
			return fmt.Errorf("route discovery: %w", err)
		}
		routes = discovered_routes

		// Build route manifest from frontend routes
		manifest := make(map[string]int, len(routes))
		for _, r := range routes {
			flag := 0
			if build.LoadersRouter().HasTaskHandler(r.pattern) {
				flag = 1
			}
			manifest[r.pattern] = flag
		}

		// include server-only loader patterns in the manifest
		all_loader_routes := build.LoadersRouter().AllRoutes()
		for pattern := range all_loader_routes {
			if _, exists := manifest[pattern]; !exists {
				flag := 0
				if build.LoadersRouter().HasTaskHandler(pattern) {
					flag = 1
				}
				manifest[pattern] = flag
			}
		}

		manifest_json, err := json.Marshal(manifest)
		if err != nil {
			return fmt.Errorf("marshalling route manifest: %w", err)
		}
		if err := ctx.ContributePublicFiles(map[string][]byte{
			constants.PUBLIC_ROUTE_MANIFEST_FILENAME: manifest_json,
		}); err != nil {
			return fmt.Errorf("contributing route manifest: %w", err)
		}

		return nil
	}()
	if err != nil {
		return nil, err
	}

	if err := write_index_ts(cfg, routes, s.app, ctx); err != nil {
		return nil, fmt.Errorf(
			"writing %s: %w",
			constants.GENERATED_TS_INDEX_FILENAME,
			err,
		)
	}

	if err := ctx.WaitFor(
		wavebuild.CheckpointOrder(wavebuild.Checkpoint_3_FullPublicFilemapFinalized),
	); err != nil {
		return nil, err
	}

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
