package esbuildutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

func CollectErrors(result esbuild.BuildResult) error {
	var errors []string
	for _, msg := range result.Errors {
		errors = append(errors, msg.Text)
	}
	if len(errors) > 0 {
		return fmt.Errorf("esbuild errors: %v", strings.Join(errors, "\n"))
	}
	return nil
}

type ESBuildMetafileSubset struct {
	Inputs map[string]struct {
		Imports []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"imports"`
	} `json:"inputs"`
	Outputs map[string]struct {
		Imports []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"imports"`
		EntryPoint string `json:"entryPoint"`
		CSSBundle  string `json:"cssBundle"`
	} `json:"outputs"`
}

const (
	KindDymanicImport = "dynamic-import"
)

func UnmarshalOutput(result esbuild.BuildResult) (*ESBuildMetafileSubset, error) {
	m := &ESBuildMetafileSubset{}
	err := json.Unmarshal([]byte(result.Metafile), m)
	return m, err
}

// FindAllDependencies recursively finds all of an es module's dependencies
// according to the provided metafile, which is a compatible, marshalable
// subset of esbuild's standard json metafile output. The importPath arg
// should be a key in the metafile's Outputs map.
func FindAllDependencies(metafile *ESBuildMetafileSubset, importPath string) []string {
	seen := make(map[string]bool)
	var result []string

	var recurse func(ip string)
	recurse = func(ip string) {
		if seen[ip] {
			return
		}
		seen[ip] = true
		result = append(result, ip)

		if output, exists := metafile.Outputs[ip]; exists {
			for _, imp := range output.Imports {
				if imp.Kind == KindDymanicImport {
					continue
				}
				recurse(imp.Path)
			}
		}
	}

	recurse(importPath)

	cleanResults := make([]string, 0, len(result)+1)
	for _, res := range result {
		cleanResults = append(cleanResults, filepath.Base(res))
	}
	if !slices.Contains(cleanResults, filepath.Base(importPath)) {
		cleanResults = append(cleanResults, filepath.Base(importPath))
	}
	return cleanResults
}

func FindRelativeEntrypointPath(metafile *ESBuildMetafileSubset, entrypointToFind string) (string, error) {
	for key, output := range metafile.Outputs {
		if output.EntryPoint == entrypointToFind {
			return key, nil
		}
	}
	return "", errors.New("entrypoint not found")
}
