package vormarun

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/searchparams"
	"github.com/vormadev/vorma/kit/set"
)

// <dist>/.vorma/static/vorma.manifest.dev.json
const ManifestStaticOutDev = "vorma.manifest.dev.json"

// <dist>/.vorma/static/vorma.manifest.prod.json
const ManifestStaticOutProd = "vorma.manifest.prod.json"

type ClientModule struct {
	URL           string
	DepURLs       []string
	CSSBundleURLs []string
}

type ClientCoreAssets struct {
	ModuleURL string
	WasmURL   string
}

// Client Build ID is a hash of the JSON-marshaled Manifest
type Manifest struct {
	VormaVersion string

	// Dev-time
	Dev_ViteServerPort int    `json:",omitempty"`
	Dev_MuxPort        int    `json:",omitempty"`
	Dev_RefreshToken   string `json:",omitempty"`

	// Normalized config values needed at runtime
	PublicStaticBasePath string
	APIMountRoot         string
	UIVariant            string
	RootHTMLTemplateHash string

	// Static build outputs
	PublicFilepaths []string
	PublicFilemap   map[string]string
	CriticalCSS     string
	SearchSchemas   map[string]searchparams.Schema

	// Dev:    {
	//             URL: "http://localhost:5173/frontend/entry.tsx",
	//             DepURLs: [],
	//             CSSBundleURLs: [],
	//         }
	// Prod:   {
	//             URL: "/public/vorma_out_vite_frontend_entry-abc123.js",
	//             DepURLs: ["/public/vorma_out_vite_chunk_some_dep-abc123.js"],
	//             CSSBundleURLs: ["/public/vorma_out_vite_chunk_some_dep-abc123.css"],
	//         }
	ClientEntry      ClientModule
	ClientCoreAssets *ClientCoreAssets `json:",omitempty"`

	// Dev:    {
	//             "/user/:id": {
	//                 URL: "http://localhost:5173/frontend/user.tsx",
	//                 DepURLs: [],
	//                 CSSBundleURLs: [],
	//             },
	//         }
	// Prod:   {
	//             "/user/:id": {
	//                 URL: "/public/vorma_out_vite_frontend_user-abc123.js",
	//                 DepURLs: ["/public/vorma_out_vite_chunk_some_dep-abc123.js"],
	//                 CSSBundleURLs: ["/public/vorma_out_vite_chunk_some_dep-abc123.css"],
	//             },
	//         }
	ClientRoutes map[string]ClientModule
}

func (m Manifest) to_client_build_id() (string, error) {
	b, err := jsonutil.Serialize(m)
	if err != nil {
		return "", fmt.Errorf("error serializing manifest for hashing: %w", err)
	}
	hash := cryptoutil.Sha256Hash(b)
	b32 := bytesutil.ToBase32Raw(hash)
	return strings.ToLower(b32)[:24], nil
}

func (inst *Instance) PublicURL(src_path string) (string, error) {
	if IsBuild() {
		return "", nil
	}
	if err := inst.init(); err != nil {
		return "", fmt.Errorf("error initializing instance: %w", err)
	}
	pub_fm, err := inst.public_filemap()
	if err != nil {
		return "", fmt.Errorf("error getting public filemap: %w", err)
	}
	clean := strings.TrimPrefix(strings.TrimSpace(src_path), "/")
	url, ok := pub_fm[clean]
	if !ok {
		return "", fmt.Errorf("file %s not found in manifest public filemap", src_path)
	}
	return url, nil
}

func (inst *Instance) ClientBuildID() (string, error) {
	if IsBuild() {
		return "", nil
	}
	if err := inst.init(); err != nil {
		return "", fmt.Errorf("error initializing instance: %w", err)
	}
	if IsDev() {
		manifest, err := inst.manifest()
		if err != nil {
			return "", fmt.Errorf("error getting manifest: %w", err)
		}
		return manifest.to_client_build_id()
	}
	return inst.client_build_id_cache, nil
}

func (inst *Instance) CriticalCSSContentSha256() (string, error) {
	if IsBuild() {
		return "", nil
	}
	if err := inst.init(); err != nil {
		return "", fmt.Errorf("error initializing instance: %w", err)
	}
	el, err := inst.critical_css_el()
	if err != nil {
		return "", fmt.Errorf("error getting critical CSS element: %w", err)
	}
	hash, err := htmlutil.ComputeContentSha256(el)
	if err != nil {
		return "", fmt.Errorf("error hashing critical CSS element: %w", err)
	}
	return hash, nil
}

func (inst *Instance) PublicFS() (fs.FS, error) {
	if IsBuild() {
		return nil, nil
	}
	if err := inst.init(); err != nil {
		return nil, fmt.Errorf("error initializing instance: %w", err)
	}
	return fs.Sub(inst.static_fs, "public")
}

func (inst *Instance) public_static_base_path() (string, error) {
	manifest, err := inst.manifest()
	if err != nil {
		return "", fmt.Errorf("error getting manifest: %w", err)
	}
	return manifest.PublicStaticBasePath, nil
}

func (inst *Instance) critical_css_el() (*htmlutil.Element, error) {
	manifest, err := inst.manifest()
	if err != nil {
		return nil, fmt.Errorf("error getting manifest: %w", err)
	}
	return &htmlutil.Element{
		Tag:                 "style",
		AttributesKnownSafe: map[string]string{"id": Critical_CSS_EL_ID},
		DangerousInnerHTML:  manifest.CriticalCSS,
	}, nil
}

func (inst *Instance) public_filemap() (map[string]string, error) {
	manifest, err := inst.manifest()
	if err != nil {
		return nil, fmt.Errorf("error getting manifest: %w", err)
	}
	return manifest.PublicFilemap, nil
}

func (inst *Instance) final_public_filepaths() (*set.Set[string], error) {
	if IsDev() {
		return inst.make_final_public_filepaths()
	}
	return inst.final_public_filepaths_cache, nil
}

func (inst *Instance) make_final_public_filepaths() (*set.Set[string], error) {
	manifest, err := inst.manifest()
	if err != nil {
		return nil, fmt.Errorf("error getting manifest: %w", err)
	}
	filepaths := set.New[string]()
	for _, p := range manifest.PublicFilepaths {
		filepaths.Add(p)
	}
	return filepaths, nil
}

func (inst *Instance) manifest() (*Manifest, error) {
	if IsDev() {
		return read_manifest(inst.static_fs)
	}
	return inst.manifest_cache, nil
}

func read_manifest(static_fs fs.FS) (*Manifest, error) {
	out := ManifestStaticOutProd
	if IsDev() {
		out = ManifestStaticOutDev
	}
	manifest_bytes, err := fs.ReadFile(static_fs, out)
	if err != nil {
		return nil, fmt.Errorf("error reading manifest from static_fs: %w", err)
	}
	manifest, err := jsonutil.Parse[*Manifest](manifest_bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing manifest JSON: %w", err)
	}
	return manifest, nil
}
