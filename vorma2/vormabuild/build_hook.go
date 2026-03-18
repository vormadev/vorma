package vormabuild

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vorma2/internal/constants"
	"github.com/vormadev/vorma/vorma2/internal/types"
	"github.com/vormadev/vorma/wave/wavebuild"
)

func (s *plugin_state) build_hook(
	ctx *wavebuild.PluginCtx,
) (*wavebuild.PluginResult, error) {
	cfg := s.cfg
	if cfg == nil {
		return nil, fmt.Errorf("vorma2 config not parsed")
	}

	is_dev := ctx.IsDev()

	/////// Phase 1: checkpoint 1–2 window
	/////// Route discovery, manifest contribution, index.ts, imports.gen.go

	frontend_routes, err := discover_routes(
		cfg.client_route_definition_patterns,
	)
	if err != nil {
		return nil, fmt.Errorf("frontend route discovery: %w", err)
	}

	backend_pkgs, err := discover_backend_packages(cfg.user_root_dir)
	if err != nil {
		return nil, fmt.Errorf("backend package discovery: %w", err)
	}
	if err := fsutil.EnsureDir(cfg.ts_gen_out_dir.Str()); err != nil {
		return nil, fmt.Errorf("creating gen output dir: %w", err)
	}
	if err := write_imports_gen(cfg.ts_gen_out_dir, backend_pkgs); err != nil {
		return nil, fmt.Errorf("writing imports.gen.go: %w", err)
	}

	build := s.app.ForBuild()

	manifest := make(map[string]int, len(frontend_routes))
	for _, r := range frontend_routes {
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

	if err := write_index_ts(cfg, frontend_routes, s.app, ctx); err != nil {
		return nil, fmt.Errorf("writing index.ts: %w", err)
	}

	/////// Phase 2: after checkpoint 3 (full public filemap finalized)
	/////// filemap.ts, prod vite build, runtime snapshot

	if err := ctx.WaitFor(
		wavebuild.CheckpointOrder(wavebuild.Checkpoint_3_FullPublicFilemapFinalized),
	); err != nil {
		return nil, err
	}

	public_fm := ctx.ReadPublicFileMap()

	if err := write_filemap_ts(cfg, public_fm.Filemap); err != nil {
		return nil, fmt.Errorf("writing filemap.ts: %w", err)
	}

	if is_dev {
		snapshot, err := build_dev_snapshot(cfg, frontend_routes)
		if err != nil {
			return nil, fmt.Errorf("building dev snapshot: %w", err)
		}
		if err := write_runtime_snapshot(ctx, s, snapshot, true); err != nil {
			return nil, fmt.Errorf("writing runtime snapshot: %w", err)
		}
	} else {
		snapshot, err := build_prod_snapshot(cfg, frontend_routes, ctx)
		if err != nil {
			return nil, err
		}
		if err := write_runtime_snapshot(ctx, s, snapshot, false); err != nil {
			return nil, fmt.Errorf("writing runtime snapshot: %w", err)
		}
	}

	return nil, nil
}

func build_dev_snapshot(
	cfg *parsed_vorma_config,
	routes []discovered_route,
) (*types.RuntimeSnapshot, error) {
	build_id, err := id.New(12)
	if err != nil {
		return nil, fmt.Errorf("generating build ID: %w", err)
	}

	snapshot := &types.RuntimeSnapshot{
		BuildID:          build_id,
		RootTemplatePath: cfg.root_template_path,
		UIVariant:        cfg.ui_variant,
		ClientEntryPath: types.SitePublicPath(
			"/" + filepath.ToSlash(cfg.client_entry_path.Str()),
		),
		Paths: make(map[types.RoutePattern]*types.RoutePath, len(routes)),
	}
	for _, r := range routes {
		snapshot.Paths[types.RoutePattern(r.pattern)] = &types.RoutePath{
			OriginalPattern: types.RoutePattern(r.pattern),
			ImportPath: types.SitePublicPath(
				"/" + filepath.ToSlash(r.module_path.Str()),
			),
			ExportKey:      r.export_key,
			ErrorExportKey: r.error_export_key,
		}
	}
	return snapshot, nil
}

func build_prod_snapshot(
	cfg *parsed_vorma_config,
	routes []discovered_route,
	ctx *wavebuild.PluginCtx,
) (*types.RuntimeSnapshot, error) {
	runtime_dir_str := ctx.WaveOutRuntimeStaticDir().Join(
		constants.RUNTIME_DIRNAME,
	).Str()
	manifest_out := filepath.Join(
		runtime_dir_str,
		constants.VITE_MANIFEST_FILENAME,
	)
	vite_out_dir := ctx.WaveOutRuntimeStaticDir().Join("assets", "public")

	if err := ctx.ViteProdBuild(wavebuild.ViteProdBuildOpts{
		OutDir:      vite_out_dir.Str(),
		ManifestOut: manifest_out,
	}); err != nil {
		return nil, fmt.Errorf("vite prod build: %w", err)
	}

	vite_manifest, err := viteutil.ReadManifest(manifest_out)
	if err != nil {
		return nil, fmt.Errorf("reading vite manifest: %w", err)
	}

	public_path_prefix := ctx.PublicPathPrefix()

	build_id, err := id.New(12)
	if err != nil {
		return nil, fmt.Errorf("generating build ID: %w", err)
	}

	snapshot := &types.RuntimeSnapshot{
		BuildID:          build_id,
		RootTemplatePath: cfg.root_template_path,
		UIVariant:        cfg.ui_variant,
		Paths: make(
			map[types.RoutePattern]*types.RoutePath,
			len(routes),
		),
	}

	client_entry_src := filepath.ToSlash(cfg.client_entry_path.Str())
	client_entry_chunk, ok := vite_manifest[client_entry_src]
	if !ok {
		return nil, fmt.Errorf(
			"client entry %q not found in vite manifest", client_entry_src,
		)
	}
	snapshot.ClientEntryPath = site_public_path(
		public_path_prefix, client_entry_chunk.File,
	)
	snapshot.ClientEntryDeps = resolve_vite_deps(
		vite_manifest, client_entry_src, public_path_prefix,
	)
	snapshot.DepToCSSBundleMap = resolve_css_bundle_map(
		vite_manifest, public_path_prefix,
	)

	for _, r := range routes {
		route_src := filepath.ToSlash(r.module_path.Str())
		chunk, ok := vite_manifest[route_src]
		if !ok {
			return nil, fmt.Errorf(
				"route %q module %q not found in vite manifest",
				r.pattern, route_src,
			)
		}
		deps := resolve_vite_deps(
			vite_manifest, route_src, public_path_prefix,
		)
		snapshot.Paths[types.RoutePattern(r.pattern)] = &types.RoutePath{
			OriginalPattern: types.RoutePattern(r.pattern),
			ImportPath:      site_public_path(public_path_prefix, chunk.File),
			ExportKey:       r.export_key,
			ErrorExportKey:  r.error_export_key,
			Deps:            deps,
		}
	}

	return snapshot, nil
}

func write_runtime_snapshot(
	ctx *wavebuild.PluginCtx,
	s *plugin_state,
	snapshot *types.RuntimeSnapshot,
	is_dev bool,
) error {
	runtime_dir := s.vorma_runtime_dir(ctx)
	if err := fsutil.EnsureDir(runtime_dir.Str()); err != nil {
		return err
	}

	filename := constants.RUNTIME_SNAPSHOT_PROD_FILENAME
	if is_dev {
		filename = constants.RUNTIME_SNAPSHOT_DEV_FILENAME
	}
	out_path := filepath.Join(runtime_dir.Str(), filename)

	data, err := jsonutil.SerializePretty(snapshot)
	if err != nil {
		return err
	}
	return os.WriteFile(out_path, append(data, '\n'), 0644)
}

func write_filemap_ts(
	cfg *parsed_vorma_config,
	filemap map[string]string,
) error {
	var b strings.Builder
	b.WriteString("// Code generated by vorma2. DO NOT EDIT.\n\n")
	b.WriteString("export const staticPublicAssetMap = {\n")

	keys := make([]string, 0, len(filemap))
	for k := range filemap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(&b, "\t%q: %q,\n", k, filemap[k])
	}
	b.WriteString("} as const;\n")
	b.WriteString(
		"\nexport type StaticPublicAsset = keyof typeof staticPublicAssetMap;\n",
	)

	out_path := filepath.Join(
		cfg.ts_gen_out_dir.Str(), constants.GENERATED_TS_FILEMAP_FILENAME,
	)
	return os.WriteFile(out_path, []byte(b.String()), 0644)
}

func site_public_path(prefix string, file string) types.SitePublicPath {
	p := prefix + strings.TrimPrefix(file, "/")
	return types.SitePublicPath(p)
}

func resolve_vite_deps(
	manifest viteutil.Manifest,
	entry_key string,
	public_path_prefix string,
) []types.SitePublicPath {
	raw_deps := viteutil.FindAllDependencies(manifest, entry_key)
	if len(raw_deps) == 0 {
		return nil
	}
	deps := make([]types.SitePublicPath, 0, len(raw_deps))
	for _, d := range raw_deps {
		deps = append(deps, site_public_path(public_path_prefix, d))
	}
	return deps
}

func resolve_css_bundle_map(
	manifest viteutil.Manifest,
	public_path_prefix string,
) map[types.SitePublicPath][]types.SitePublicPath {
	result := map[types.SitePublicPath][]types.SitePublicPath{}
	for _, chunk := range manifest {
		if len(chunk.CSS) == 0 {
			continue
		}
		dep_key := site_public_path(public_path_prefix, chunk.File)
		css_paths := make([]types.SitePublicPath, 0, len(chunk.CSS))
		for _, css := range chunk.CSS {
			css_paths = append(
				css_paths, site_public_path(public_path_prefix, css),
			)
		}
		result[dep_key] = css_paths
	}
	return result
}
