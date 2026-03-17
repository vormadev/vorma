package cssbundle

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
)

type BundleOutput struct {
	CSS     string
	Imports []strict.CWDRelPath
}

func Bundle(
	entry_path strict.CWDRelPath,
	url_lookup_map map[string]string,
	public_path_prefix string,
) (BundleOutput, error) {
	src, err := os.ReadFile(entry_path.Str())
	if err != nil {
		return BundleOutput{}, err
	}

	result := esbuild.Build(esbuild.BuildOptions{
		Bundle:            true,
		Write:             false,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Metafile:          true,
		LogLevel:          esbuild.LogLevelSilent,
		Plugins: []esbuild.Plugin{
			url_rewriter(url_lookup_map, public_path_prefix),
		},
		Stdin: &esbuild.StdinOptions{
			Contents:   string(src),
			ResolveDir: entry_path.Dir().Str(),
			Sourcefile: filepath.Base(entry_path.Str()),
			Loader:     esbuild.LoaderCSS,
		},
	})

	if len(result.Errors) > 0 {
		return BundleOutput{}, errors.New(result.Errors[0].Text)
	}
	if len(result.OutputFiles) == 0 {
		return BundleOutput{}, nil
	}

	var meta struct {
		Inputs map[string]struct{} `json:"inputs"`
	}
	if err := json.Unmarshal([]byte(result.Metafile), &meta); err != nil {
		return BundleOutput{}, err
	}

	// The entry file is fed via Stdin, so esbuild records it as
	// "<stdin>" in the metafile inputs. We skip keys starting with
	// "<" below, which means the entry file itself is excluded from
	// the returned Imports slice. This is fine because the caller
	// (classify_evt) checks the entry file path directly via
	// cfg.core.CSSEntryFiles.Critical / NonCritical.
	imports := set.Set[strict.CWDRelPath]{}
	for raw := range meta.Inputs {
		if strings.HasPrefix(raw, "<") {
			continue
		}
		imports.Add(strict.MustNormalizeCWDRelPath(raw))
	}

	// esbuild seems to include this anyway, but add manually to be sure
	imports.Add(entry_path)

	return BundleOutput{
		CSS:     strings.TrimSpace(string(result.OutputFiles[0].Contents)),
		Imports: imports.Slice(),
	}, nil
}

func url_rewriter(
	url_lookup_map map[string]string,
	public_path_prefix string,
) esbuild.Plugin {
	return esbuild.Plugin{
		Name: "url_rewriter",
		Setup: func(build esbuild.PluginBuild) {
			build.OnResolve(
				esbuild.OnResolveOptions{Filter: ".*", Namespace: "file"},
				func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					if args.Kind != esbuild.ResolveCSSURLToken {
						return esbuild.OnResolveResult{}, nil
					}

					raw := strings.TrimSpace(args.Path)
					parsed, parse_err := url.Parse(raw)
					if parse_err != nil || parsed.Scheme != "" ||
						strings.HasPrefix(raw, "//") {
						return esbuild.OnResolveResult{
							Path:     raw,
							External: true,
						}, nil
					}

					lookup := strings.TrimPrefix(parsed.Path, "/")
					if strings.HasPrefix(lookup, ".") {
						panic(
							"[wave]: CSS URL paths must not be relative (must not start with '.' or '..')",
						)
					}

					suffix := ""
					if parsed.RawQuery != "" || parsed.Fragment != "" {
						suffix = raw[len(lookup):]
					}

					if public_url, ok := url_lookup_map[lookup]; ok {
						return esbuild.OnResolveResult{
							Path: path.Join(
								public_path_prefix,
								public_url,
							) + suffix,
							External: true,
						}, nil
					}

					return esbuild.OnResolveResult{
						Path:     raw,
						External: true,
					}, nil
				},
			)
		},
	}
}
