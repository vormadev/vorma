// Package vormapublicfilemap writes the generated TypeScript public-asset map
// consumed by Vorma runtime and build tooling.
//
// Keeping this logic in one package avoids drift between full builds and
// dev-time reload paths.
package vormapublicfilemap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vormadev/vorma/internal/artifactio"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
)

type canonicalPublicFileMapValue struct {
	Dist string `json:"dist"`
}

// WriteTypeScriptFromCanonicalWaveOutput reads Wave's canonical public filemap
// JSON artifact and rewrites Vorma's generated `filemap.ts`.
func WriteTypeScriptFromCanonicalWaveOutput(
	parsedWaveConfig waveconfig.ParsedConfig,
	outputDirectoryPath string,
) error {
	canonicalPublicFileMap, readError := readCanonicalWavePublicFileMap(
		parsedWaveConfig,
	)
	if readError != nil {
		return readError
	}

	if mkdirError := os.MkdirAll(outputDirectoryPath, 0o755); mkdirError != nil {
		return fmt.Errorf("create public filemap output directory: %w", mkdirError)
	}

	typeScriptContent := renderPublicFileMapTypeScript(canonicalPublicFileMap)
	typeScriptPath := filepath.Join(
		outputDirectoryPath,
		runtimepaths.GeneratedTypeScriptPublicFileMapFileName,
	)
	if writeError := artifactio.WriteFileAtomically(
		typeScriptPath,
		[]byte(typeScriptContent),
		0o644,
	); writeError != nil {
		return fmt.Errorf("write public filemap TypeScript output: %w", writeError)
	}

	legacyJSONPath := filepath.Join(
		outputDirectoryPath,
		waveartifacts.PublicFileMapJSONName,
	)
	if removeError := os.Remove(legacyJSONPath); removeError != nil &&
		!errors.Is(removeError, os.ErrNotExist) {
		return fmt.Errorf("remove legacy public filemap JSON output: %w", removeError)
	}

	return nil
}

func readCanonicalWavePublicFileMap(
	parsedWaveConfig waveconfig.ParsedConfig,
) (map[string]string, error) {
	if parsedWaveConfig == nil {
		return nil, errors.New("wave build config is nil")
	}
	if parsedWaveConfig.Dist() == nil {
		return nil, errors.New("wave dist config is nil")
	}

	publicFileMapRefPath := strings.TrimSpace(parsedWaveConfig.Dist().PublicFileMapRef())
	if publicFileMapRefPath == "" {
		return nil, errors.New("wave dist public filemap ref path is empty")
	}
	refFileBytes, readRefError := os.ReadFile(publicFileMapRefPath)
	if readRefError != nil {
		return nil, fmt.Errorf("read canonical public filemap ref: %w", readRefError)
	}

	referencedFileName := strings.TrimSpace(string(refFileBytes))
	if referencedFileName == "" {
		return nil, errors.New("canonical public filemap ref is empty")
	}
	referencedFileName = filepath.ToSlash(filepath.Clean(referencedFileName))
	if referencedFileName == "." ||
		referencedFileName == ".." ||
		strings.HasPrefix(referencedFileName, "../") {
		return nil, fmt.Errorf(
			"canonical public filemap ref escapes static public root: %q",
			referencedFileName,
		)
	}

	staticPublicRootPath := strings.TrimSpace(parsedWaveConfig.Dist().StaticPublic())
	if staticPublicRootPath == "" {
		return nil, errors.New("wave dist static public path is empty")
	}

	canonicalJSONPath := filepath.Join(
		staticPublicRootPath,
		filepath.FromSlash(referencedFileName),
	)
	relativePathFromPublicRoot, relativePathError := filepath.Rel(
		staticPublicRootPath,
		canonicalJSONPath,
	)
	if relativePathError != nil {
		return nil, fmt.Errorf(
			"resolve canonical public filemap path %q: %w",
			referencedFileName,
			relativePathError,
		)
	}
	normalizedRelativePathFromPublicRoot := filepath.ToSlash(
		relativePathFromPublicRoot,
	)
	if normalizedRelativePathFromPublicRoot == ".." ||
		strings.HasPrefix(normalizedRelativePathFromPublicRoot, "../") {
		return nil, fmt.Errorf(
			"canonical public filemap path escapes static public root: %q",
			referencedFileName,
		)
	}

	canonicalJSONBytes, readJSONError := os.ReadFile(canonicalJSONPath)
	if readJSONError != nil {
		return nil, fmt.Errorf(
			"read canonical public filemap JSON %q: %w",
			referencedFileName,
			readJSONError,
		)
	}

	var canonicalRawMap map[string]canonicalPublicFileMapValue
	if unmarshalError := json.Unmarshal(canonicalJSONBytes, &canonicalRawMap); unmarshalError != nil {
		return nil, fmt.Errorf(
			"parse canonical public filemap JSON %q: %w",
			referencedFileName,
			unmarshalError,
		)
	}

	publicFileMap := make(map[string]string, len(canonicalRawMap))
	for sourcePath, rawValue := range canonicalRawMap {
		trimmedDistName := strings.TrimSpace(rawValue.Dist)
		if trimmedDistName == "" {
			return nil, fmt.Errorf(
				"canonical public filemap entry %q is missing dist output",
				sourcePath,
			)
		}
		publicFileMap[sourcePath] = trimmedDistName
	}
	return publicFileMap, nil
}

func renderPublicFileMapTypeScript(publicFileMap map[string]string) string {
	sortedKeys := make([]string, 0, len(publicFileMap))
	for key := range publicFileMap {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	typeScriptBuilder := &strings.Builder{}
	typeScriptBuilder.WriteString("/////// Auto-generated by Vorma. Do not edit.\n\n")
	typeScriptBuilder.WriteString("export const staticPublicAssetMap = {\n")
	for _, key := range sortedKeys {
		typeScriptBuilder.WriteString(
			fmt.Sprintf("\t%q: %q,\n", key, publicFileMap[key]),
		)
	}
	typeScriptBuilder.WriteString("} as const;\n")
	return typeScriptBuilder.String()
}
