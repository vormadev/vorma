package viteutil

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/lab/stringsutil"
)

type ManifestChunk struct {
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

type Manifest map[string]ManifestChunk

func ReadManifest(manifest_path string) (Manifest, error) {
	manifest := make(Manifest)
	contents, err := os.ReadFile(manifest_path)
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(contents, &manifest)
	return manifest, err
}

// FindAllDependencies recursively finds all of a module's dependencies
// according to the provided Vite manifest. The importPath arg
// should be a key in the manifest map.
func FindAllDependencies(manifest Manifest, import_path string) []string {
	seen := &set.Set[string]{}
	var result []string

	var recurse func(ip string)
	recurse = func(ip string) {
		if seen.Has(ip) {
			return
		}
		seen.Add(ip)
		result = append(result, ip)

		if chunk, exists := manifest[ip]; exists {
			for _, imp := range chunk.Imports {
				recurse(imp)
			}
		}
	}

	recurse(import_path)

	clean_results := make([]string, 0, len(result)+1)
	for _, res := range result {
		if chunk, exists := manifest[res]; exists {
			clean_results = append(clean_results, path.Base(chunk.File))
		}
	}

	if chunk, exists := manifest[import_path]; exists {
		if !slices.Contains(clean_results, path.Base(chunk.File)) {
			clean_results = append(clean_results, path.Base(chunk.File))
		}
	}

	return clean_results
}

type Variant string

const (
	VariantReact Variant = "react"
	VariantOther Variant = "other"
)

type ToDevScriptsOptions struct {
	ClientEntry string
	Variant     Variant
	Port        int
}

func ToDevScripts(options ToDevScriptsOptions) (template.HTML, error) {
	var html_builder strings.Builder
	var err error

	if options.ClientEntry == "" {
		return "", fmt.Errorf("ClientEntry is required")
	}
	if options.Port <= 0 {
		return "", fmt.Errorf("Port must be a positive integer")
	}

	if options.Variant == VariantReact {
		var b stringsutil.Builder

		b.Linef(
			`import RefreshRuntime from "http://127.0.0.1:%d/@react-refresh";`,
			options.Port,
		)
		b.Line("RefreshRuntime.injectIntoGlobalHook(window);")
		b.Line("window.$RefreshReg$ = () => {};")
		b.Line("window.$RefreshSig$ = () => (type) => type;")
		b.Line("window.__vite_plugin_react_preamble_installed__ = true;")

		err = htmlutil.RenderElementToBuilder(&htmlutil.Element{
			Tag:                 "script",
			AttributesKnownSafe: map[string]string{"type": "module"},
			DangerousInnerHTML:  b.String(),
		}, &html_builder)
		if err != nil {
			return "", fmt.Errorf("could not render vite script: %w", err)
		}
	}

	err = htmlutil.RenderModuleScriptToBuilder(
		fmt.Sprintf(
			"http://localhost:%d/@vite/client",
			options.Port,
		),
		&html_builder,
	)
	if err != nil {
		return "", fmt.Errorf("could not render vite script: %w", err)
	}

	err = htmlutil.RenderModuleScriptToBuilder(
		fmt.Sprintf(
			"http://localhost:%d/%s",
			options.Port,
			strip_preceding_slash(options.ClientEntry),
		),
		&html_builder,
	)
	if err != nil {
		return "", fmt.Errorf("could not render vite script: %w", err)
	}

	return template.HTML(html_builder.String()), nil
}

func strip_preceding_slash(s string) string {
	if strings.HasPrefix(s, "/") {
		return s[1:]
	}
	return s
}
