package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/executil"
	"golang.org/x/sync/errgroup"
)

const cache_version = 2

var target_dir = "./npm_dist"
var cache_path = "./npm_dist/.buildts_cache.json"

var input_paths = []string{
	"./typescript/kit",
	"./vorma2/client",
	"./typescript/vorma/vite",
	"./typescript/vorma/create",
	"./internal/cmd/buildts",
	"./package.json",
	"./pnpm-lock.yaml",
	"./tsconfig.base.json",
}

var output_paths = []string{
	target_dir,
	"./typescript/vorma/create/dist",
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("%v", err)
	}
}

func run() error {
	if err := os.MkdirAll(target_dir, 0755); err != nil {
		return fmt.Errorf("failed to create target dir: %w", err)
	}

	cached, _ := read_cache()
	input_hash, err := hash_file_set(input_paths, skip_input)
	if err != nil {
		return err
	}
	output_hash, err := hash_file_set(output_paths, skip_output)
	if err != nil {
		return err
	}

	if cached != nil &&
		cached.Version == cache_version &&
		cached.InputHash == input_hash &&
		cached.OutputHash == output_hash {
		log.Println("buildts: unchanged; skipping")
		return nil
	}

	// Clean and rebuild.
	for _, p := range output_paths {
		os.RemoveAll(p)
	}
	if err := os.MkdirAll(target_dir, 0755); err != nil {
		return fmt.Errorf("failed to recreate target dir: %w", err)
	}

	if err := build_all(); err != nil {
		return err
	}

	remove_test_files()

	final_hash, err := hash_file_set(output_paths, skip_output)
	if err != nil {
		return err
	}
	return write_cache(build_cache{
		Version:    cache_version,
		InputHash:  input_hash,
		OutputHash: final_hash,
	})
}

/////////////////////////////////////////////////////////////////////
/////// Build stages
/////////////////////////////////////////////////////////////////////

func build_all() error {
	// Stage 1: independent packages.
	if err := run_parallel("stage-1", []build_task{
		{"kit", build_kit},
		{"client", build_client},
		{"vite", build_vite},
		{"create", build_create},
	}); err != nil {
		return err
	}

	// Stage 2: UI adapters depend on client.
	if err := run_parallel("stage-2", []build_task{
		{"react", build_react},
		{"solid", build_solid},
		{"preact", build_preact},
	}); err != nil {
		return err
	}

	return nil
}

func build_kit() error {
	tsconfig := "./typescript/kit/tsconfig.json"
	if err := run_tsgo(tsconfig); err != nil {
		return err
	}
	return run_esbuild("kit", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Splitting:   true,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{
			"./typescript/kit/converters/converters.ts",
			"./typescript/kit/cookies/cookies.ts",
			"./typescript/kit/csrf/csrf.ts",
			"./typescript/kit/debounce/debounce.ts",
			"./typescript/kit/fmt/fmt.ts",
			"./typescript/kit/json/json.ts",
			"./typescript/kit/listeners/listeners.ts",
			"./typescript/kit/matcher/register.ts",
			"./typescript/kit/matcher/find_best_match.ts",
			"./typescript/kit/matcher/find_nested_matches.ts",
			"./typescript/kit/matcher/utils.ts",
			"./typescript/kit/theme/theme.ts",
			"./typescript/kit/url/url.ts",
		},
		External: []string{"vorma"},
		Outdir:   "./npm_dist/typescript/kit",
		Tsconfig: tsconfig,
	})
}

func build_client() error {
	tsconfig := "./vorma2/client/tsconfig.json"
	if err := run_tsgo(tsconfig); err != nil {
		return err
	}
	return run_esbuild("client", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Splitting:   true,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{
			"./vorma2/client/_index.ts",
			"./vorma2/client/_internal.ts",
			"./vorma2/client/_testing.ts",
			"./vorma2/client/_buildtime.ts",
			"./vorma2/client/_hmr_dev.ts",
		},
		External: []string{"vorma"},
		Outdir:   "./npm_dist/vorma2/client",
		Tsconfig: tsconfig,
	})
}

func build_react() error {
	tsconfig := "./vorma2/client/ui-adapters/react/tsconfig.json"
	if err := run_tsgo(tsconfig); err != nil {
		return err
	}
	return run_esbuild("react", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Splitting:   true,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{"./vorma2/client/ui-adapters/react/_index.ts"},
		External:    []string{"vorma", "react", "react-dom"},
		Outdir:      "./npm_dist/vorma2/client/ui-adapters/react",
		Tsconfig:    tsconfig,
	})
}

func build_solid() error {
	if err := run_tsgo("./vorma2/client/ui-adapters/solid/tsconfig.json"); err != nil {
		return err
	}
	// Solid needs babel transforms via esbuild-plugin-solid.
	if err := executil.RunCmd("node", "./internal/cmd/buildts/build-solid.mjs"); err != nil {
		return fmt.Errorf("build-solid.mjs failed: %w", err)
	}
	log.Println("solid: esbuild (via node) succeeded")
	return nil
}

func build_preact() error {
	tsconfig := "./vorma2/client/ui-adapters/preact/tsconfig.json"
	if err := run_tsgo(tsconfig); err != nil {
		return err
	}
	return run_esbuild("preact", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Splitting:   true,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{"./vorma2/client/ui-adapters/preact/_index.ts"},
		External: []string{
			"vorma",
			"preact", "preact/hooks",
			"@preact/signals",
			"preact/jsx-runtime", "preact/compat", "preact/test-utils",
		},
		Outdir:   "./npm_dist/vorma2/client/ui-adapters/preact",
		Tsconfig: tsconfig,
	})
}

func build_vite() error {
	tsconfig := "./typescript/vorma/vite/tsconfig.json"
	if err := run_tsgo(tsconfig); err != nil {
		return err
	}
	return run_esbuild("vite", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Splitting:   true,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{"./typescript/vorma/vite/vite.ts"},
		External: []string{
			"vorma",
			"vite",
			"node:fs",
			"node:path",
			"node:url",
		},
		Outdir:   "./npm_dist/typescript/vorma/vite",
		Tsconfig: tsconfig,
	})
}

func build_create() error {
	tsconfig := "./typescript/vorma/create/tsconfig.json"
	if err := run_tsgo(tsconfig); err != nil {
		return err
	}
	return run_esbuild("create", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{"./typescript/vorma/create/main.ts"},
		External: []string{
			"node:child_process", "node:fs", "node:os", "node:path",
			"node:process", "node:readline", "node:stream", "node:util", "node:url",
		},
		Outdir:   "./typescript/vorma/create/dist",
		Tsconfig: tsconfig,
	})
}

/////////////////////////////////////////////////////////////////////
/////// Parallelism
/////////////////////////////////////////////////////////////////////

type build_task struct {
	name string
	fn   func() error
}

func run_parallel(stage string, tasks []build_task) error {
	n := runtime.NumCPU()
	if n > len(tasks) {
		n = len(tasks)
	}
	if n > 4 {
		n = 4
	}
	if n < 1 {
		n = 1
	}

	log.Printf("%s: %d tasks, parallelism=%d", stage, len(tasks), n)

	var g errgroup.Group
	g.SetLimit(n)
	for _, t := range tasks {
		t := t
		g.Go(func() error {
			log.Printf("%s: started", t.name)
			if err := t.fn(); err != nil {
				return fmt.Errorf("%s: %w", t.name, err)
			}
			log.Printf("%s: done", t.name)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("%s: %w", stage, err)
	}
	log.Printf("%s: done", stage)
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// tsgo + esbuild runners
/////////////////////////////////////////////////////////////////////

func run_tsgo(tsconfig string) error {
	args := []string{
		"tsgo",
		"--project", tsconfig,
		"--declaration",
		"--emitDeclarationOnly",
		"--outDir", target_dir,
		"--noEmit", "false",
		"--rootDir", "./",
		"--sourceMap",
		"--declarationMap",
	}
	log.Printf("running: pnpm %s", strings.Join(args, " "))
	cmd := exec.Command("pnpm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tsgo failed for %s: %w", tsconfig, err)
	}
	log.Printf("tsgo succeeded (%s)", tsconfig)
	return nil
}

func run_esbuild(label string, opts esbuild.BuildOptions) error {
	result := esbuild.Build(opts)
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			log.Printf("%s: %s", label, e.Text)
		}
		return fmt.Errorf("%s: esbuild failed", label)
	}
	if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			log.Printf("%s: %s", label, w.Text)
		}
		return fmt.Errorf("%s: esbuild had warnings", label)
	}
	log.Printf("%s: esbuild succeeded", label)
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// Test file cleanup
/////////////////////////////////////////////////////////////////////

func remove_test_files() {
	os.RemoveAll(filepath.Join(target_dir, "vorma2/client/tests"))

	filepath.Walk(
		target_dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.Contains(path, ".test.") ||
				strings.Contains(path, ".bench.") {
				os.Remove(path)
			}
			return nil
		},
	)

	log.Println("test files removed")
}

/////////////////////////////////////////////////////////////////////
/////// Build cache
/////////////////////////////////////////////////////////////////////

type build_cache struct {
	Version    int    `json:"version"`
	InputHash  string `json:"input_hash"`
	OutputHash string `json:"output_hash"`
}

func read_cache() (*build_cache, error) {
	data, err := os.ReadFile(cache_path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read build cache: %w", err)
	}
	var c build_cache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, nil
	}
	if c.Version != cache_version {
		return nil, nil
	}
	return &c, nil
}

func write_cache(c build_cache) error {
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal build cache: %w", err)
	}
	return os.WriteFile(cache_path, data, 0644)
}

/////////////////////////////////////////////////////////////////////
/////// File hashing
/////////////////////////////////////////////////////////////////////

type file_entry struct {
	hash_path string
	real_path string
}

func hash_file_set(
	roots []string,
	should_skip func(string, bool) bool,
) (string, error) {
	h := sha256.New()
	var files []file_entry
	var present, missing []string

	for _, root := range roots {
		clean := filepath.Clean(root)
		if should_skip(clean, true) {
			continue
		}
		info, err := os.Stat(clean)
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, filepath.ToSlash(clean))
			continue
		}
		if err != nil {
			return "", fmt.Errorf("stat %s: %w", clean, err)
		}
		present = append(present, filepath.ToSlash(clean))
		if !info.IsDir() {
			files = append(files, file_entry{
				hash_path: filepath.ToSlash(clean),
				real_path: clean,
			})
			continue
		}
		filepath.WalkDir(
			clean,
			func(path string, d os.DirEntry, walk_err error) error {
				if walk_err != nil {
					return walk_err
				}
				cp := filepath.Clean(path)
				if should_skip(cp, d.IsDir()) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if !d.IsDir() {
					files = append(files, file_entry{
						hash_path: filepath.ToSlash(cp),
						real_path: cp,
					})
				}
				return nil
			},
		)
	}

	sort.Strings(present)
	sort.Strings(missing)
	sort.Slice(
		files,
		func(i, j int) bool { return files[i].hash_path < files[j].hash_path },
	)

	for _, r := range present {
		io.WriteString(h, "ROOT:"+r+"\n")
	}
	for _, r := range missing {
		io.WriteString(h, "MISSING:"+r+"\n")
	}
	for _, f := range files {
		io.WriteString(h, "FILE:"+f.hash_path+"\n")
		contents, err := os.ReadFile(f.real_path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", f.real_path, err)
		}
		h.Write(contents)
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func skip_input(path string, is_dir bool) bool {
	p := filepath.ToSlash(filepath.Clean(path))
	if has_segment(p, ".git") || has_segment(p, "node_modules") {
		return true
	}
	td := filepath.ToSlash(filepath.Clean(target_dir))
	if p == td || strings.HasPrefix(p, td+"/") {
		return true
	}
	if p == "typescript/vorma/create/dist" ||
		strings.HasPrefix(p, "typescript/vorma/create/dist/") {
		return true
	}
	if is_dir && filepath.Base(p) == "dist" &&
		strings.Contains(p, "typescript/vorma/create") {
		return true
	}
	return false
}

func skip_output(path string, _ bool) bool {
	p := filepath.ToSlash(filepath.Clean(path))
	if has_segment(p, ".git") || has_segment(p, "node_modules") {
		return true
	}
	if p == filepath.ToSlash(filepath.Clean(cache_path)) {
		return true
	}
	return false
}

func has_segment(path, seg string) bool {
	return path == seg ||
		strings.HasPrefix(path, seg+"/") ||
		strings.Contains(path, "/"+seg+"/") ||
		strings.HasSuffix(path, "/"+seg)
}
