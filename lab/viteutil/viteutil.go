package viteutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/netutil"
	"github.com/vormadev/vorma/lab/stringsutil"
)

const defaultDevVitePort = 5199

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
	normalizedEntrypointToFind := normalizeViteManifestPath(entrypointToFind)

	for key, chunk := range manifest {
		if !chunk.IsEntry {
			continue
		}

		if normalizeViteManifestPath(chunk.Src) == normalizedEntrypointToFind {
			return key, nil
		}

		if normalizeViteManifestPath(key) == normalizedEntrypointToFind {
			return key, nil
		}
	}

	return "", errors.New("entrypoint not found")
}

func normalizeViteManifestPath(pathValue string) string {
	trimmedPathValue := strings.TrimSpace(pathValue)
	trimmedPathValue = strings.TrimPrefix(trimmedPathValue, "/")
	trimmedPathValue = strings.TrimPrefix(trimmedPathValue, "./")
	if trimmedPathValue == "" {
		return ""
	}
	return path.Clean(trimmedPathValue)
}

type Variant string

const (
	VariantReact Variant = "react"
	VariantOther Variant = "other"
)

type ToDevScriptsOptions struct {
	ClientEntry string
	Variant     Variant
}

func ToDevScripts(options ToDevScriptsOptions) (template.HTML, error) {
	var htmlBuilder strings.Builder
	var err error

	port, resolvePortError := resolveVitePortStrict(strings.TrimSpace(GetVitePortStr()))
	if resolvePortError != nil {
		return "", resolvePortError
	}

	if options.Variant == VariantReact {
		var b stringsutil.Builder

		b.Linef(`import RefreshRuntime from "http://127.0.0.1:%s/@react-refresh";`, port)
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
		fmt.Sprintf("http://127.0.0.1:%s/@vite/client", port), &htmlBuilder,
	)
	if err != nil {
		return "", fmt.Errorf("could not render vite script: %w", err)
	}

	err = htmlutil.RenderModuleScriptToBuilder(
		fmt.Sprintf(
			"http://127.0.0.1:%s/%s",
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

func resolveVitePortStrict(port string) (string, error) {
	if strings.TrimSpace(port) == "" {
		return "", errors.New("__VITE_PORT is not set")
	}
	parsedPort, parseError := strconv.Atoi(port)
	if parseError != nil || parsedPort <= 0 || parsedPort > 65535 {
		return "", fmt.Errorf("__VITE_PORT is invalid: %q", port)
	}
	return strconv.Itoa(parsedPort), nil
}

func stripPrecedingSlash(s string) string {
	if strings.HasPrefix(s, "/") {
		return s[1:]
	}
	return s
}

const PortEnvName = "__VITE_PORT"

func InitPort(defaultPort int) (int, error) {
	if defaultPort <= 0 || defaultPort > 65535 {
		defaultPort = defaultDevVitePort
	}

	vitePort, err := netutil.GetFreePort(defaultPort)
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
