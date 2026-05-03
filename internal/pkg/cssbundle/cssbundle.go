package cssbundle

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/set"
)

type BundleOutput struct {
	CSS     string
	Imports []string
}

type BundleArgs struct {
	EntryPath  string
	ResolveURL URLResolver
}

type URLResolver func(raw string, parsed *url.URL) (string, bool, error)

func Bundle(args BundleArgs) (BundleOutput, error) {
	src, err := os.ReadFile(args.EntryPath)
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
		Plugins:           []esbuild.Plugin{args.ResolveURL.plugin()},
		Stdin: &esbuild.StdinOptions{
			Contents:   string(src),
			ResolveDir: filepath.Dir(args.EntryPath),
			Sourcefile: filepath.Base(args.EntryPath),
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
	// the returned Imports slice. The entry file is added manually below.
	imports := set.Set[string]{}
	for raw := range meta.Inputs {
		if strings.HasPrefix(raw, "<") {
			continue
		}
		imports.Add(raw)
	}

	// esbuild seems to include this anyway, but add manually to be sure
	imports.Add(args.EntryPath)

	return BundleOutput{
		CSS:     strings.TrimSpace(string(result.OutputFiles[0].Contents)),
		Imports: imports.Slice(),
	}, nil
}

func (resolve_url URLResolver) plugin() esbuild.Plugin {
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

					if parsed.Path == "" && parsed.Fragment != "" {
						return esbuild.OnResolveResult{
							Path:     raw,
							External: true,
						}, nil
					}

					if resolve_url != nil {
						resolved_url, ok, err := resolve_url(raw, parsed)
						if err != nil {
							return esbuild.OnResolveResult{}, err
						}
						if ok {
							return esbuild.OnResolveResult{
								Path:     resolved_url,
								External: true,
							}, nil
						}
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
