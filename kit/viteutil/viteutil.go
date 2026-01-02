package viteutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/netutil"
	"github.com/vormadev/vorma/kit/stringsutil"
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

func ReadManifest(manifestPath string) (Manifest, error) {
	manifest := make(Manifest)
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(contents, &manifest)
	return manifest, err
}

// FindAllDependencies recursively finds all of a module's dependencies
// according to the provided Vite manifest. The importPath arg
// should be a key in the manifest map.
func FindAllDependencies(manifest Manifest, importPath string) []string {
	seen := make(map[string]bool)
	var result []string

	var recurse func(ip string)
	recurse = func(ip string) {
		if seen[ip] {
			return
		}
		seen[ip] = true
		result = append(result, ip)

		if chunk, exists := manifest[ip]; exists {
			for _, imp := range chunk.Imports {
				recurse(imp)
			}
		}
	}

	recurse(importPath)

	cleanResults := make([]string, 0, len(result)+1)
	for _, res := range result {
		if chunk, exists := manifest[res]; exists {
			cleanResults = append(cleanResults, path.Base(chunk.File))
		}
	}

	if chunk, exists := manifest[importPath]; exists {
		if !slices.Contains(cleanResults, path.Base(chunk.File)) {
			cleanResults = append(cleanResults, path.Base(chunk.File))
		}
	}

	return cleanResults
}

// FindRelativeEntrypointPath finds the manifest key for a given entry point file
func FindRelativeEntrypointPath(manifest Manifest, entrypointToFind string) (string, error) {
	for key, chunk := range manifest {
		if chunk.IsEntry && path.Base(chunk.File) == path.Base(entrypointToFind) {
			return key, nil
		}
	}
	return "", errors.New("entrypoint not found")
}

type Variant string

var Variants = struct {
	React Variant
	Other Variant
}{
	React: "react",
	Other: "other",
}

type ToDevScriptsOptions struct {
	ClientEntry string
	Variant     Variant
}

func ToDevScripts(options ToDevScriptsOptions) (template.HTML, error) {
	var htmlBuilder strings.Builder
	var err error

	port := GetVitePortStr()

	if options.Variant == Variants.React {
		var b stringsutil.Builder

		b.Linef(`import RefreshRuntime from "http://localhost:%s/@react-refresh";`, port)
		b.Line("RefreshRuntime.injectIntoGlobalHook(window);")
		b.Line("window.$RefreshReg$ = () => {};")
		b.Line("window.$RefreshSig$ = () => (type) => type;")
		b.Line("window.__vite_plugin_react_preamble_installed__ = true;")

		err = htmlutil.RenderElementToBuilder(&htmlutil.Element{
			Tag:                 "script",
			AttributesKnownSafe: map[string]string{"type": "module"},
			DangerousInnerHTML:  b.String(),
		}, &htmlBuilder)
		if err != nil {
			return "", fmt.Errorf("could not render vite script: %w", err)
		}
	}

	err = htmlutil.RenderModuleScriptToBuilder(
		fmt.Sprintf("http://localhost:%s/@vite/client", port), &htmlBuilder,
	)
	if err != nil {
		return "", fmt.Errorf("could not render vite script: %w", err)
	}

	err = htmlutil.RenderModuleScriptToBuilder(
		fmt.Sprintf(
			"http://localhost:%s/%s",
			port,
			stripPrecedingSlash(options.ClientEntry),
		),
		&htmlBuilder,
	)
	if err != nil {
		return "", fmt.Errorf("could not render vite script: %w", err)
	}

	return template.HTML(htmlBuilder.String()), nil
}

func stripPrecedingSlash(s string) string {
	if strings.HasPrefix(s, "/") {
		return s[1:]
	}
	return s
}

const PortEnvName = "__VITE_PORT"

func InitPort(defaultPort int) (int, error) {
	vitePort, err := netutil.GetFreePort(5199)
	if err != nil {
		return 0, err
	}

	err = os.Setenv(PortEnvName, fmt.Sprintf("%d", vitePort))
	if err != nil {
		return 0, err
	}

	return vitePort, nil
}

func GetVitePortStr() string {
	return os.Getenv(PortEnvName)
}
