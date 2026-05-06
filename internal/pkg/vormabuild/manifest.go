package vormabuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/pkg/npm"
	"github.com/vormadev/vorma/internal/pkg/viteutil"
	"github.com/vormadev/vorma/internal/pkg/vormarun"
	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/fsutil"
)

func (cfg vorma_cfg) manifest_json_out(is_dev bool) string {
	out := vormarun.ManifestStaticOutProd
	if is_dev {
		out = vormarun.ManifestStaticOutDev
	}
	return filepath.Join(cfg.vorma_out(), "static", out)
}

func (rs *run_state) write_manifest() error {
	cfg, err := rs.get_config()
	if err != nil {
		return fmt.Errorf("error getting config: %w", err)
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	var ts_entry_cm vormarun.ClientModule
	ts_routes := make(map[string]vormarun.ClientModule, len(rs.ts_modules))

	vite_server_port := rs.vite_server_sv.port()

	if rs.is_dev {
		var to_url = func(p string) string {
			return fmt.Sprintf(
				"http://%s:%d/%s",
				dev_loopback_host,
				vite_server_port,
				filepath.ToSlash(p),
			)
		}
		ts_entry_rel, err := filepath.Rel(cfg.js_package_manager_dir(), cfg.ts_entry())
		if err != nil {
			return fmt.Errorf("error getting relative path for TS entry: %w", err)
		}
		ts_entry_cm = vormarun.ClientModule{
			URL:           to_url(ts_entry_rel),
			DepURLs:       []string{},
			CSSBundleURLs: []string{},
		}
		for pattern, r := range rs.ts_modules {
			ip_rel, err := filepath.Rel(
				cfg.js_package_manager_dir(),
				fsutil.SysNorm(r.ImportPath),
			)
			if err != nil {
				return fmt.Errorf(
					"error getting relative path for TS route (pattern: %s, import path: %s): %w",
					pattern,
					r.ImportPath,
					err,
				)
			}
			ts_routes[pattern] = vormarun.ClientModule{
				URL:           to_url(ip_rel),
				DepURLs:       []string{},
				CSSBundleURLs: []string{},
			}
		}
	} else {
		vite_manifest, err := viteutil.ReadViteManifest(cfg.prod_tmp_vite_manifest_out())
		if err != nil {
			return fmt.Errorf("error reading Vite manifest: %w", err)
		}

		ts_entry_src := filepath.ToSlash(cfg.ts_entry())
		ts_entry_cm, err = cfg.to_client_module(vite_manifest, ts_entry_src)
		if err != nil {
			return fmt.Errorf("error processing TS entry module for manifest: %w", err)
		}

		for pattern, r := range rs.ts_modules {
			src := filepath.ToSlash(r.ImportPath)
			ts_routes[pattern], err = cfg.to_client_module(vite_manifest, src)
			if err != nil {
				return fmt.Errorf("error processing TS route module for manifest (pattern: %s, import path: %s): %w", pattern, r.ImportPath, err)
			}
		}
	}

	root_html_tmpl_hash := cryptoutil.Sha256Hash(
		[]byte(strings.TrimSpace(cfg.root_html_template())),
	)

	vorma_version, err := npm.Version()
	if err != nil {
		return fmt.Errorf("error getting Vorma version: %w", err)
	}

	m := vormarun.Manifest{
		VormaVersion: vorma_version,

		PublicStaticBasePath: cfg.public_static_base_path(),
		APIMountRoot:         cfg.actions_mount_root(),
		UIVariant:            cfg.ui_variant(),
		RootHTMLTemplateHash: bytesutil.ToBase64(root_html_tmpl_hash),

		PublicFilemap: rs.pub_fm,
		CriticalCSS:   rs.critical_css,
		SearchSchemas: rs.search_schemas,

		ClientEntry:  ts_entry_cm,
		ClientRoutes: ts_routes,
	}

	if rs.is_dev {
		m.Dev_ViteServerPort = vite_server_port
		m.Dev_MuxPort = rs.dev_mux_port
		m.Dev_RefreshToken = rs.dev_refresh_token
	}

	if err := write_json_to_file(m, cfg.manifest_json_out(rs.is_dev)); err != nil {
		return fmt.Errorf("error writing vorma manifest json: %w", err)
	}

	if err := cfg.remove_prod_tmp_vite_manifest(); err != nil {
		return fmt.Errorf("error removing temporary Vite manifest: %w", err)
	}

	rs.manifest = &m

	return nil
}

func (cfg vorma_cfg) to_client_module(
	manifest viteutil.ViteManifest,
	_import_path string,
) (vormarun.ClientModule, error) {
	base := cfg.public_static_base_path()

	// Needs to be made relative to js_package_manager_dir given that
	// that is where the vite command is being run from
	import_path, err := filepath.Rel(cfg.js_package_manager_dir(), _import_path)
	if err != nil {
		return vormarun.ClientModule{}, fmt.Errorf("error getting relative import path: %w", err)
	}

	own_chunk, ok := manifest[import_path]
	if !ok {
		return vormarun.ClientModule{}, fmt.Errorf(
			"error finding module in Vite manifest: %s", import_path,
		)
	}
	own_file := base + own_chunk.File

	deps_res := manifest.FindAllDeps(import_path)
	mod_urls := make([]string, len(deps_res.Modules))
	for i, m := range deps_res.Modules {
		mod_urls[i] = base + m
	}
	css_bundle_urls := make([]string, len(deps_res.CSSBundles))
	for i, m := range deps_res.CSSBundles {
		css_bundle_urls[i] = base + m
	}

	cm := vormarun.ClientModule{
		URL:           own_file,
		DepURLs:       mod_urls,
		CSSBundleURLs: css_bundle_urls,
	}
	return cm, nil
}
