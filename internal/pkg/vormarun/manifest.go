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

// Client Build ID is a hash of the JSON-marshaled Manifest
type Manifest struct {
	VormaVersion string

	// Dev-time
	Dev_ViteServerPort int
	Dev_MuxPort        int
	Dev_RefreshToken   string

	// Normalized config values needed at runtime
	PublicStaticBasePath string
	APIMountRoot         string
	UIVariant            string
	RootHTMLTemplateHash string

	// Static build outputs
	PublicFilemap map[string]string
	CriticalCSS   string
	SearchSchemas map[string]searchparams.Schema

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
	ClientEntry ClientModule

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

func (instance *Instance) PublicURL(src_path string) (string, error) {
	if IsBuild() {
		return "", nil
	}
	if err := instance.init(); err != nil {
		return "", fmt.Errorf("error initializing instance: %w", err)
	}
	pub_fm, err := instance.public_filemap()
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

func (instance *Instance) ClientBuildID() (string, error) {
	if IsBuild() {
		return "", nil
	}
	if err := instance.init(); err != nil {
		return "", fmt.Errorf("error initializing instance: %w", err)
	}
	if IsDev() {
		manifest, err := instance.manifest()
		if err != nil {
			return "", fmt.Errorf("error getting manifest: %w", err)
		}
		return manifest.to_client_build_id()
	}
	return instance.client_build_id_cache, nil
}

func (instance *Instance) CriticalCSSContentSha256() (string, error) {
	if IsBuild() {
		return "", nil
	}
	if err := instance.init(); err != nil {
		return "", fmt.Errorf("error initializing instance: %w", err)
	}
	el, err := instance.critical_css_el()
	if err != nil {
		return "", fmt.Errorf("error getting critical CSS element: %w", err)
	}
	hash, err := htmlutil.ComputeContentSha256(el)
	if err != nil {
		return "", fmt.Errorf("error hashing critical CSS element: %w", err)
	}
	return hash, nil
}

func (instance *Instance) PublicFS() (fs.FS, error) {
	if IsBuild() {
		return nil, nil
	}
	if err := instance.init(); err != nil {
		return nil, fmt.Errorf("error initializing instance: %w", err)
	}
	return fs.Sub(instance.static_fs, "public")
}

func (instance *Instance) public_static_base_path() (string, error) {
	manifest, err := instance.manifest()
	if err != nil {
		return "", fmt.Errorf("error getting manifest: %w", err)
	}
	return manifest.PublicStaticBasePath, nil
}

func (instance *Instance) critical_css_el() (*htmlutil.Element, error) {
	manifest, err := instance.manifest()
	if err != nil {
		return nil, fmt.Errorf("error getting manifest: %w", err)
	}
	return &htmlutil.Element{
		Tag:                 "style",
		AttributesKnownSafe: map[string]string{"id": Critical_CSS_EL_ID},
		DangerousInnerHTML:  manifest.CriticalCSS,
	}, nil
}

func (instance *Instance) public_filemap() (map[string]string, error) {
	manifest, err := instance.manifest()
	if err != nil {
		return nil, fmt.Errorf("error getting manifest: %w", err)
	}
	return manifest.PublicFilemap, nil
}

func (instance *Instance) final_public_filepaths() (*set.Set[string], error) {
	if IsDev() {
		return instance.make_final_public_filepaths()
	}
	return instance.final_public_filepaths_cache, nil
}

func (instance *Instance) make_final_public_filepaths() (*set.Set[string], error) {
	manifest, err := instance.manifest()
	if err != nil {
		return nil, fmt.Errorf("error getting manifest: %w", err)
	}
	filepaths := set.New[string]()
	for _, v := range manifest.PublicFilemap {
		filepaths.Add(v)
	}
	for _, cm := range manifest.ClientEntry.DepURLs {
		filepaths.Add(cm)
	}
	for _, cm := range manifest.ClientEntry.CSSBundleURLs {
		filepaths.Add(cm)
	}
	for _, route := range manifest.ClientRoutes {
		for _, cm := range route.DepURLs {
			filepaths.Add(cm)
		}
		for _, cm := range route.CSSBundleURLs {
			filepaths.Add(cm)
		}
	}
	return filepaths, nil
}

func (instance *Instance) manifest() (*Manifest, error) {
	if IsDev() {
		return read_manifest(instance.static_fs)
	}
	return instance.manifest_cache, nil
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
