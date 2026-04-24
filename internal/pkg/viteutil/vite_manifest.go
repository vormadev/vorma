package viteutil

import (
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/set"
)

type ViteManifestChunk struct {
	Src            string   `json:"src"`
	File           string   `json:"file"`
	CSS            []string `json:"css"`
	Assets         []string `json:"assets"`
	IsEntry        bool     `json:"isEntry"`
	Name           string   `json:"name"`
	IsDynamicEntry bool     `json:"isDynamicEntry"`
	Imports        []string `json:"imports"`
	DynamicImports []string `json:"dynamicImports"`
}

type ViteManifest map[string]ViteManifestChunk

func ReadViteManifest(manifest_path string) (ViteManifest, error) {
	return jsonutil.ParseFromFile[ViteManifest](manifest_path)
}

type DepsResult struct {
	ImportPath string
	Modules    []string
	CSSBundles []string
}

// FindAllDeps recursively finds all of a module's dependencies
// according to the provided Vite manifest. The import_path arg
// should be a key in the manifest map. Final result set includes
// the root module and all of its direct and indirect dependencies.
func (m ViteManifest) FindAllDeps(import_path string) DepsResult {
	seen := set.New[string]()
	var results []string

	var recurse func(ip string)
	recurse = func(ip string) {
		if seen.Has(ip) {
			return
		}
		seen.Add(ip)
		results = append(results, ip)

		if chunk, exists := m[ip]; exists {
			for _, imp := range chunk.Imports {
				recurse(imp)
			}
		}
	}

	recurse(import_path)

	seen_modules := set.New[string]()
	modules := make([]string, 0, len(results))
	seen_css_bundles := set.New[string]()
	css_bundles := make([]string, 0, len(results))

	for _, res := range results {
		if chunk, exists := m[res]; exists {
			if !seen_modules.Has(chunk.File) {
				seen_modules.Add(chunk.File)
				modules = append(modules, chunk.File)
			}
			for _, css := range chunk.CSS {
				if !seen_css_bundles.Has(css) {
					seen_css_bundles.Add(css)
					css_bundles = append(css_bundles, css)
				}
			}
		}
	}

	return DepsResult{
		ImportPath: import_path,
		Modules:    modules,
		CSSBundles: css_bundles,
	}
}
