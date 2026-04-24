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
	ActionsMountRoot     string
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

func (v *Vorma) PublicURL(src_path string) (string, error) {
	pub_fm, err := v.public_filemap()
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

func (v *Vorma) ClientBuildID() (string, error) {
	if IsDev() {
		manifest, err := v.manifest()
		if err != nil {
			return "", fmt.Errorf("error getting manifest: %w", err)
		}
		return manifest.to_client_build_id()
	}
	return v.client_build_id, nil
}

func (v *Vorma) CriticalCSSContentSha256() (string, error) {
	el, err := v.critical_css_el()
	if err != nil {
		return "", fmt.Errorf("error getting critical CSS element: %w", err)
	}
	hash, err := htmlutil.ComputeContentSha256(el)
	if err != nil {
		return "", fmt.Errorf("error hashing critical CSS element: %w", err)
	}
	return hash, nil
}

func (v *Vorma) PublicFS() (fs.FS, error) {
	return fs.Sub(v.static_fs, "public")
}

func (v *Vorma) public_static_base_path() (string, error) {
	manifest, err := v.manifest()
	if err != nil {
		return "", fmt.Errorf("error getting manifest: %w", err)
	}
	return manifest.PublicStaticBasePath, nil
}

func (v *Vorma) critical_css_el() (*htmlutil.Element, error) {
	manifest, err := v.manifest()
	if err != nil {
		return nil, fmt.Errorf("error getting manifest: %w", err)
	}
	return &htmlutil.Element{
		Tag:                 "style",
		AttributesKnownSafe: map[string]string{"id": Critical_CSS_EL_ID},
		DangerousInnerHTML:  manifest.CriticalCSS,
	}, nil
}

func (v *Vorma) main_css_el() (*htmlutil.Element, error) {
	pub_fm, err := v.public_filemap()
	if err != nil {
		return nil, fmt.Errorf("error getting public filemap: %w", err)
	}
	url, ok := pub_fm[Main_CSS_Filename]
	if !ok {
		return nil, fmt.Errorf(
			"main CSS file %s not found in manifest public filemap",
			Main_CSS_Filename,
		)
	}
	return &htmlutil.Element{
		Tag: "link",
		AttributesKnownSafe: map[string]string{
			"rel":  "stylesheet",
			"id":   Main_CSS_El_ID,
			"href": url,
		},
	}, nil
}

func (v *Vorma) public_filemap() (map[string]string, error) {
	manifest, err := v.manifest()
	if err != nil {
		return nil, fmt.Errorf("error getting manifest: %w", err)
	}
	return manifest.PublicFilemap, nil
}

func (v *Vorma) final_public_filepaths() (*set.Set[string], error) {
	if IsDev() {
		return v.make_final_public_filepaths()
	}
	return v._final_public_filepaths, nil
}

func (v *Vorma) make_final_public_filepaths() (*set.Set[string], error) {
	manifest, err := v.manifest()
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

func (v *Vorma) manifest() (*Manifest, error) {
	if IsDev() {
		return read_manifest(v.static_fs)
	}
	return v._manifest, nil
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
