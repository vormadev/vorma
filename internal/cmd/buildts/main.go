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
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/executil"
	"golang.org/x/sync/errgroup"
)

const buildCacheVersion = 2

var targetDir = "./npm_dist"
var buildCachePath = "./npm_dist/.buildts_cache.json"
var buildInputPaths = []string{
	"./typescript/kit",
	"./typescript/vorma/client",
	"./typescript/vorma/ui-adapters/react",
	"./typescript/vorma/ui-adapters/preact",
	"./typescript/vorma/ui-adapters/solid",
	"./typescript/vorma/vite",
	"./typescript/vorma/create",
	"./internal/cmd/buildts",
	"./package.json",
	"./pnpm-lock.yaml",
	"./tsconfig.base.json",
}
var buildOutputPaths = []string{
	targetDir,
	"./typescript/vorma/create/dist",
}
var tscRunMutex sync.Mutex

func main() {
	if err := run(); err != nil {
		log.Fatalf("failed to build TypeScript packages: %v", err)
	}
}

func run() error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target dir: %w", err)
	}

	cachedHashes, err := readBuildCache()
	if err != nil {
		return err
	}

	inputHash, err := hashBuildInputs()
	if err != nil {
		return err
	}

	currentOutputHash, err := hashBuildOutputs()
	if err != nil {
		return err
	}

	if cachedHashes != nil &&
		cachedHashes.Version == buildCacheVersion &&
		cachedHashes.InputHash == inputHash &&
		cachedHashes.OutputHash == currentOutputHash {
		log.Println("buildts: inputs and outputs unchanged; skipping build")
		return nil
	}

	if err := cleanBuildOutputs(); err != nil {
		return err
	}

	if err := runBuildStages(); err != nil {
		return err
	}

	if err := removeTestFiles(); err != nil {
		return err
	}

	if err := assertBuildOutputsAreConsistent(); err != nil {
		return err
	}

	updatedOutputHash, err := hashBuildOutputs()
	if err != nil {
		return err
	}

	if err := writeBuildCache(buildHashes{
		Version:    buildCacheVersion,
		InputHash:  inputHash,
		OutputHash: updatedOutputHash,
	}); err != nil {
		return err
	}

	return nil
}

func cleanBuildOutputs() error {
	for _, outputPath := range buildOutputPaths {
		if err := os.RemoveAll(outputPath); err != nil {
			return fmt.Errorf(
				"failed to clean output path %s: %w",
				outputPath,
				err,
			)
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate target dir: %w", err)
	}

	return nil
}

type buildTask struct {
	name string
	run  func() error
}

type buildHashes struct {
	Version    int    `json:"version"`
	InputHash  string `json:"input_hash"`
	OutputHash string `json:"output_hash"`
}

func runBuildStages() error {
	// Stage 1 contains independent packages.
	if err := runTasksInParallel(
		"stage-1",
		[]buildTask{
			{
				name: "kit",
				run:  buildKit,
			},
			{
				name: "client",
				run:  buildClient,
			},
			{
				name: "vite",
				run:  buildVite,
			},
			{
				name: "create",
				run:  buildCreate,
			},
		},
	); err != nil {
		return err
	}

	// Stage 2 adapters depend on the client API surface.
	if err := runTasksInParallel(
		"stage-2",
		[]buildTask{
			{
				name: "react",
				run:  buildReact,
			},
			{
				name: "solid",
				run:  buildSolid,
			},
			{
				name: "preact",
				run:  buildPreact,
			},
		},
	); err != nil {
		return err
	}

	return nil
}

func runTasksInParallel(stageName string, tasks []buildTask) error {
	if len(tasks) == 0 {
		return nil
	}

	parallelism := runtime.NumCPU()
	if parallelism < 1 {
		parallelism = 1
	}
	if parallelism > len(tasks) {
		parallelism = len(tasks)
	}
	if parallelism > 4 {
		parallelism = 4
	}

	log.Printf(
		"%s: starting %d tasks with parallelism=%d",
		stageName,
		len(tasks),
		parallelism,
	)

	var g errgroup.Group
	g.SetLimit(parallelism)

	for _, task := range tasks {
		task := task
		g.Go(func() error {
			log.Printf("%s: started", task.name)
			if err := task.run(); err != nil {
				return fmt.Errorf("%s failed: %w", task.name, err)
			}
			log.Printf("%s: completed", task.name)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("%s failed: %w", stageName, err)
	}

	log.Printf("%s: completed", stageName)
	return nil
}

func buildKit() error {
	tsconfig := "./typescript/kit/tsconfig.json"
	if err := runTSC(tsconfig); err != nil {
		return err
	}
	if err := build("kit", esbuild.BuildOptions{
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
			"./typescript/kit/theme/theme.ts",
			"./typescript/kit/url/url.ts",
		},
		External: []string{"vorma"},
		Outdir:   "./npm_dist/typescript/kit",
		Tsconfig: tsconfig,
	}); err != nil {
		return err
	}
	return nil
}

func buildClient() error {
	tsconfig := "./typescript/vorma/client/tsconfig.json"
	if err := runTSC(tsconfig); err != nil {
		return err
	}
	if err := build("client", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Splitting:   true,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{
			"./typescript/vorma/client/index.ts",
			"./typescript/vorma/client/internal.ts",
			"./typescript/vorma/client/buildtime.ts",
		},
		External: []string{
			"vorma",
		},
		Outdir:   "./npm_dist/typescript/vorma/client",
		Tsconfig: tsconfig,
	}); err != nil {
		return err
	}
	return nil
}

func buildReact() error {
	tsconfig := "./typescript/vorma/ui-adapters/react/tsconfig.json"
	if err := runTSC(tsconfig); err != nil {
		return err
	}
	if err := build("react", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Splitting:   true,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{"./typescript/vorma/ui-adapters/react/index.tsx"},
		External: []string{
			"vorma",
			"react", "react-dom",
		},
		Outdir:   "./npm_dist/typescript/vorma/ui-adapters/react",
		Tsconfig: tsconfig,
	}); err != nil {
		return err
	}
	return nil
}

func buildSolid() error {
	if err := runTSC("./typescript/vorma/ui-adapters/solid/tsconfig.json"); err != nil {
		return err
	}

	// we need babel transforms via esbuild-plugin-solid
	if err := executil.RunCmd("node", "./internal/cmd/buildts/build-solid.mjs"); err != nil {
		return fmt.Errorf("failed to run build-solid.mjs: %w", err)
	}

	log.Println("solid: esbuild (via node) succeeded")
	return nil
}

func buildPreact() error {
	tsconfig := "./typescript/vorma/ui-adapters/preact/tsconfig.json"
	if err := runTSC(tsconfig); err != nil {
		return err
	}
	if err := build("preact", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Splitting:   true,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{"./typescript/vorma/ui-adapters/preact/index.tsx"},
		External: []string{
			"vorma",
			"preact", "preact/hooks",
			"@preact/signals",
			"preact/jsx-runtime", "preact/compat", "preact/test-utils",
		},
		Outdir:   "./npm_dist/typescript/vorma/ui-adapters/preact",
		Tsconfig: tsconfig,
	}); err != nil {
		return err
	}
	return nil
}

func buildVite() error {
	tsconfig := "./typescript/vorma/vite/tsconfig.json"
	if err := runTSC(tsconfig); err != nil {
		return err
	}
	if err := build("vite", esbuild.BuildOptions{
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
		},
		Outdir:   "./npm_dist/typescript/vorma/vite",
		Tsconfig: tsconfig,
	}); err != nil {
		return err
	}
	return nil
}

func buildCreate() error {
	tsconfig := "./typescript/vorma/create/tsconfig.json"
	if err := runTSC(tsconfig); err != nil {
		return err
	}
	if err := build("create", esbuild.BuildOptions{
		Sourcemap:   esbuild.SourceMapLinked,
		Target:      esbuild.ESNext,
		Format:      esbuild.FormatESModule,
		TreeShaking: esbuild.TreeShakingTrue,
		Write:       true,
		Bundle:      true,
		EntryPoints: []string{"./typescript/vorma/create/main.ts"},
		External: []string{
			"node:child_process",
			"node:fs",
			"node:os",
			"node:path",
			"node:process",
			"node:readline",
			"node:stream",
			"node:util",
			"node:url",
		},
		Outdir:   "./typescript/vorma/create/dist",
		Tsconfig: tsconfig,
	}); err != nil {
		return err
	}
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// Build helpers
/////////////////////////////////////////////////////////////////////

func readBuildCache() (*buildHashes, error) {
	cacheContents, err := os.ReadFile(buildCachePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read build cache: %w", err)
	}

	var cache buildHashes
	if err := json.Unmarshal(cacheContents, &cache); err != nil {
		log.Printf("buildts: ignoring invalid cache file (%v)", err)
		return nil, nil
	}

	if cache.Version != buildCacheVersion {
		return nil, nil
	}

	return &cache, nil
}

func writeBuildCache(cache buildHashes) error {
	cacheContents, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal build cache: %w", err)
	}

	if err := os.WriteFile(buildCachePath, cacheContents, 0644); err != nil {
		return fmt.Errorf("failed to write build cache: %w", err)
	}

	return nil
}

func hashBuildInputs() (string, error) {
	return hashFileSet(buildInputPaths, shouldSkipInputPath)
}

func hashBuildOutputs() (string, error) {
	return hashFileSet(buildOutputPaths, shouldSkipOutputPath)
}

type fileHashEntry struct {
	hashPath string
	realPath string
}

func hashFileSet(
	roots []string,
	shouldSkipPath func(path string, isDir bool) bool,
) (string, error) {
	hasher := sha256.New()
	files := make([]fileHashEntry, 0, 256)
	presentRoots := make([]string, 0, len(roots))
	missingRoots := make([]string, 0, len(roots))

	for _, root := range roots {
		cleanRoot := filepath.Clean(root)
		if shouldSkipPath(cleanRoot, true) {
			continue
		}

		fileInfo, err := os.Stat(cleanRoot)
		if errors.Is(err, os.ErrNotExist) {
			missingRoots = append(missingRoots, filepath.ToSlash(cleanRoot))
			continue
		}
		if err != nil {
			return "", fmt.Errorf("failed to stat %s: %w", cleanRoot, err)
		}

		presentRoots = append(presentRoots, filepath.ToSlash(cleanRoot))
		if !fileInfo.IsDir() {
			files = append(files, fileHashEntry{
				hashPath: filepath.ToSlash(cleanRoot),
				realPath: cleanRoot,
			})
			continue
		}

		if err := filepath.WalkDir(cleanRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			cleanPath := filepath.Clean(path)
			if shouldSkipPath(cleanPath, d.IsDir()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				return nil
			}

			files = append(files, fileHashEntry{
				hashPath: filepath.ToSlash(cleanPath),
				realPath: cleanPath,
			})
			return nil
		}); err != nil {
			return "", fmt.Errorf("failed to walk %s: %w", cleanRoot, err)
		}
	}

	sort.Strings(presentRoots)
	sort.Strings(missingRoots)
	sort.Slice(files, func(i int, j int) bool {
		return files[i].hashPath < files[j].hashPath
	})

	for _, root := range presentRoots {
		if _, err := io.WriteString(hasher, "ROOT:"+root+"\n"); err != nil {
			return "", fmt.Errorf("failed to hash present roots: %w", err)
		}
	}
	for _, root := range missingRoots {
		if _, err := io.WriteString(hasher, "MISSING:"+root+"\n"); err != nil {
			return "", fmt.Errorf("failed to hash missing roots: %w", err)
		}
	}
	for _, file := range files {
		if _, err := io.WriteString(hasher, "FILE:"+file.hashPath+"\n"); err != nil {
			return "", fmt.Errorf(
				"failed to hash file path (%s): %w",
				file.hashPath,
				err,
			)
		}

		contents, err := os.ReadFile(file.realPath)
		if err != nil {
			return "", fmt.Errorf(
				"failed to read file for hash (%s): %w",
				file.realPath,
				err,
			)
		}
		if _, err := hasher.Write(contents); err != nil {
			return "", fmt.Errorf(
				"failed to hash file contents (%s): %w",
				file.hashPath,
				err,
			)
		}
		if _, err := hasher.Write([]byte{0}); err != nil {
			return "", fmt.Errorf(
				"failed to hash file separator (%s): %w",
				file.hashPath,
				err,
			)
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func shouldSkipInputPath(path string, isDir bool) bool {
	normalizedPath := filepath.ToSlash(filepath.Clean(path))

	if pathHasSegment(normalizedPath, ".git") ||
		pathHasSegment(normalizedPath, "node_modules") {
		return true
	}

	if normalizedPath == filepath.ToSlash(filepath.Clean(targetDir)) ||
		strings.HasPrefix(
			normalizedPath,
			filepath.ToSlash(filepath.Clean(targetDir))+"/",
		) {
		return true
	}

	if normalizedPath == "typescript/vorma/create/dist" ||
		strings.HasPrefix(normalizedPath, "typescript/vorma/create/dist/") {
		return true
	}

	// Ignore package output produced by `create`.
	if isDir && filepath.Base(normalizedPath) == "dist" &&
		strings.Contains(normalizedPath, "typescript/vorma/create") {
		return true
	}

	return false
}

func shouldSkipOutputPath(path string, _ bool) bool {
	normalizedPath := filepath.ToSlash(filepath.Clean(path))
	if pathHasSegment(normalizedPath, ".git") ||
		pathHasSegment(normalizedPath, "node_modules") {
		return true
	}
	if normalizedPath == filepath.ToSlash(filepath.Clean(buildCachePath)) {
		return true
	}

	return false
}

func pathHasSegment(path string, segment string) bool {
	if path == segment {
		return true
	}
	if strings.HasPrefix(path, segment+"/") {
		return true
	}
	if strings.Contains(path, "/"+segment+"/") {
		return true
	}
	if strings.HasSuffix(path, "/"+segment) {
		return true
	}
	return false
}

func runTSC(tsConfig string) error {
	tscRunMutex.Lock()
	defer tscRunMutex.Unlock()

	args := []string{
		"tsc",
		"--project", tsConfig,
		"--declaration",
		"--emitDeclarationOnly",
		"--outDir", "./npm_dist",
		"--noEmit", "false",
		"--rootDir", "./",
		"--sourceMap",
		"--declarationMap",
	}
	log.Printf("running command: pnpm %s", strings.Join(args, " "))
	cmd := exec.Command("pnpm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run tsc for %s: %w", tsConfig, err)
	}

	log.Printf("tsc succeeded (%s)", tsConfig)
	return nil
}

func build(label string, opts esbuild.BuildOptions) error {
	result := esbuild.Build(opts)

	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			log.Println(fmt.Sprintf("%s:", label), err.Text)
		}
		return fmt.Errorf("%s: esbuild failed", label)
	}

	if len(result.Warnings) > 0 {
		for _, warn := range result.Warnings {
			log.Println(fmt.Sprintf("%s:", label), warn.Text)
		}
		return fmt.Errorf("%s: esbuild had warnings", label)
	}

	log.Printf("%s: esbuild succeeded\n", label)
	return nil
}

func removeTestFiles() error {
	if err := os.RemoveAll(filepath.Join(targetDir, "typescript/vorma/client/src/tests")); err != nil {
		return fmt.Errorf("failed to remove client test declarations: %w", err)
	}

	err := filepath.Walk(
		targetDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Remove test files and their source maps
			if strings.Contains(path, ".test.") ||
				strings.Contains(path, ".bench.") {
				return os.Remove(path)
			}

			return nil
		},
	)

	if err != nil {
		return fmt.Errorf("failed to remove test files: %w", err)
	}

	log.Println("Test files removed successfully")
	return nil
}

func assertBuildOutputsAreConsistent() error {
	if err := assertClientTestDeclarationsAbsent(); err != nil {
		return err
	}
	if err := assertClientDeclarationsHaveSourceFiles(); err != nil {
		return err
	}
	if err := assertNoOrphanClientChunks(); err != nil {
		return err
	}
	return nil
}

func assertClientTestDeclarationsAbsent() error {
	clientTestsPath := filepath.Join(
		targetDir,
		"typescript/vorma/client/src/tests",
	)
	if _, err := os.Stat(clientTestsPath); err == nil {
		return fmt.Errorf(
			"client test declarations leaked into build output: %s",
			filepath.ToSlash(clientTestsPath),
		)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %w", filepath.ToSlash(clientTestsPath), err)
	}
	return nil
}

func assertClientDeclarationsHaveSourceFiles() error {
	clientSrcDistPath := filepath.Join(targetDir, "typescript/vorma/client/src")
	if _, err := os.Stat(clientSrcDistPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to stat client dist source path: %w", err)
	}

	var staleDeclarationPaths []string
	err := filepath.WalkDir(
		clientSrcDistPath,
		func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".d.ts") {
				return nil
			}

			relativeDistPath, err := filepath.Rel(targetDir, path)
			if err != nil {
				return fmt.Errorf(
					"failed to compute relative path for %s: %w",
					filepath.ToSlash(path),
					err,
				)
			}
			relativeDistPath = filepath.ToSlash(relativeDistPath)
			relativeSourcePathWithoutDeclarationSuffix := strings.TrimSuffix(
				relativeDistPath,
				".d.ts",
			)

			candidateSourcePaths := []string{
				"./" + relativeSourcePathWithoutDeclarationSuffix + ".ts",
				"./" + relativeSourcePathWithoutDeclarationSuffix + ".tsx",
				"./" + relativeSourcePathWithoutDeclarationSuffix + ".d.ts",
			}

			for _, sourcePath := range candidateSourcePaths {
				if _, err := os.Stat(sourcePath); err == nil {
					return nil
				} else if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("failed to stat source candidate %s: %w", filepath.ToSlash(sourcePath), err)
				}
			}

			staleDeclarationPaths = append(
				staleDeclarationPaths,
				relativeDistPath,
			)
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to validate client declarations: %w", err)
	}

	if len(staleDeclarationPaths) > 0 {
		sort.Strings(staleDeclarationPaths)
		return fmt.Errorf(
			"client build output contains declarations without source files: %s",
			strings.Join(staleDeclarationPaths, ", "),
		)
	}

	return nil
}

func assertNoOrphanClientChunks() error {
	clientDistPath := filepath.Join(targetDir, "typescript/vorma/client")
	if _, err := os.Stat(clientDistPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to stat client dist path: %w", err)
	}

	reachableChunks, err := collectReachableClientChunks(clientDistPath)
	if err != nil {
		return err
	}

	chunkPaths, err := filepath.Glob(
		filepath.Join(clientDistPath, "chunk-*.js"),
	)
	if err != nil {
		return fmt.Errorf("failed to glob client chunks: %w", err)
	}

	var orphanChunkNames []string
	for _, chunkPath := range chunkPaths {
		chunkName := filepath.Base(chunkPath)
		if _, ok := reachableChunks[chunkName]; !ok {
			orphanChunkNames = append(orphanChunkNames, chunkName)
		}
	}

	if len(orphanChunkNames) > 0 {
		sort.Strings(orphanChunkNames)
		return fmt.Errorf(
			"client build output contains orphan chunks: %s",
			strings.Join(orphanChunkNames, ", "),
		)
	}

	return nil
}

func collectReachableClientChunks(
	clientDistPath string,
) (map[string]struct{}, error) {
	entryPaths := []string{
		filepath.Join(clientDistPath, "index.js"),
		filepath.Join(clientDistPath, "internal.js"),
		filepath.Join(clientDistPath, "buildtime.js"),
	}

	chunkImportPattern := regexp.MustCompile(
		`(?:from\s+|import\()\s*["']\./(chunk-[^"']+\.js)["']`,
	)
	reachableChunks := make(map[string]struct{})
	visitedScriptPaths := make(map[string]struct{})

	scriptQueue := make([]string, 0, len(entryPaths))
	for _, entryPath := range entryPaths {
		if _, err := os.Stat(entryPath); err == nil {
			scriptQueue = append(scriptQueue, entryPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to stat entry script %s: %w", filepath.ToSlash(entryPath), err)
		}
	}

	for len(scriptQueue) > 0 {
		currentScriptPath := scriptQueue[0]
		scriptQueue = scriptQueue[1:]
		if _, seen := visitedScriptPaths[currentScriptPath]; seen {
			continue
		}
		visitedScriptPaths[currentScriptPath] = struct{}{}

		fileContents, err := os.ReadFile(currentScriptPath)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read script %s: %w",
				filepath.ToSlash(currentScriptPath),
				err,
			)
		}

		for _, match := range chunkImportPattern.FindAllStringSubmatch(string(fileContents), -1) {
			chunkName := match[1]
			if _, seen := reachableChunks[chunkName]; seen {
				continue
			}
			reachableChunks[chunkName] = struct{}{}
			scriptQueue = append(
				scriptQueue,
				filepath.Join(clientDistPath, chunkName),
			)
		}
	}

	return reachableChunks, nil
}
