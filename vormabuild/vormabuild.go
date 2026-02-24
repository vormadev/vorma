// Package vormabuild contains Vorma build and dev entrypoints.
//
// This package is build-time only and is intended for build commands (for
// example, ./cmd/build). Do not import it from runtime request-serving code.
// Keeping build-time orchestration out of production binaries avoids pulling in
// unnecessary tooling dependencies and keeps app binaries smaller.
//
// Build and fast-rebuild flows follow a deterministic plan/stage/commit model:
// 1. plan: capture immutable runtime snapshots and compute build decisions.
// 2. stage: run filesystem/codegen side effects outside runtime write locks.
// 3. commit: apply bounded runtime-state commits under lock.
//
// Runtime invariants:
// - runtime state writes commit through the shared runtime-state commit path so
// readers never observe mixed-field updates.
// - rollback of captured runtime snapshots is attempt-scoped and guarded by
// build-ID token checks so stale attempts cannot overwrite newer commits.
// - artifact writers treat superseded runtime snapshots as no-op completion
// rather than mutating newer runtime state.
package vormabuild

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormabuild/tsgenruntime"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver"
)

type atomicFileWriteDependencies struct {
	createTempFileInDirectory func(string, string) (*os.File, error)
	writeAllBytesToTempFile   func(*os.File, []byte) (int, error)
	setTempFileMode           func(*os.File, fs.FileMode) error
	syncTempFileToDisk        func(*os.File) error
	closeTempFile             func(*os.File) error
	renameTempFile            func(string, string) error
	removeExistingTargetFile  func(string) error
	removeTempFile            func(string) error
	openParentDirectory       func(string) (*os.File, error)
	syncParentDirectory       func(*os.File) error
	closeParentDirectory      func(*os.File) error
}

type atomicFileWriteExecutor struct {
	dependencies atomicFileWriteDependencies
}

const atomicFileWriteTempFilePattern = ".vorma-atomic-write-*"

func defaultAtomicFileWriteDependencies() atomicFileWriteDependencies {
	return atomicFileWriteDependencies{
		createTempFileInDirectory: os.CreateTemp,
		writeAllBytesToTempFile: func(file *os.File, fileContents []byte) (int, error) {
			return file.Write(fileContents)
		},
		setTempFileMode: func(file *os.File, fileMode fs.FileMode) error {
			return file.Chmod(fileMode)
		},
		syncTempFileToDisk: func(file *os.File) error {
			return file.Sync()
		},
		closeTempFile:            func(file *os.File) error { return file.Close() },
		renameTempFile:           os.Rename,
		removeExistingTargetFile: os.Remove,
		removeTempFile:           os.Remove,
		openParentDirectory:      os.Open,
		syncParentDirectory: func(dir *os.File) error {
			return dir.Sync()
		},
		closeParentDirectory: func(dir *os.File) error {
			return dir.Close()
		},
	}
}

func normalizeAtomicFileWriteDependencies(
	dependencies atomicFileWriteDependencies,
) atomicFileWriteDependencies {
	defaultDependencies := defaultAtomicFileWriteDependencies()
	if dependencies.createTempFileInDirectory == nil {
		dependencies.createTempFileInDirectory = defaultDependencies.createTempFileInDirectory
	}
	if dependencies.writeAllBytesToTempFile == nil {
		dependencies.writeAllBytesToTempFile = defaultDependencies.writeAllBytesToTempFile
	}
	if dependencies.setTempFileMode == nil {
		dependencies.setTempFileMode = defaultDependencies.setTempFileMode
	}
	if dependencies.syncTempFileToDisk == nil {
		dependencies.syncTempFileToDisk = defaultDependencies.syncTempFileToDisk
	}
	if dependencies.closeTempFile == nil {
		dependencies.closeTempFile = defaultDependencies.closeTempFile
	}
	if dependencies.renameTempFile == nil {
		dependencies.renameTempFile = defaultDependencies.renameTempFile
	}
	if dependencies.removeExistingTargetFile == nil {
		dependencies.removeExistingTargetFile = defaultDependencies.removeExistingTargetFile
	}
	if dependencies.removeTempFile == nil {
		dependencies.removeTempFile = defaultDependencies.removeTempFile
	}
	if dependencies.openParentDirectory == nil {
		dependencies.openParentDirectory = defaultDependencies.openParentDirectory
	}
	if dependencies.syncParentDirectory == nil {
		dependencies.syncParentDirectory = defaultDependencies.syncParentDirectory
	}
	if dependencies.closeParentDirectory == nil {
		dependencies.closeParentDirectory = defaultDependencies.closeParentDirectory
	}
	return dependencies
}

func newAtomicFileWriteExecutor(
	dependencies atomicFileWriteDependencies,
) atomicFileWriteExecutor {
	return atomicFileWriteExecutor{
		dependencies: normalizeAtomicFileWriteDependencies(dependencies),
	}
}

var defaultAtomicFileWriteExecutor = newAtomicFileWriteExecutor(
	atomicFileWriteDependencies{},
)

func writeFileAtomically(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
) error {
	return defaultAtomicFileWriteExecutor.writeFileAtomically(
		targetPath,
		fileContents,
		fileMode,
	)
}

func writeFileAtomicallyWithDependencies(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
	dependencies atomicFileWriteDependencies,
) error {
	return newAtomicFileWriteExecutor(
		dependencies,
	).writeFileAtomically(targetPath, fileContents, fileMode)
}

func (executor atomicFileWriteExecutor) writeFileAtomically(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
) error {
	tempFile, err := executor.dependencies.createTempFileInDirectory(
		filepath.Dir(targetPath),
		atomicFileWriteTempFilePattern,
	)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tempPath := tempFile.Name()
	shouldRemoveTempPath := true
	defer func() {
		if shouldRemoveTempPath {
			_ = executor.dependencies.removeTempFile(tempPath)
		}
	}()

	bytesWritten, err := executor.dependencies.writeAllBytesToTempFile(
		tempFile,
		fileContents,
	)
	if err != nil {
		return executor.closeTempFileAfterAtomicWriteFailure(
			tempFile,
			err,
			"write temp file",
		)
	}
	if bytesWritten != len(fileContents) {
		return executor.closeTempFileAfterAtomicWriteFailure(
			tempFile,
			io.ErrShortWrite,
			"write temp file",
		)
	}

	if err := executor.dependencies.setTempFileMode(tempFile, fileMode); err != nil {
		return executor.closeTempFileAfterAtomicWriteFailure(
			tempFile,
			err,
			"set temp file mode",
		)
	}

	if err := executor.dependencies.syncTempFileToDisk(tempFile); err != nil {
		return executor.closeTempFileAfterAtomicWriteFailure(
			tempFile,
			err,
			"sync temp file",
		)
	}

	if err := executor.dependencies.closeTempFile(tempFile); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := executor.renameAtomicWriteTempPath(tempPath, targetPath); err != nil {
		return err
	}
	if err := executor.syncParentDirectoryAfterAtomicRename(targetPath); err != nil {
		return err
	}

	shouldRemoveTempPath = false
	return nil
}

func (executor atomicFileWriteExecutor) renameAtomicWriteTempPath(
	tempPath string,
	targetPath string,
) error {
	renameErr := executor.dependencies.renameTempFile(tempPath, targetPath)
	if renameErr == nil {
		return nil
	}

	if !shouldRetryRenameByReplacingTarget(renameErr, targetPath) {
		return fmt.Errorf("rename temp file: %w", renameErr)
	}

	removeExistingFileErr := executor.dependencies.removeExistingTargetFile(
		targetPath,
	)
	if removeExistingFileErr != nil && !os.IsNotExist(removeExistingFileErr) {
		return fmt.Errorf(
			"remove existing target file before rename: %w",
			removeExistingFileErr,
		)
	}

	renameAfterRemoveErr := executor.dependencies.renameTempFile(
		tempPath,
		targetPath,
	)
	if renameAfterRemoveErr != nil {
		return fmt.Errorf(
			"rename temp file after replacing existing target: %w",
			renameAfterRemoveErr,
		)
	}

	return nil
}

func shouldRetryRenameByReplacingTarget(
	renameErr error,
	targetPath string,
) bool {
	if errors.Is(renameErr, fs.ErrExist) || errors.Is(renameErr, os.ErrExist) {
		return true
	}

	if errors.Is(renameErr, fs.ErrPermission) ||
		errors.Is(renameErr, os.ErrPermission) {
		_, statErr := os.Stat(targetPath)
		return statErr == nil
	}

	return false
}

func (executor atomicFileWriteExecutor) closeTempFileAfterAtomicWriteFailure(
	tempFile *os.File,
	operationError error,
	operationContext string,
) error {
	closeErr := executor.dependencies.closeTempFile(tempFile)
	if closeErr != nil {
		return fmt.Errorf(
			"%s: %w",
			operationContext,
			errors.Join(
				operationError,
				fmt.Errorf("close temp file: %w", closeErr),
			),
		)
	}

	return fmt.Errorf("%s: %w", operationContext, operationError)
}

func (executor atomicFileWriteExecutor) syncParentDirectoryAfterAtomicRename(
	targetPath string,
) error {
	parentDirectoryPath := filepath.Dir(targetPath)
	parentDirectory, err := executor.dependencies.openParentDirectory(
		parentDirectoryPath,
	)
	if err != nil {
		return fmt.Errorf("open parent directory for sync: %w", err)
	}

	syncErr := executor.dependencies.syncParentDirectory(parentDirectory)
	closeErr := executor.dependencies.closeParentDirectory(parentDirectory)
	if syncErr == nil && closeErr == nil {
		return nil
	}

	if syncErr != nil && closeErr != nil {
		return fmt.Errorf(
			"sync parent directory: %w",
			errors.Join(
				syncErr,
				fmt.Errorf("close parent directory: %w", closeErr),
			),
		)
	}
	if syncErr != nil {
		return fmt.Errorf("sync parent directory: %w", syncErr)
	}
	return fmt.Errorf("close parent directory: %w", closeErr)
}

type backendRouteDiscoveryDependencies struct {
	expandPattern      func(string) ([]string, error)
	statPath           func(string) (fs.FileInfo, error)
	readFile           func(string) ([]byte, error)
	parseGoSourceAST   func(*token.FileSet, string, any, parser.Mode) (*ast.File, error)
	importGoPackageDir func(string) (*build.Package, error)
}

type backendRouteDiscoveryExecutor struct {
	dependencies backendRouteDiscoveryDependencies
}

var defaultBackendRouteDiscoveryExecutor = newBackendRouteDiscoveryExecutor(
	backendRouteDiscoveryDependencies{},
)

func defaultBackendRouteDiscoveryDependencies() backendRouteDiscoveryDependencies {
	return backendRouteDiscoveryDependencies{
		expandPattern: func(pattern string) ([]string, error) {
			return doublestar.FilepathGlob(pattern)
		},
		statPath:         os.Stat,
		readFile:         os.ReadFile,
		parseGoSourceAST: parser.ParseFile,
		importGoPackageDir: func(packageDir string) (*build.Package, error) {
			return build.Default.ImportDir(packageDir, 0)
		},
	}
}

func normalizeBackendRouteDiscoveryDependencies(
	dependencies backendRouteDiscoveryDependencies,
) backendRouteDiscoveryDependencies {
	defaultDependencies := defaultBackendRouteDiscoveryDependencies()

	if dependencies.expandPattern == nil {
		dependencies.expandPattern = defaultDependencies.expandPattern
	}
	if dependencies.statPath == nil {
		dependencies.statPath = defaultDependencies.statPath
	}
	if dependencies.readFile == nil {
		dependencies.readFile = defaultDependencies.readFile
	}
	if dependencies.parseGoSourceAST == nil {
		dependencies.parseGoSourceAST = defaultDependencies.parseGoSourceAST
	}
	if dependencies.importGoPackageDir == nil {
		dependencies.importGoPackageDir = defaultDependencies.importGoPackageDir
	}

	return dependencies
}

func newBackendRouteDiscoveryExecutor(
	dependencies backendRouteDiscoveryDependencies,
) backendRouteDiscoveryExecutor {
	return backendRouteDiscoveryExecutor{
		dependencies: normalizeBackendRouteDiscoveryDependencies(dependencies),
	}
}

type parsedServerRouteFile struct {
	path           string
	parsedAST      *ast.File
	importAliases  map[string]string
	dotImportPaths map[string]struct{}
}

type localFunctionDeclaration struct {
	name string
	decl *ast.FuncDecl
	file *parsedServerRouteFile
}

type backendRoutePackageAnalysis struct {
	goFileSet                           *token.FileSet
	files                               []*parsedServerRouteFile
	rootFilePathSet                     map[string]struct{}
	stringConstResolver                 *goStringConstResolver
	localFunctionsByName                map[string][]*localFunctionDeclaration
	localFunctionsByDeclarationPosition map[token.Pos]*localFunctionDeclaration
	packageLevelIdentifierNames         map[string]struct{}
	packageDir                          string
	packageName                         string
	filesByPath                         map[string]*parsedServerRouteFile
	goTypesInfo                         *types.Info
	goTypesPackagePath                  string
	goTypesInfoInitializationAttempted  bool
	goTypesInitializationCount          int
	functionScopeRanges                 []tokenPosRange
	dependencies                        backendRouteDiscoveryDependencies
}

type tokenPosRange struct {
	start token.Pos
	end   token.Pos
}

type backendRouteDiscoveryState struct {
	loaderPatternSet                    map[string]struct{}
	activeFunctions                     map[*ast.FuncDecl]struct{}
	discoveredVormaRegistrationCallByID map[string]*discoveredVormaRegistrationCall
}

type expressionBindings struct {
	values map[string]ast.Expr
	parent *expressionBindings
}

type canonicalRouteRegistrationCall struct {
	isLoader                        bool
	pattern                         string
	discoveredVormaRegistrationCall *discoveredVormaRegistrationCall
}

type discoveredVormaRegistrationCall struct {
	isLoader              bool
	appExpression         ast.Expr
	methodExpression      ast.Expr
	patternExpression     ast.Expr
	handlerExpression     ast.Expr
	decorateCtxExpression ast.Expr
	sourcePosition        token.Pos
}

func parseBackendLoaderPatterns(v *vormaruntime.Vorma) ([]string, error) {
	return defaultBackendRouteDiscoveryExecutor.parseBackendLoaderPatterns(v)
}

func (executor backendRouteDiscoveryExecutor) parseBackendLoaderPatterns(
	v *vormaruntime.Vorma,
) ([]string, error) {
	serverRouteDefinitionFiles, err := executor.resolveServerRouteDefinitionFiles(
		v,
	)
	if err != nil {
		return nil, err
	}
	if len(serverRouteDefinitionFiles) == 0 {
		return nil, nil
	}

	packageAnalyses, err := executor.parseServerRouteFilesIntoPackageAnalyses(
		serverRouteDefinitionFiles,
	)
	if err != nil {
		return nil, err
	}

	loaderPatternSet := map[string]struct{}{}
	for _, packageAnalysis := range packageAnalyses {
		packageLoaderPatterns, err := packageAnalysis.discoverLoaderPatterns()
		if err != nil {
			return nil, err
		}
		for _, loaderPattern := range packageLoaderPatterns {
			loaderPatternSet[loaderPattern] = struct{}{}
		}
	}

	loaderPatterns := make([]string, 0, len(loaderPatternSet))
	for loaderPattern := range loaderPatternSet {
		loaderPatterns = append(loaderPatterns, loaderPattern)
	}
	sort.Strings(loaderPatterns)
	return loaderPatterns, nil
}

func parseServerRouteFilesIntoPackageAnalyses(
	serverRouteDefinitionFiles []string,
) ([]*backendRoutePackageAnalysis, error) {
	return defaultBackendRouteDiscoveryExecutor.parseServerRouteFilesIntoPackageAnalyses(
		serverRouteDefinitionFiles,
	)
}

func (executor backendRouteDiscoveryExecutor) parseServerRouteFilesIntoPackageAnalyses(
	serverRouteDefinitionFiles []string,
) ([]*backendRoutePackageAnalysis, error) {
	goFileSet := token.NewFileSet()
	analysesByPackageKey := map[string]*backendRoutePackageAnalysis{}

	for _, serverRouteDefinitionFile := range serverRouteDefinitionFiles {
		absoluteRouteDefinitionFile, err := filepath.Abs(
			serverRouteDefinitionFile,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"resolve absolute server route definition file %q: %w",
				serverRouteDefinitionFile,
				err,
			)
		}

		parsedAST, err := parseServerRouteDefinitionFileWithDependencies(
			goFileSet,
			absoluteRouteDefinitionFile,
			executor.dependencies,
		)
		if err != nil {
			return nil, err
		}

		parsedServerFile := parseServerRouteFileMetadata(
			absoluteRouteDefinitionFile,
			parsedAST,
		)
		packageKey := deriveServerRoutePackageAnalysisKey(parsedServerFile)
		packageAnalysis, hasPackageAnalysis := analysesByPackageKey[packageKey]
		if !hasPackageAnalysis {
			packageAnalysis = &backendRoutePackageAnalysis{
				goFileSet:                           goFileSet,
				rootFilePathSet:                     map[string]struct{}{},
				localFunctionsByName:                map[string][]*localFunctionDeclaration{},
				localFunctionsByDeclarationPosition: map[token.Pos]*localFunctionDeclaration{},
				packageLevelIdentifierNames:         map[string]struct{}{},
				packageDir: filepath.ToSlash(
					filepath.Dir(parsedServerFile.path),
				),
				packageName:  parsedServerFile.parsedAST.Name.Name,
				filesByPath:  map[string]*parsedServerRouteFile{},
				dependencies: executor.dependencies,
			}
			analysesByPackageKey[packageKey] = packageAnalysis
		}
		packageAnalysis.files = append(packageAnalysis.files, parsedServerFile)
		packageAnalysis.filesByPath[parsedServerFile.path] = parsedServerFile
		packageAnalysis.rootFilePathSet[parsedServerFile.path] = struct{}{}
	}

	packageKeys := make([]string, 0, len(analysesByPackageKey))
	for packageKey := range analysesByPackageKey {
		packageKeys = append(packageKeys, packageKey)
	}
	sort.Strings(packageKeys)

	packageAnalyses := make([]*backendRoutePackageAnalysis, 0, len(packageKeys))
	for _, packageKey := range packageKeys {
		packageAnalysis := analysesByPackageKey[packageKey]
		if err := packageAnalysis.initialize(); err != nil {
			return nil, err
		}
		packageAnalyses = append(packageAnalyses, packageAnalysis)
	}
	return packageAnalyses, nil
}

func parseServerRouteFileMetadata(
	serverRouteDefinitionFile string,
	parsedAST *ast.File,
) *parsedServerRouteFile {
	parsedServerFile := &parsedServerRouteFile{
		path: filepath.ToSlash(
			filepath.Clean(serverRouteDefinitionFile),
		),
		parsedAST:      parsedAST,
		importAliases:  map[string]string{},
		dotImportPaths: map[string]struct{}{},
	}

	for _, importSpec := range parsedAST.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			continue
		}
		if importSpec.Name == nil {
			defaultAlias := path.Base(importPath)
			if defaultAlias != "" {
				parsedServerFile.importAliases[defaultAlias] = importPath
			}
			continue
		}

		importAlias := importSpec.Name.Name
		switch importAlias {
		case "_":
			continue
		case ".":
			parsedServerFile.dotImportPaths[importPath] = struct{}{}
		default:
			parsedServerFile.importAliases[importAlias] = importPath
		}
	}

	return parsedServerFile
}

func deriveServerRoutePackageAnalysisKey(
	parsedServerFile *parsedServerRouteFile,
) string {
	return filepath.ToSlash(
		filepath.Dir(parsedServerFile.path),
	) + "|" + parsedServerFile.parsedAST.Name.Name
}

func (analysis *backendRoutePackageAnalysis) initialize() error {
	containsRouteRegistrationHints, err := analysis.packageContainsRouteRegistrationHints()
	if err != nil {
		return err
	}
	if !containsRouteRegistrationHints {
		return nil
	}

	if err := analysis.expandAndFilterFilesForCompiledPackage(); err != nil {
		return err
	}

	sort.Slice(analysis.files, func(i int, j int) bool {
		return analysis.files[i].path < analysis.files[j].path
	})

	parsedGoFiles := make([]*ast.File, 0, len(analysis.files))
	for _, parsedServerFile := range analysis.files {
		parsedGoFiles = append(parsedGoFiles, parsedServerFile.parsedAST)
	}
	analysis.stringConstResolver = newGoStringConstResolver(
		collectPackageStringConstExpressions(parsedGoFiles),
	)

	for _, parsedServerFile := range analysis.files {
		for _, declaration := range parsedServerFile.parsedAST.Decls {
			analysis.collectPackageLevelIdentifierNamesFromDeclaration(
				declaration,
			)

			functionDeclaration, isFunctionDeclaration := declaration.(*ast.FuncDecl)
			if !isFunctionDeclaration || functionDeclaration.Recv != nil {
				continue
			}
			localFunction := &localFunctionDeclaration{
				name: functionDeclaration.Name.Name,
				decl: functionDeclaration,
				file: parsedServerFile,
			}
			analysis.localFunctionsByName[localFunction.name] = append(
				analysis.localFunctionsByName[localFunction.name],
				localFunction,
			)
			functionDeclarationPosition := token.NoPos
			if functionDeclaration.Name != nil {
				functionDeclarationPosition = functionDeclaration.Name.Pos()
			}
			if functionDeclarationPosition != token.NoPos {
				analysis.localFunctionsByDeclarationPosition[functionDeclarationPosition] = localFunction
			}
		}
	}

	analysis.functionScopeRanges = collectFunctionScopeRangesFromParsedFiles(
		analysis.files,
	)

	return nil
}

var routeRegistrationHintNeedles = [][]byte{
	[]byte("DefineLoaderForRegistration"),
	[]byte("DefineActionForRegistration"),
	[]byte("AddTaskHandler"),
}

func (analysis *backendRoutePackageAnalysis) packageContainsRouteRegistrationHints() (bool, error) {
	packageDirEntries, readDirErr := os.ReadDir(analysis.packageDir)
	if readDirErr != nil {
		return false, fmt.Errorf(
			"read package directory %q for route registration hint scan: %w",
			analysis.packageDir,
			readDirErr,
		)
	}

	for _, packageDirEntry := range packageDirEntries {
		if packageDirEntry.IsDir() {
			continue
		}
		entryName := packageDirEntry.Name()
		if filepath.Ext(entryName) != ".go" ||
			strings.HasSuffix(entryName, "_test.go") {
			continue
		}
		entryFilePath := filepath.ToSlash(
			filepath.Clean(filepath.Join(analysis.packageDir, entryName)),
		)
		entryFileBytes, readErr := analysis.dependencies.readFile(entryFilePath)
		if readErr != nil {
			return false, fmt.Errorf(
				"read file %q for route registration hint scan: %w",
				entryFilePath,
				readErr,
			)
		}
		for _, routeRegistrationHintNeedle := range routeRegistrationHintNeedles {
			if bytes.Contains(entryFileBytes, routeRegistrationHintNeedle) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (analysis *backendRoutePackageAnalysis) expandAndFilterFilesForCompiledPackage() error {
	compiledFilePathSet, err := analysis.resolveCompiledFilePathSetForPackageDir()
	if err != nil {
		return err
	}

	filteredFiles := make([]*parsedServerRouteFile, 0, len(analysis.files))
	filteredFilesByPath := map[string]*parsedServerRouteFile{}
	filteredRootFilePathSet := map[string]struct{}{}

	for _, parsedServerFile := range analysis.files {
		if len(compiledFilePathSet) > 0 {
			if _, isCompiledFile := compiledFilePathSet[parsedServerFile.path]; !isCompiledFile {
				continue
			}
		}
		filteredFiles = append(filteredFiles, parsedServerFile)
		filteredFilesByPath[parsedServerFile.path] = parsedServerFile
		if _, isRootFile := analysis.rootFilePathSet[parsedServerFile.path]; isRootFile {
			filteredRootFilePathSet[parsedServerFile.path] = struct{}{}
		}
	}

	analysis.files = filteredFiles
	analysis.filesByPath = filteredFilesByPath
	analysis.rootFilePathSet = filteredRootFilePathSet

	if len(compiledFilePathSet) == 0 {
		return nil
	}

	for compiledFilePath := range compiledFilePathSet {
		if _, hasParsedFile := analysis.filesByPath[compiledFilePath]; hasParsedFile {
			continue
		}
		parsedAST, err := parseServerRouteDefinitionFileWithDependencies(
			analysis.goFileSet,
			compiledFilePath,
			analysis.dependencies,
		)
		if err != nil {
			return err
		}
		parsedServerFile := parseServerRouteFileMetadata(
			compiledFilePath,
			parsedAST,
		)
		analysis.files = append(analysis.files, parsedServerFile)
		analysis.filesByPath[parsedServerFile.path] = parsedServerFile
	}

	return nil
}

func (analysis *backendRoutePackageAnalysis) resolveCompiledFilePathSetForPackageDir() (map[string]struct{}, error) {
	importedPackage, err := analysis.dependencies.importGoPackageDir(
		analysis.packageDir,
	)
	if err != nil {
		if _, isNoGoError := err.(*build.NoGoError); isNoGoError {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf(
			"resolve compiled Go files for package directory %q: %w",
			analysis.packageDir,
			err,
		)
	}

	compiledFilePathSet := map[string]struct{}{}
	compiledGoFiles := append([]string{}, importedPackage.GoFiles...)
	compiledGoFiles = append(compiledGoFiles, importedPackage.CgoFiles...)
	for _, compiledGoFile := range compiledGoFiles {
		compiledFilePath := filepath.ToSlash(
			filepath.Clean(filepath.Join(importedPackage.Dir, compiledGoFile)),
		)
		compiledFilePathSet[compiledFilePath] = struct{}{}
	}
	return compiledFilePathSet, nil
}

func (analysis *backendRoutePackageAnalysis) initializeGoTypesInfo() error {
	analysis.goTypesInitializationCount++

	if len(analysis.files) == 0 {
		return nil
	}

	parsedGoFiles := make([]*ast.File, 0, len(analysis.files))
	for _, parsedServerFile := range analysis.files {
		parsedGoFiles = append(parsedGoFiles, parsedServerFile.parsedAST)
	}

	goTypesInfo := &types.Info{
		Uses:       map[*ast.Ident]types.Object{},
		Defs:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	goTypesConfig := &types.Config{
		// Use export-data importer for speed. We only need best-effort symbol
		// identity and fall back when type info is incomplete.
		Importer: importer.Default(),
	}
	checkedPackage, _ := goTypesConfig.Check(
		analysis.packageDir,
		analysis.goFileSet,
		parsedGoFiles,
		goTypesInfo,
	)
	analysis.goTypesInfo = goTypesInfo
	if checkedPackage != nil {
		analysis.goTypesPackagePath = checkedPackage.Path()
	}
	return nil
}

func (analysis *backendRoutePackageAnalysis) ensureGoTypesInfoInitialized() error {
	if analysis.goTypesInfo != nil ||
		analysis.goTypesInfoInitializationAttempted {
		return nil
	}
	analysis.goTypesInfoInitializationAttempted = true
	return analysis.initializeGoTypesInfo()
}

func (analysis *backendRoutePackageAnalysis) collectPackageLevelIdentifierNamesFromDeclaration(
	declaration ast.Decl,
) {
	switch typedDeclaration := declaration.(type) {
	case *ast.FuncDecl:
		if typedDeclaration.Recv != nil || typedDeclaration.Name == nil {
			return
		}
		analysis.packageLevelIdentifierNames[typedDeclaration.Name.Name] = struct{}{}
	case *ast.GenDecl:
		for _, declarationSpec := range typedDeclaration.Specs {
			switch typedDeclarationSpec := declarationSpec.(type) {
			case *ast.ValueSpec:
				for _, declaredIdentifier := range typedDeclarationSpec.Names {
					if declaredIdentifier == nil {
						continue
					}
					analysis.packageLevelIdentifierNames[declaredIdentifier.Name] = struct{}{}
				}
			case *ast.TypeSpec:
				if typedDeclarationSpec.Name == nil {
					continue
				}
				analysis.packageLevelIdentifierNames[typedDeclarationSpec.Name.Name] = struct{}{}
			}
		}
	}
}

func (analysis *backendRoutePackageAnalysis) isKnownPackageLevelIdentifierName(
	identifierName string,
) bool {
	if analysis == nil || analysis.packageLevelIdentifierNames == nil {
		return false
	}
	_, hasIdentifier := analysis.packageLevelIdentifierNames[identifierName]
	return hasIdentifier
}

func isPredeclaredUnqualifiedIdentifierName(identifierName string) bool {
	if identifierName == "" {
		return false
	}
	return types.Universe.Lookup(identifierName) != nil
}

func collectFunctionScopeRangesFromParsedFiles(
	parsedServerFiles []*parsedServerRouteFile,
) []tokenPosRange {
	functionScopeRanges := make([]tokenPosRange, 0)
	for _, parsedServerFile := range parsedServerFiles {
		ast.Inspect(
			parsedServerFile.parsedAST,
			func(currentNode ast.Node) bool {
				if currentNode == nil {
					return true
				}
				switch typedNode := currentNode.(type) {
				case *ast.FuncDecl:
					if typedNode.Body == nil {
						return true
					}
					functionScopeRanges = append(functionScopeRanges, tokenPosRange{
						start: typedNode.Body.Pos(),
						end:   typedNode.Body.End(),
					})
				case *ast.FuncLit:
					if typedNode.Body == nil {
						return true
					}
					functionScopeRanges = append(functionScopeRanges, tokenPosRange{
						start: typedNode.Body.Pos(),
						end:   typedNode.Body.End(),
					})
				}
				return true
			},
		)
	}
	return functionScopeRanges
}

func (analysis *backendRoutePackageAnalysis) isPositionInFunctionScope(
	position token.Pos,
) bool {
	if position == token.NoPos {
		return false
	}
	for _, functionScopeRange := range analysis.functionScopeRanges {
		if position >= functionScopeRange.start &&
			position <= functionScopeRange.end {
			return true
		}
	}
	return false
}

func (analysis *backendRoutePackageAnalysis) discoveryRootFiles() []*parsedServerRouteFile {
	rootFiles := make(
		[]*parsedServerRouteFile,
		0,
		len(analysis.rootFilePathSet),
	)
	for _, parsedServerFile := range analysis.files {
		if _, isRootFile := analysis.rootFilePathSet[parsedServerFile.path]; !isRootFile {
			continue
		}
		rootFiles = append(rootFiles, parsedServerFile)
	}
	sort.Slice(rootFiles, func(i int, j int) bool {
		return rootFiles[i].path < rootFiles[j].path
	})
	return rootFiles
}

func (analysis *backendRoutePackageAnalysis) discoverLoaderPatterns() ([]string, error) {
	state := &backendRouteDiscoveryState{
		loaderPatternSet:                    map[string]struct{}{},
		activeFunctions:                     map[*ast.FuncDecl]struct{}{},
		discoveredVormaRegistrationCallByID: map[string]*discoveredVormaRegistrationCall{},
	}

	if err := analysis.discoverRouteRegistrationsInRootFiles(state); err != nil {
		return nil, err
	}

	loaderPatterns := make([]string, 0, len(state.loaderPatternSet))
	for loaderPattern := range state.loaderPatternSet {
		loaderPatterns = append(loaderPatterns, loaderPattern)
	}
	sort.Strings(loaderPatterns)
	return loaderPatterns, nil
}

func (analysis *backendRoutePackageAnalysis) discoverRouteRegistrationsInRootFiles(
	state *backendRouteDiscoveryState,
) error {
	for _, parsedServerFile := range analysis.discoveryRootFiles() {
		for _, declaration := range parsedServerFile.parsedAST.Decls {
			switch typedDeclaration := declaration.(type) {
			case *ast.GenDecl:
				if typedDeclaration.Tok != token.VAR {
					continue
				}
				if err := analysis.discoverLoaderPatternsFromVarDeclaration(
					typedDeclaration,
					parsedServerFile,
					state,
				); err != nil {
					return err
				}
			case *ast.FuncDecl:
				if typedDeclaration.Recv != nil || typedDeclaration.Name.Name != "init" {
					continue
				}
				if err := analysis.discoverLoaderPatternsFromFunctionDeclaration(
					&localFunctionDeclaration{
						name: "init",
						decl: typedDeclaration,
						file: parsedServerFile,
					},
					nil,
					state,
				); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (analysis *backendRoutePackageAnalysis) discoverLoaderPatternsFromVarDeclaration(
	varDeclaration *ast.GenDecl,
	parsedServerFile *parsedServerRouteFile,
	state *backendRouteDiscoveryState,
) error {
	for _, specNode := range varDeclaration.Specs {
		valueSpec, isValueSpec := specNode.(*ast.ValueSpec)
		if !isValueSpec {
			continue
		}
		for _, valueExpression := range valueSpec.Values {
			if err := walkCallExpressionsInNode(
				valueExpression,
				func(call *ast.CallExpr) error {
					return analysis.analyzeCallExpression(
						call,
						parsedServerFile,
						nil,
						state,
					)
				},
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (analysis *backendRoutePackageAnalysis) analyzeCallExpression(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
	state *backendRouteDiscoveryState,
) error {
	canonicalCall, isCanonicalCall, err := analysis.parseCanonicalRouteRegistrationCall(
		call,
		parsedServerFile,
		bindings,
	)
	if err != nil {
		return analysis.withPositionError(call.Pos(), err)
	}
	if isCanonicalCall {
		if canonicalCall.isLoader {
			state.loaderPatternSet[canonicalCall.pattern] = struct{}{}
		}
		if canonicalCall.discoveredVormaRegistrationCall != nil {
			callID, err := analysis.discoveredVormaRegistrationCallID(
				canonicalCall.discoveredVormaRegistrationCall,
			)
			if err != nil {
				return analysis.withPositionError(call.Pos(), err)
			}
			state.discoveredVormaRegistrationCallByID[callID] = canonicalCall.discoveredVormaRegistrationCall
		}
		return nil
	}

	localFunctionDeclaration, hasLocalFunction, err := analysis.resolveLocalFunctionDeclarationForCall(
		call,
		call.Pos(),
	)
	if err != nil {
		return err
	}
	if !hasLocalFunction {
		calleeExpression := unwrapGenericCalleeExpression(call.Fun)
		functionLiteral, isFunctionLiteral := calleeExpression.(*ast.FuncLit)
		if !isFunctionLiteral {
			return nil
		}
		childBindings := bindCallArgumentsToParameterNames(
			functionLiteral.Type,
			call.Args,
			bindings,
		)
		return analysis.discoverLoaderPatternsFromFunctionLiteral(
			functionLiteral,
			parsedServerFile,
			childBindings,
			state,
		)
	}

	childBindings := bindCallArgumentsToParameterNames(
		localFunctionDeclaration.decl.Type,
		call.Args,
		bindings,
	)
	return analysis.discoverLoaderPatternsFromFunctionDeclaration(
		localFunctionDeclaration,
		childBindings,
		state,
	)
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalRouteRegistrationCall(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	calleeExpression := unwrapGenericCalleeExpression(call.Fun)

	switch typedCallee := calleeExpression.(type) {
	case *ast.SelectorExpr:
		importAlias, isImportAlias := typedCallee.X.(*ast.Ident)
		if isImportAlias && importAlias.Obj == nil {
			importPath, hasImportAlias := parsedServerFile.importAliases[importAlias.Name]
			if hasImportAlias {
				registrationCall, isRegistrationCall, err := analysis.parseCanonicalRouteRegistrationCallByImportPath(
					call,
					parsedServerFile,
					bindings,
					importPath,
					typedCallee.Sel.Name,
				)
				if err != nil {
					return nil, false, err
				}
				if isRegistrationCall {
					return registrationCall, true, nil
				}
			}
		}
	case *ast.Ident:
		if typedCallee.Obj == nil {
			for importPath := range parsedServerFile.dotImportPaths {
				registrationCall, isRegistrationCall, err := analysis.parseCanonicalRouteRegistrationCallByImportPath(
					call,
					parsedServerFile,
					bindings,
					importPath,
					typedCallee.Name,
				)
				if err != nil {
					return nil, false, err
				}
				if isRegistrationCall {
					return registrationCall, true, nil
				}
			}
		}
	default:
		// Continue to fallback resolution when canonical AST import matching is unavailable.
	}

	if !callExpressionCouldReferenceCanonicalRouteRegistrationByName(call) {
		return nil, false, nil
	}

	if goTypesRegistrationCall, isRegistrationCall, err := analysis.parseCanonicalRouteRegistrationCallByGoTypesIdentity(
		call,
		parsedServerFile,
		bindings,
	); err != nil {
		return nil, false, err
	} else if isRegistrationCall {
		return goTypesRegistrationCall, true, nil
	}

	return nil, false, nil
}

var canonicalRouteRegistrationFunctionNames = map[string]struct{}{
	"DefineLoaderForRegistration": {},
	"DefineActionForRegistration": {},
	"AddTaskHandler":              {},
}

func callExpressionCouldReferenceCanonicalRouteRegistrationByName(
	call *ast.CallExpr,
) bool {
	if call == nil {
		return false
	}

	calleeExpression := unwrapGenericCalleeExpression(call.Fun)
	switch typedCallee := calleeExpression.(type) {
	case *ast.SelectorExpr:
		_, isCanonicalName := canonicalRouteRegistrationFunctionNames[typedCallee.Sel.Name]
		return isCanonicalName
	case *ast.Ident:
		_, isCanonicalName := canonicalRouteRegistrationFunctionNames[typedCallee.Name]
		return isCanonicalName
	default:
		return false
	}
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalRouteRegistrationCallByGoTypesIdentity(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	if err := analysis.ensureGoTypesInfoInitialized(); err != nil {
		return nil, false, err
	}
	if analysis.goTypesInfo == nil {
		return nil, false, nil
	}

	importPath, functionName, hasGoTypesIdentity := analysis.resolveGoTypesFunctionIdentityForCall(
		call,
	)
	if !hasGoTypesIdentity {
		return nil, false, nil
	}
	return analysis.parseCanonicalRouteRegistrationCallByImportPath(
		call,
		parsedServerFile,
		bindings,
		importPath,
		functionName,
	)
}

func (analysis *backendRoutePackageAnalysis) resolveGoTypesFunctionIdentityForCall(
	call *ast.CallExpr,
) (string, string, bool) {
	if analysis.goTypesInfo == nil {
		return "", "", false
	}

	calleeExpression := unwrapGenericCalleeExpression(call.Fun)
	switch typedCallee := calleeExpression.(type) {
	case *ast.Ident:
		functionObject, hasFunctionObject := analysis.goTypesInfo.Uses[typedCallee]
		if !hasFunctionObject || functionObject == nil {
			return "", "", false
		}
		return qualifiedGoTypesObjectIdentity(functionObject)
	case *ast.SelectorExpr:
		functionObject, hasFunctionObject := analysis.goTypesInfo.Uses[typedCallee.Sel]
		if !hasFunctionObject || functionObject == nil {
			return "", "", false
		}
		return qualifiedGoTypesObjectIdentity(functionObject)
	default:
		return "", "", false
	}
}

func qualifiedGoTypesObjectIdentity(
	functionObject types.Object,
) (string, string, bool) {
	if functionObject == nil {
		return "", "", false
	}

	functionPackage := functionObject.Pkg()
	if functionPackage == nil {
		return "", "", false
	}
	return functionPackage.Path(), functionObject.Name(), true
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalRouteRegistrationCallByImportPath(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
	importPath string,
	functionName string,
) (*canonicalRouteRegistrationCall, bool, error) {
	switch {
	case importPath == "github.com/vormadev/vorma" && functionName == "DefineLoaderForRegistration":
		return analysis.parseCanonicalVormaLoaderRegistrationCall(
			call,
			parsedServerFile,
			bindings,
		)
	case importPath == "github.com/vormadev/vorma" && functionName == "DefineActionForRegistration":
		return analysis.parseCanonicalVormaActionRegistrationCall(
			call,
			parsedServerFile,
			bindings,
		)
	case importPath == "github.com/vormadev/vorma/kit/nestedmux" &&
		functionName == "AddTaskHandler":
		return analysis.parseCanonicalLoaderRegistrationCall(
			call,
			bindings,
			"github.com/vormadev/vorma/kit/nestedmux.AddTaskHandler",
			1,
		)
	case importPath == "github.com/vormadev/vorma/kit/mux" && functionName == "AddTaskHandler":
		return analysis.parseCanonicalActionRegistrationCall(
			call,
			bindings,
			"github.com/vormadev/vorma/kit/mux.AddTaskHandler",
			1,
			2,
		)
	default:
		return nil, false, nil
	}
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalVormaLoaderRegistrationCall(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	_ = parsedServerFile
	canonicalCall, isCanonicalCall, err := analysis.parseCanonicalLoaderRegistrationCall(
		call,
		bindings,
		"github.com/vormadev/vorma.DefineLoaderForRegistration",
		1,
	)
	if err != nil || !isCanonicalCall {
		return canonicalCall, isCanonicalCall, err
	}
	if len(call.Args) < 4 {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.DefineLoaderForRegistration requires app, pattern, handler, and decorateCtx arguments",
		)
	}

	appExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[0],
		bindings,
		0,
	)
	handlerExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[2],
		bindings,
		0,
	)
	decorateCtxExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[3],
		bindings,
		0,
	)

	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		appExpression,
		"github.com/vormadev/vorma.DefineLoaderForRegistration app argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		handlerExpression,
		"github.com/vormadev/vorma.DefineLoaderForRegistration handler argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		decorateCtxExpression,
		"github.com/vormadev/vorma.DefineLoaderForRegistration decorateCtx argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}

	canonicalCall.discoveredVormaRegistrationCall = &discoveredVormaRegistrationCall{
		isLoader:              true,
		appExpression:         appExpression,
		patternExpression:     quotedStringExpression(canonicalCall.pattern),
		handlerExpression:     handlerExpression,
		decorateCtxExpression: decorateCtxExpression,
		sourcePosition:        call.Pos(),
	}
	return canonicalCall, true, nil
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalVormaActionRegistrationCall(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	canonicalCall, isCanonicalCall, err := analysis.parseCanonicalActionRegistrationCall(
		call,
		bindings,
		"github.com/vormadev/vorma.DefineActionForRegistration",
		1,
		2,
	)
	if err != nil || !isCanonicalCall {
		return canonicalCall, isCanonicalCall, err
	}
	if len(call.Args) < 5 {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.DefineActionForRegistration requires app, method, pattern, handler, and decorateCtx arguments",
		)
	}

	resolvedMethod, isMethod := resolveCompileTimeStringExpression(
		call.Args[1],
		bindings,
		analysis.stringConstResolver,
		0,
	)
	if !isMethod {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.DefineActionForRegistration method argument must resolve to compile-time string",
		)
	}
	resolvedPattern, isPattern := resolveCompileTimeStringExpression(
		call.Args[2],
		bindings,
		analysis.stringConstResolver,
		0,
	)
	if !isPattern {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.DefineActionForRegistration pattern argument must resolve to compile-time string",
		)
	}

	appExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[0],
		bindings,
		0,
	)
	handlerExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[3],
		bindings,
		0,
	)
	decorateCtxExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[4],
		bindings,
		0,
	)

	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		appExpression,
		"github.com/vormadev/vorma.DefineActionForRegistration app argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		handlerExpression,
		"github.com/vormadev/vorma.DefineActionForRegistration handler argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		decorateCtxExpression,
		"github.com/vormadev/vorma.DefineActionForRegistration decorateCtx argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}

	canonicalCall.discoveredVormaRegistrationCall = &discoveredVormaRegistrationCall{
		isLoader:              false,
		appExpression:         appExpression,
		methodExpression:      quotedStringExpression(resolvedMethod),
		patternExpression:     quotedStringExpression(resolvedPattern),
		handlerExpression:     handlerExpression,
		decorateCtxExpression: decorateCtxExpression,
		sourcePosition:        call.Pos(),
	}
	_ = parsedServerFile
	return canonicalCall, true, nil
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalLoaderRegistrationCall(
	call *ast.CallExpr,
	bindings *expressionBindings,
	calleeDisplayName string,
	patternArgumentIndex int,
) (*canonicalRouteRegistrationCall, bool, error) {
	if len(call.Args) <= patternArgumentIndex {
		return nil, false, fmt.Errorf(
			"%s requires a pattern argument",
			calleeDisplayName,
		)
	}

	loaderPattern, isCompileTimePattern := resolveCompileTimeStringExpression(
		call.Args[patternArgumentIndex],
		bindings,
		analysis.stringConstResolver,
		0,
	)
	if !isCompileTimePattern {
		return nil, false, fmt.Errorf(
			"%s pattern argument must resolve to compile-time string",
			calleeDisplayName,
		)
	}

	return &canonicalRouteRegistrationCall{
		isLoader: true,
		pattern:  loaderPattern,
	}, true, nil
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalActionRegistrationCall(
	call *ast.CallExpr,
	bindings *expressionBindings,
	calleeDisplayName string,
	methodArgumentIndex int,
	patternArgumentIndex int,
) (*canonicalRouteRegistrationCall, bool, error) {
	if len(call.Args) <= patternArgumentIndex {
		return nil, false, fmt.Errorf(
			"%s requires method and pattern arguments",
			calleeDisplayName,
		)
	}

	if _, isCompileTimeMethod := resolveCompileTimeStringExpression(
		call.Args[methodArgumentIndex],
		bindings,
		analysis.stringConstResolver,
		0,
	); !isCompileTimeMethod {
		return nil, false, fmt.Errorf(
			"%s method argument must resolve to compile-time string",
			calleeDisplayName,
		)
	}

	if _, isCompileTimePattern := resolveCompileTimeStringExpression(
		call.Args[patternArgumentIndex],
		bindings,
		analysis.stringConstResolver,
		0,
	); !isCompileTimePattern {
		return nil, false, fmt.Errorf(
			"%s pattern argument must resolve to compile-time string",
			calleeDisplayName,
		)
	}

	return &canonicalRouteRegistrationCall{isLoader: false}, true, nil
}

func unwrapGenericCalleeExpression(calleeExpression ast.Expr) ast.Expr {
	currentExpression := calleeExpression
	for {
		switch typedExpression := currentExpression.(type) {
		case *ast.IndexExpr:
			currentExpression = typedExpression.X
		case *ast.IndexListExpr:
			currentExpression = typedExpression.X
		default:
			return currentExpression
		}
	}
}

func (analysis *backendRoutePackageAnalysis) resolveLocalFunctionDeclarationForCall(
	call *ast.CallExpr,
	position token.Pos,
) (*localFunctionDeclaration, bool, error) {
	calleeExpression := unwrapGenericCalleeExpression(call.Fun)
	calleeIdentifier, isIdentifier := calleeExpression.(*ast.Ident)
	if !isIdentifier {
		return nil, false, nil
	}

	if calleeIdentifier.Obj != nil {
		if calleeIdentifier.Obj.Kind != ast.Fun {
			return nil, false, nil
		}
		functionDeclaration, isFunctionDeclaration := calleeIdentifier.Obj.Decl.(*ast.FuncDecl)
		if !isFunctionDeclaration || functionDeclaration.Name == nil {
			return nil, false, nil
		}
		functionDeclarationPosition := functionDeclaration.Name.Pos()
		if functionDeclarationPosition == token.NoPos {
			return nil, false, nil
		}
		localFunctionDeclaration, hasLocalFunction := analysis.localFunctionsByDeclarationPosition[functionDeclarationPosition]
		if !hasLocalFunction {
			return nil, false, nil
		}
		return localFunctionDeclaration, true, nil
	}

	localFunctionDeclarations := analysis.localFunctionsByName[calleeIdentifier.Name]
	switch len(localFunctionDeclarations) {
	case 0:
		return nil, false, nil
	case 1:
		return localFunctionDeclarations[0], true, nil
	default:
		return nil, false, analysis.withPositionError(
			position,
			fmt.Errorf(
				"ambiguous local function %q in route discovery; choose unique function name",
				calleeIdentifier.Name,
			),
		)
	}
}

func bindCallArgumentsToParameterNames(
	functionType *ast.FuncType,
	callArguments []ast.Expr,
	parentBindings *expressionBindings,
) *expressionBindings {
	if functionType == nil || functionType.Params == nil {
		return &expressionBindings{
			values: map[string]ast.Expr{},
			parent: parentBindings,
		}
	}

	parameterNames := make([]string, 0)
	for _, parameterField := range functionType.Params.List {
		for _, parameterName := range parameterField.Names {
			parameterNames = append(parameterNames, parameterName.Name)
		}
	}

	childBindings := &expressionBindings{
		values: map[string]ast.Expr{},
		parent: parentBindings,
	}
	for parameterIndex, parameterName := range parameterNames {
		if parameterIndex >= len(callArguments) {
			continue
		}
		childBindings.values[parameterName] = callArguments[parameterIndex]
	}
	return childBindings
}

func (analysis *backendRoutePackageAnalysis) discoverLoaderPatternsFromFunctionLiteral(
	functionLiteral *ast.FuncLit,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
	state *backendRouteDiscoveryState,
) error {
	if functionLiteral == nil || functionLiteral.Body == nil {
		return nil
	}

	for _, statement := range functionLiteral.Body.List {
		if err := walkCallExpressionsInNode(statement, func(call *ast.CallExpr) error {
			return analysis.analyzeCallExpression(
				call,
				parsedServerFile,
				bindings,
				state,
			)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (analysis *backendRoutePackageAnalysis) discoverLoaderPatternsFromFunctionDeclaration(
	localFunction *localFunctionDeclaration,
	bindings *expressionBindings,
	state *backendRouteDiscoveryState,
) error {
	if localFunction.decl == nil || localFunction.decl.Body == nil {
		return nil
	}

	if _, isActive := state.activeFunctions[localFunction.decl]; isActive {
		return analysis.withPositionError(
			localFunction.decl.Pos(),
			fmt.Errorf(
				"recursive route registration call graph through %q is unsupported",
				localFunction.name,
			),
		)
	}
	state.activeFunctions[localFunction.decl] = struct{}{}
	defer delete(state.activeFunctions, localFunction.decl)

	for _, statement := range localFunction.decl.Body.List {
		if err := walkCallExpressionsInNode(statement, func(call *ast.CallExpr) error {
			return analysis.analyzeCallExpression(
				call,
				localFunction.file,
				bindings,
				state,
			)
		}); err != nil {
			return err
		}
	}

	return nil
}

func walkCallExpressionsInNode(
	node ast.Node,
	visitCall func(*ast.CallExpr) error,
) error {
	var walkError error
	ast.Inspect(node, func(currentNode ast.Node) bool {
		if walkError != nil {
			return false
		}
		if currentNode == nil {
			return true
		}
		if _, isFunctionLiteral := currentNode.(*ast.FuncLit); isFunctionLiteral {
			return false
		}
		callExpression, isCallExpression := currentNode.(*ast.CallExpr)
		if !isCallExpression {
			return true
		}
		walkError = visitCall(callExpression)
		return walkError == nil
	})
	return walkError
}

func resolveCompileTimeStringExpression(
	expression ast.Expr,
	bindings *expressionBindings,
	stringConstResolver *goStringConstResolver,
	depth int,
) (string, bool) {
	if depth > 64 {
		return "", false
	}

	switch typedExpression := expression.(type) {
	case *ast.BasicLit:
		if typedExpression.Kind != token.STRING {
			return "", false
		}
		unquotedValue, err := strconv.Unquote(typedExpression.Value)
		if err != nil {
			return "", false
		}
		return unquotedValue, true
	case *ast.ParenExpr:
		return resolveCompileTimeStringExpression(
			typedExpression.X,
			bindings,
			stringConstResolver,
			depth+1,
		)
	case *ast.BinaryExpr:
		if typedExpression.Op != token.ADD {
			return "", false
		}
		leftValue, leftIsString := resolveCompileTimeStringExpression(
			typedExpression.X,
			bindings,
			stringConstResolver,
			depth+1,
		)
		if !leftIsString {
			return "", false
		}
		rightValue, rightIsString := resolveCompileTimeStringExpression(
			typedExpression.Y,
			bindings,
			stringConstResolver,
			depth+1,
		)
		if !rightIsString {
			return "", false
		}
		return leftValue + rightValue, true
	case *ast.Ident:
		if bindings != nil {
			if boundExpression, hasBoundExpression := bindings.resolveBoundExpression(typedExpression.Name); hasBoundExpression {
				return resolveCompileTimeStringExpression(
					boundExpression,
					bindings,
					stringConstResolver,
					depth+1,
				)
			}
		}
		return stringConstResolver.resolveConstIdentifier(typedExpression.Name)
	default:
		return "", false
	}
}

func quotedStringExpression(value string) ast.Expr {
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: strconv.Quote(value),
	}
}

func resolveExpressionWithBindingsForDiscoveredRegistration(
	expression ast.Expr,
	bindings *expressionBindings,
	depth int,
) ast.Expr {
	if depth > 64 {
		return expression
	}
	switch typedExpression := expression.(type) {
	case *ast.Ident:
		if bindings != nil {
			if boundExpression, hasBoundExpression := bindings.resolveBoundExpression(typedExpression.Name); hasBoundExpression {
				return resolveExpressionWithBindingsForDiscoveredRegistration(
					boundExpression,
					bindings,
					depth+1,
				)
			}
		}
		return typedExpression
	default:
		return typedExpression
	}
}

func (analysis *backendRoutePackageAnalysis) validateDiscoveredRegistrationExpressionForPackageInitScope(
	expression ast.Expr,
	argumentDisplayName string,
	callPosition token.Pos,
) error {
	expressionStart := expression.Pos()
	expressionEnd := expression.End()

	var validationError error
	ast.Inspect(expression, func(currentNode ast.Node) bool {
		if validationError != nil {
			return false
		}
		identifier, isIdentifier := currentNode.(*ast.Ident)
		if !isIdentifier {
			return true
		}
		if identifier.Obj == nil {
			return true
		}

		identifierDefinitionPosition := identifier.Obj.Pos()
		if identifierDefinitionPosition == token.NoPos {
			return true
		}
		// Symbols declared inside the emitted expression itself are safe.
		if expressionStart != token.NoPos && expressionEnd != token.NoPos {
			if identifierDefinitionPosition >= expressionStart &&
				identifierDefinitionPosition <= expressionEnd {
				return true
			}
		}
		if !analysis.isPositionInFunctionScope(identifierDefinitionPosition) {
			return true
		}

		validationError = analysis.withPositionError(
			callPosition,
			fmt.Errorf(
				"%s references function-local symbol %q which cannot be emitted at package init scope",
				argumentDisplayName,
				identifier.Name,
			),
		)
		return false
	})
	if validationError != nil {
		return validationError
	}
	return nil
}

func (bindings *expressionBindings) resolveBoundExpression(
	expressionName string,
) (ast.Expr, bool) {
	for currentBindings := bindings; currentBindings != nil; currentBindings = currentBindings.parent {
		boundExpression, hasBoundExpression := currentBindings.values[expressionName]
		if !hasBoundExpression {
			continue
		}
		if boundIdentifier, isBoundIdentifier := boundExpression.(*ast.Ident); isBoundIdentifier &&
			boundIdentifier.Name == expressionName {
			continue
		}
		return boundExpression, true
	}
	return nil, false
}

func (analysis *backendRoutePackageAnalysis) withPositionError(
	position token.Pos,
	err error,
) error {
	filePosition := analysis.goFileSet.Position(position)
	if filePosition.Filename == "" {
		return err
	}
	lineNumber := filePosition.Line
	if lineNumber <= 0 {
		lineNumber = 1
	}
	return fmt.Errorf(
		"%s:%d: %w",
		filepath.ToSlash(filePosition.Filename),
		lineNumber,
		err,
	)
}

func resolveServerRouteDefinitionFiles(
	v *vormaruntime.Vorma,
) ([]string, error) {
	return defaultBackendRouteDiscoveryExecutor.resolveServerRouteDefinitionFiles(
		v,
	)
}

func (executor backendRouteDiscoveryExecutor) resolveServerRouteDefinitionFiles(
	v *vormaruntime.Vorma,
) ([]string, error) {
	if v == nil {
		return nil, fmt.Errorf("vorma runtime is required")
	}
	if v.Config == nil {
		return nil, fmt.Errorf("vorma config is required")
	}

	normalizedPatterns, err := normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ServerRouteDefinitionPatterns,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"normalize server route definition patterns: %w",
			err,
		)
	}
	if len(normalizedPatterns) == 0 {
		return nil, nil
	}

	matchedFilesByPath := map[string]struct{}{}
	for _, routeDefinitionPattern := range normalizedPatterns {
		matchedFiles, err := executor.resolveServerRouteDefinitionPattern(
			routeDefinitionPattern,
		)
		if err != nil {
			return nil, err
		}
		for _, matchedFile := range matchedFiles {
			matchedFilesByPath[filepath.ToSlash(filepath.Clean(matchedFile))] = struct{}{}
		}
	}

	if len(matchedFilesByPath) == 0 {
		return nil, fmt.Errorf(
			"no server route definition files matched patterns: %s",
			strings.Join(normalizedPatterns, ", "),
		)
	}

	serverRouteDefinitionFiles := make([]string, 0, len(matchedFilesByPath))
	for serverRouteDefinitionFile := range matchedFilesByPath {
		serverRouteDefinitionFiles = append(
			serverRouteDefinitionFiles,
			serverRouteDefinitionFile,
		)
	}
	sort.Strings(serverRouteDefinitionFiles)
	return serverRouteDefinitionFiles, nil
}

func (executor backendRouteDiscoveryExecutor) resolveServerRouteDefinitionPattern(
	routeDefinitionPattern string,
) ([]string, error) {
	if !patternContainsGlobMeta(routeDefinitionPattern) {
		fileInfo, err := executor.dependencies.statPath(routeDefinitionPattern)
		if err != nil {
			return nil, fmt.Errorf(
				"stat server route definition path %q: %w",
				routeDefinitionPattern,
				err,
			)
		}
		if fileInfo.IsDir() {
			return nil, fmt.Errorf(
				"server route definition path %q is a directory",
				routeDefinitionPattern,
			)
		}
		if filepath.Ext(routeDefinitionPattern) != ".go" {
			return nil, fmt.Errorf(
				"server route definition path %q must be a .go file",
				routeDefinitionPattern,
			)
		}
		return []string{routeDefinitionPattern}, nil
	}

	matchedPaths, err := executor.dependencies.expandPattern(
		routeDefinitionPattern,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"expand server route definition pattern %q: %w",
			routeDefinitionPattern,
			err,
		)
	}

	matchedGoFiles := make([]string, 0, len(matchedPaths))
	for _, matchedPath := range matchedPaths {
		fileInfo, err := executor.dependencies.statPath(matchedPath)
		if err != nil {
			return nil, fmt.Errorf(
				"stat server route definition path %q: %w",
				matchedPath,
				err,
			)
		}
		if fileInfo.IsDir() {
			continue
		}
		if filepath.Ext(matchedPath) != ".go" {
			continue
		}
		matchedGoFiles = append(matchedGoFiles, matchedPath)
	}
	return matchedGoFiles, nil
}

func parseServerRouteDefinitionFileWithDependencies(
	goFileSet *token.FileSet,
	serverRouteDefinitionFile string,
	dependencies backendRouteDiscoveryDependencies,
) (*ast.File, error) {
	dependencies = normalizeBackendRouteDiscoveryDependencies(dependencies)

	fileBytes, err := dependencies.readFile(serverRouteDefinitionFile)
	if err != nil {
		return nil, fmt.Errorf(
			"read server route definition file %q: %w",
			serverRouteDefinitionFile,
			err,
		)
	}
	parsedFile, err := dependencies.parseGoSourceAST(
		goFileSet,
		serverRouteDefinitionFile,
		fileBytes,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"parse server route definition file %q: %w",
			serverRouteDefinitionFile,
			err,
		)
	}
	return parsedFile, nil
}

func collectPackageStringConstExpressions(
	parsedGoFiles []*ast.File,
) map[string]ast.Expr {
	stringConstExpressions := map[string]ast.Expr{}
	for _, parsedFile := range parsedGoFiles {
		for _, declaration := range parsedFile.Decls {
			constDeclaration, isConstDeclaration := declaration.(*ast.GenDecl)
			if !isConstDeclaration || constDeclaration.Tok != token.CONST {
				continue
			}
			for _, specNode := range constDeclaration.Specs {
				valueSpec, isValueSpec := specNode.(*ast.ValueSpec)
				if !isValueSpec || len(valueSpec.Values) == 0 {
					continue
				}

				if len(valueSpec.Values) == 1 {
					for _, constName := range valueSpec.Names {
						stringConstExpressions[constName.Name] = valueSpec.Values[0]
					}
					continue
				}

				if len(valueSpec.Values) != len(valueSpec.Names) {
					continue
				}
				for valueIndex, constName := range valueSpec.Names {
					stringConstExpressions[constName.Name] = valueSpec.Values[valueIndex]
				}
			}
		}
	}
	return stringConstExpressions
}

type goStringConstResolver struct {
	stringConstExpressions map[string]ast.Expr
	resolvedValues         map[string]string
	currentlyResolving     map[string]struct{}
}

func newGoStringConstResolver(
	stringConstExpressions map[string]ast.Expr,
) *goStringConstResolver {
	return &goStringConstResolver{
		stringConstExpressions: stringConstExpressions,
		resolvedValues:         map[string]string{},
		currentlyResolving:     map[string]struct{}{},
	}
}

func (resolver *goStringConstResolver) Resolve(
	expression ast.Expr,
) (string, bool) {
	switch typedExpression := expression.(type) {
	case *ast.BasicLit:
		if typedExpression.Kind != token.STRING {
			return "", false
		}
		unquotedValue, err := strconv.Unquote(typedExpression.Value)
		if err != nil {
			return "", false
		}
		return unquotedValue, true
	case *ast.Ident:
		return resolver.resolveConstIdentifier(typedExpression.Name)
	case *ast.ParenExpr:
		return resolver.Resolve(typedExpression.X)
	case *ast.BinaryExpr:
		if typedExpression.Op != token.ADD {
			return "", false
		}
		leftValue, leftIsString := resolver.Resolve(typedExpression.X)
		if !leftIsString {
			return "", false
		}
		rightValue, rightIsString := resolver.Resolve(typedExpression.Y)
		if !rightIsString {
			return "", false
		}
		return leftValue + rightValue, true
	default:
		return "", false
	}
}

func (resolver *goStringConstResolver) resolveConstIdentifier(
	constIdentifier string,
) (string, bool) {
	if cachedValue, hasCachedValue := resolver.resolvedValues[constIdentifier]; hasCachedValue {
		return cachedValue, true
	}
	if _, isResolving := resolver.currentlyResolving[constIdentifier]; isResolving {
		return "", false
	}

	constExpression, hasConstExpression := resolver.stringConstExpressions[constIdentifier]
	if !hasConstExpression {
		return "", false
	}

	resolver.currentlyResolving[constIdentifier] = struct{}{}
	defer delete(resolver.currentlyResolving, constIdentifier)

	resolvedValue, ok := resolver.Resolve(constExpression)
	if !ok {
		return "", false
	}
	resolver.resolvedValues[constIdentifier] = resolvedValue
	return resolvedValue, true
}

const discoveredRouteRegistrarGeneratedFilename = "vorma_discovered_routes.gen.go"

type discoveredRouteRegistrarOverlay struct {
	goOverlayConfigPath   string
	cleanupTemporaryFiles func() error
}

func (overlay *discoveredRouteRegistrarOverlay) cleanup() error {
	if overlay == nil || overlay.cleanupTemporaryFiles == nil {
		return nil
	}
	return overlay.cleanupTemporaryFiles()
}

type backendRouteRegistrarOverlayDependencies struct {
	makeTempDir  func(string, string) (string, error)
	writeFile    func(string, []byte, os.FileMode) error
	readDir      func(string) ([]os.DirEntry, error)
	readFile     func(string) ([]byte, error)
	removeAll    func(string) error
	marshalJSON  func(any) ([]byte, error)
	absolutePath func(string) (string, error)
}

type backendRouteRegistrarOverlayExecutor struct {
	dependencies           backendRouteRegistrarOverlayDependencies
	routeDiscoveryExecutor backendRouteDiscoveryExecutor
}

var defaultBackendRouteRegistrarOverlayExecutor = newBackendRouteRegistrarOverlayExecutor(
	backendRouteRegistrarOverlayDependencies{},
	backendRouteDiscoveryExecutor{},
)

func defaultBackendRouteRegistrarOverlayDependencies() backendRouteRegistrarOverlayDependencies {
	return backendRouteRegistrarOverlayDependencies{
		makeTempDir:  os.MkdirTemp,
		writeFile:    os.WriteFile,
		readDir:      os.ReadDir,
		readFile:     os.ReadFile,
		removeAll:    os.RemoveAll,
		marshalJSON:  json.Marshal,
		absolutePath: filepath.Abs,
	}
}

func normalizeBackendRouteRegistrarOverlayDependencies(
	dependencies backendRouteRegistrarOverlayDependencies,
) backendRouteRegistrarOverlayDependencies {
	defaultDependencies := defaultBackendRouteRegistrarOverlayDependencies()

	if dependencies.makeTempDir == nil {
		dependencies.makeTempDir = defaultDependencies.makeTempDir
	}
	if dependencies.writeFile == nil {
		dependencies.writeFile = defaultDependencies.writeFile
	}
	if dependencies.readDir == nil {
		dependencies.readDir = defaultDependencies.readDir
	}
	if dependencies.readFile == nil {
		dependencies.readFile = defaultDependencies.readFile
	}
	if dependencies.removeAll == nil {
		dependencies.removeAll = defaultDependencies.removeAll
	}
	if dependencies.marshalJSON == nil {
		dependencies.marshalJSON = defaultDependencies.marshalJSON
	}
	if dependencies.absolutePath == nil {
		dependencies.absolutePath = defaultDependencies.absolutePath
	}

	return dependencies
}

func newBackendRouteRegistrarOverlayExecutor(
	dependencies backendRouteRegistrarOverlayDependencies,
	routeDiscoveryExecutor backendRouteDiscoveryExecutor,
) backendRouteRegistrarOverlayExecutor {
	if routeDiscoveryExecutor.dependencies.expandPattern == nil {
		routeDiscoveryExecutor = defaultBackendRouteDiscoveryExecutor
	}
	return backendRouteRegistrarOverlayExecutor{
		dependencies: normalizeBackendRouteRegistrarOverlayDependencies(
			dependencies,
		),
		routeDiscoveryExecutor: routeDiscoveryExecutor,
	}
}

type goOverlayReplaceConfiguration struct {
	Replace map[string]string `json:"Replace"`
}

type discoveredRouteRegistrarSourceArtifact struct {
	targetFilePath string
	sourceBytes    []byte
}

type discoveredRouteRegistrarArtifactCacheEntry struct {
	sourceFingerprint string
	artifacts         []discoveredRouteRegistrarSourceArtifact
	lastAccessSeq     uint64
}

type discoveredRouteRegistrarArtifactCache struct {
	mutex      sync.Mutex
	maxEntries int
	accessSeq  uint64
	entries    map[string]discoveredRouteRegistrarArtifactCacheEntry
}

const discoveredRouteRegistrarArtifactCacheDefaultMaxEntries = 128

func newDiscoveredRouteRegistrarArtifactCache(
	maxEntries int,
) *discoveredRouteRegistrarArtifactCache {
	if maxEntries <= 0 {
		maxEntries = discoveredRouteRegistrarArtifactCacheDefaultMaxEntries
	}

	return &discoveredRouteRegistrarArtifactCache{
		maxEntries: maxEntries,
		entries:    map[string]discoveredRouteRegistrarArtifactCacheEntry{},
	}
}

func (cache *discoveredRouteRegistrarArtifactCache) get(
	cacheKey string,
	sourceFingerprint string,
) ([]discoveredRouteRegistrarSourceArtifact, bool) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cacheEntry, hasCacheEntry := cache.entries[cacheKey]
	if !hasCacheEntry {
		return nil, false
	}
	if cacheEntry.sourceFingerprint != sourceFingerprint {
		delete(cache.entries, cacheKey)
		return nil, false
	}

	cache.accessSeq++
	cacheEntry.lastAccessSeq = cache.accessSeq
	cache.entries[cacheKey] = cacheEntry

	return cloneDiscoveredRouteRegistrarSourceArtifacts(
		cacheEntry.artifacts,
	), true
}

func (cache *discoveredRouteRegistrarArtifactCache) set(
	cacheKey string,
	sourceFingerprint string,
	artifacts []discoveredRouteRegistrarSourceArtifact,
) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.accessSeq++
	cache.entries[cacheKey] = discoveredRouteRegistrarArtifactCacheEntry{
		sourceFingerprint: sourceFingerprint,
		artifacts: cloneDiscoveredRouteRegistrarSourceArtifacts(
			artifacts,
		),
		lastAccessSeq: cache.accessSeq,
	}
	cache.evictEntriesOverCapacityLocked()
}

func (cache *discoveredRouteRegistrarArtifactCache) evictEntriesOverCapacityLocked() {
	for cache.maxEntries > 0 && len(cache.entries) > cache.maxEntries {
		leastRecentlyUsedCacheKey := ""
		var leastRecentlyUsedAccessSeq uint64
		for candidateCacheKey, candidateCacheEntry := range cache.entries {
			if leastRecentlyUsedCacheKey == "" {
				leastRecentlyUsedCacheKey = candidateCacheKey
				leastRecentlyUsedAccessSeq = candidateCacheEntry.lastAccessSeq
				continue
			}

			if candidateCacheEntry.lastAccessSeq < leastRecentlyUsedAccessSeq {
				leastRecentlyUsedCacheKey = candidateCacheKey
				leastRecentlyUsedAccessSeq = candidateCacheEntry.lastAccessSeq
				continue
			}
			if candidateCacheEntry.lastAccessSeq == leastRecentlyUsedAccessSeq &&
				candidateCacheKey < leastRecentlyUsedCacheKey {
				leastRecentlyUsedCacheKey = candidateCacheKey
			}
		}
		delete(cache.entries, leastRecentlyUsedCacheKey)
	}
}

func cloneDiscoveredRouteRegistrarSourceArtifacts(
	artifacts []discoveredRouteRegistrarSourceArtifact,
) []discoveredRouteRegistrarSourceArtifact {
	if len(artifacts) == 0 {
		return nil
	}

	clonedArtifacts := make(
		[]discoveredRouteRegistrarSourceArtifact,
		0,
		len(artifacts),
	)
	for _, artifact := range artifacts {
		clonedArtifacts = append(
			clonedArtifacts,
			discoveredRouteRegistrarSourceArtifact{
				targetFilePath: artifact.targetFilePath,
				sourceBytes:    append([]byte(nil), artifact.sourceBytes...),
			},
		)
	}
	return clonedArtifacts
}

func prepareDiscoveredRouteRegistrarOverlay(
	v *vormaruntime.Vorma,
) (*discoveredRouteRegistrarOverlay, error) {
	return defaultBackendRouteRegistrarOverlayExecutor.prepareDiscoveredRouteRegistrarOverlay(
		v,
	)
}

func prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
	v *vormaruntime.Vorma,
	discoveredRouteRegistrarArtifactsCache *discoveredRouteRegistrarArtifactCache,
) (*discoveredRouteRegistrarOverlay, error) {
	return defaultBackendRouteRegistrarOverlayExecutor.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
		v,
		discoveredRouteRegistrarArtifactsCache,
	)
}

func prepareDiscoveredRouteRegistrarOverlayWithArtifactCacheAndDiscoveryDependencies(
	v *vormaruntime.Vorma,
	discoveredRouteRegistrarArtifactsCache *discoveredRouteRegistrarArtifactCache,
	discoveryDependencies backendRouteDiscoveryDependencies,
) (*discoveredRouteRegistrarOverlay, error) {
	overlayExecutor := newBackendRouteRegistrarOverlayExecutor(
		backendRouteRegistrarOverlayDependencies{},
		newBackendRouteDiscoveryExecutor(discoveryDependencies),
	)
	return overlayExecutor.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
		v,
		discoveredRouteRegistrarArtifactsCache,
	)
}

func (executor backendRouteRegistrarOverlayExecutor) prepareDiscoveredRouteRegistrarOverlay(
	v *vormaruntime.Vorma,
) (*discoveredRouteRegistrarOverlay, error) {
	return executor.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
		v,
		newDiscoveredRouteRegistrarArtifactCache(
			discoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
		),
	)
}

func (executor backendRouteRegistrarOverlayExecutor) prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
	v *vormaruntime.Vorma,
	discoveredRouteRegistrarArtifactsCache *discoveredRouteRegistrarArtifactCache,
) (*discoveredRouteRegistrarOverlay, error) {
	if discoveredRouteRegistrarArtifactsCache == nil {
		discoveredRouteRegistrarArtifactsCache = newDiscoveredRouteRegistrarArtifactCache(
			discoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
		)
	}

	serverRouteDefinitionFiles, err := executor.routeDiscoveryExecutor.resolveServerRouteDefinitionFiles(
		v,
	)
	if err != nil {
		return nil, err
	}
	if len(serverRouteDefinitionFiles) == 0 {
		return nil, nil
	}

	discoverySourceFingerprint, err := executor.computeDiscoveredRouteRegistrarDiscoveryFingerprint(
		serverRouteDefinitionFiles,
	)
	if err != nil {
		return nil, err
	}

	discoveryCacheKey := discoveredRouteRegistrarDiscoveryCacheKey(v)
	discoveredRegistrarArtifacts, hasCachedArtifacts := discoveredRouteRegistrarArtifactsCache.get(
		discoveryCacheKey,
		discoverySourceFingerprint,
	)
	if !hasCachedArtifacts {
		discoveredRegistrarArtifacts, err = executor.discoverRouteRegistrarSourceArtifacts(
			serverRouteDefinitionFiles,
		)
		if err != nil {
			return nil, err
		}
		discoveredRouteRegistrarArtifactsCache.set(
			discoveryCacheKey,
			discoverySourceFingerprint,
			discoveredRegistrarArtifacts,
		)
	}

	return executor.writeDiscoveredRouteRegistrarOverlay(
		discoveredRegistrarArtifacts,
	)
}

func discoveredRouteRegistrarDiscoveryCacheKey(v *vormaruntime.Vorma) string {
	if v == nil || v.Wave == nil || v.Config == nil {
		return "<nil-vorma>"
	}

	normalizedServerRoutePatterns, err := normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ServerRouteDefinitionPatterns,
	)
	if err != nil {
		normalizedServerRoutePatterns = []string{
			"<invalid-server-route-patterns>",
		}
	}
	return strings.Join(
		[]string{
			filepath.ToSlash(filepath.Clean(v.Wave.ConfigFile())),
			filepath.ToSlash(filepath.Clean(v.Wave.DistDir())),
			filepath.ToSlash(filepath.Clean(v.Wave.StaticPrivateOutDir())),
			filepath.ToSlash(filepath.Clean(v.Wave.StaticPublicOutDir())),
			strings.TrimSpace(v.Config.MainBuildEntry),
			strings.Join(normalizedServerRoutePatterns, ","),
		},
		"|",
	)
}

func (executor backendRouteRegistrarOverlayExecutor) computeDiscoveredRouteRegistrarDiscoveryFingerprint(
	serverRouteDefinitionFiles []string,
) (string, error) {
	normalizedRouteDefinitionFiles := make(
		[]string,
		0,
		len(serverRouteDefinitionFiles),
	)
	packageDirSet := map[string]struct{}{}
	for _, serverRouteDefinitionFile := range serverRouteDefinitionFiles {
		absoluteRouteDefinitionFilePath, err := executor.dependencies.absolutePath(
			serverRouteDefinitionFile,
		)
		if err != nil {
			return "", fmt.Errorf(
				"resolve absolute server route definition file %q for discovered route registrar cache fingerprint: %w",
				serverRouteDefinitionFile,
				err,
			)
		}

		normalizedRouteDefinitionFile := filepath.ToSlash(
			filepath.Clean(absoluteRouteDefinitionFilePath),
		)
		normalizedRouteDefinitionFiles = append(
			normalizedRouteDefinitionFiles,
			normalizedRouteDefinitionFile,
		)
		packageDirSet[filepath.ToSlash(filepath.Clean(filepath.Dir(normalizedRouteDefinitionFile)))] = struct{}{}
	}
	sort.Strings(normalizedRouteDefinitionFiles)

	packageDirs := make([]string, 0, len(packageDirSet))
	for packageDir := range packageDirSet {
		packageDirs = append(packageDirs, packageDir)
	}
	sort.Strings(packageDirs)

	discoveryFingerprintHasher := sha256.New()
	appendFingerprintSegment := func(value string) {
		_, _ = discoveryFingerprintHasher.Write([]byte(value))
		_, _ = discoveryFingerprintHasher.Write([]byte{0})
	}
	appendFingerprintBytes := func(value []byte) {
		_, _ = discoveryFingerprintHasher.Write(value)
		_, _ = discoveryFingerprintHasher.Write([]byte{0})
	}

	appendFingerprintSegment("server-route-definition-files")
	for _, routeDefinitionFile := range normalizedRouteDefinitionFiles {
		appendFingerprintSegment(routeDefinitionFile)
	}

	for _, packageDir := range packageDirs {
		appendFingerprintSegment("package-dir")
		appendFingerprintSegment(packageDir)

		packageDirEntries, err := executor.dependencies.readDir(packageDir)
		if err != nil {
			return "", fmt.Errorf(
				"read package directory %q for discovered route registrar cache fingerprint: %w",
				packageDir,
				err,
			)
		}

		packageGoFiles := make([]string, 0, len(packageDirEntries))
		for _, packageDirEntry := range packageDirEntries {
			if packageDirEntry.IsDir() {
				continue
			}

			entryName := packageDirEntry.Name()
			if filepath.Ext(entryName) != ".go" ||
				strings.HasSuffix(entryName, "_test.go") {
				continue
			}

			packageGoFiles = append(packageGoFiles, entryName)
		}
		sort.Strings(packageGoFiles)

		for _, packageGoFile := range packageGoFiles {
			packageGoFilePath := filepath.ToSlash(
				filepath.Clean(filepath.Join(packageDir, packageGoFile)),
			)
			packageGoFileBytes, err := executor.dependencies.readFile(
				packageGoFilePath,
			)
			if err != nil {
				return "", fmt.Errorf(
					"read package Go source file %q for discovered route registrar cache fingerprint: %w",
					packageGoFilePath,
					err,
				)
			}

			appendFingerprintSegment(packageGoFilePath)
			appendFingerprintBytes(packageGoFileBytes)
		}
	}

	return hex.EncodeToString(discoveryFingerprintHasher.Sum(nil)), nil
}

func (executor backendRouteRegistrarOverlayExecutor) discoverRouteRegistrarSourceArtifacts(
	serverRouteDefinitionFiles []string,
) ([]discoveredRouteRegistrarSourceArtifact, error) {
	packageAnalyses, err := executor.routeDiscoveryExecutor.parseServerRouteFilesIntoPackageAnalyses(
		serverRouteDefinitionFiles,
	)
	if err != nil {
		return nil, err
	}

	discoveredRegistrarArtifacts := make(
		[]discoveredRouteRegistrarSourceArtifact,
		0,
	)
	for _, packageAnalysis := range packageAnalyses {
		discoveredCalls, err := packageAnalysis.discoverVormaRegistrationCalls()
		if err != nil {
			return nil, err
		}
		if len(discoveredCalls) == 0 {
			continue
		}

		generatedSource, err := packageAnalysis.renderDiscoveredRouteRegistrarSource(
			discoveredCalls,
		)
		if err != nil {
			return nil, err
		}

		generatedFilePath := filepath.Join(
			packageAnalysis.packageDir,
			discoveredRouteRegistrarGeneratedFilename,
		)
		absoluteGeneratedFilePath, err := executor.dependencies.absolutePath(
			generatedFilePath,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"resolve absolute generated backend route registrar path %q: %w",
				generatedFilePath,
				err,
			)
		}

		discoveredRegistrarArtifacts = append(
			discoveredRegistrarArtifacts,
			discoveredRouteRegistrarSourceArtifact{
				targetFilePath: filepath.ToSlash(
					filepath.Clean(absoluteGeneratedFilePath),
				),
				sourceBytes: generatedSource,
			},
		)
	}
	sort.Slice(discoveredRegistrarArtifacts, func(i int, j int) bool {
		return discoveredRegistrarArtifacts[i].targetFilePath < discoveredRegistrarArtifacts[j].targetFilePath
	})
	return discoveredRegistrarArtifacts, nil
}

func (executor backendRouteRegistrarOverlayExecutor) writeDiscoveredRouteRegistrarOverlay(
	discoveredRegistrarArtifacts []discoveredRouteRegistrarSourceArtifact,
) (*discoveredRouteRegistrarOverlay, error) {
	if len(discoveredRegistrarArtifacts) == 0 {
		return nil, nil
	}

	overlayTempDir, err := executor.dependencies.makeTempDir(
		"",
		"vorma_discovered_route_registrars_*",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create temporary discovered route registrar overlay directory: %w",
			err,
		)
	}
	cleanupOverlayTempDir := func() error {
		return executor.dependencies.removeAll(overlayTempDir)
	}

	overlayReplacements := map[string]string{}
	for artifactIndex, discoveredArtifact := range discoveredRegistrarArtifacts {
		overlaySourceFilePath := filepath.ToSlash(filepath.Join(
			overlayTempDir,
			fmt.Sprintf("discovered_route_registrar_%d.gen.go", artifactIndex),
		))
		if err := executor.dependencies.writeFile(
			overlaySourceFilePath,
			discoveredArtifact.sourceBytes,
			0o644,
		); err != nil {
			if cleanupErr := cleanupOverlayTempDir(); cleanupErr != nil {
				return nil, fmt.Errorf(
					"write discovered route registrar overlay source %q: %w (cleanup overlay dir failed: %v)",
					overlaySourceFilePath,
					err,
					cleanupErr,
				)
			}
			return nil, fmt.Errorf(
				"write discovered route registrar overlay source %q: %w",
				overlaySourceFilePath,
				err,
			)
		}
		overlayReplacements[discoveredArtifact.targetFilePath] = overlaySourceFilePath
	}

	overlayConfigBytes, err := executor.dependencies.marshalJSON(
		goOverlayReplaceConfiguration{Replace: overlayReplacements},
	)
	if err != nil {
		if cleanupErr := cleanupOverlayTempDir(); cleanupErr != nil {
			return nil, fmt.Errorf(
				"marshal discovered route registrar go overlay config: %w (cleanup overlay dir failed: %v)",
				err,
				cleanupErr,
			)
		}
		return nil, fmt.Errorf(
			"marshal discovered route registrar go overlay config: %w",
			err,
		)
	}

	overlayConfigPath := filepath.ToSlash(filepath.Join(
		overlayTempDir,
		"go-overlay.json",
	))
	if err := executor.dependencies.writeFile(
		overlayConfigPath,
		overlayConfigBytes,
		0o644,
	); err != nil {
		if cleanupErr := cleanupOverlayTempDir(); cleanupErr != nil {
			return nil, fmt.Errorf(
				"write discovered route registrar go overlay config %q: %w (cleanup overlay dir failed: %v)",
				overlayConfigPath,
				err,
				cleanupErr,
			)
		}
		return nil, fmt.Errorf(
			"write discovered route registrar go overlay config %q: %w",
			overlayConfigPath,
			err,
		)
	}

	return &discoveredRouteRegistrarOverlay{
		goOverlayConfigPath: overlayConfigPath,
		cleanupTemporaryFiles: func() error {
			return cleanupOverlayTempDir()
		},
	}, nil
}

func (analysis *backendRoutePackageAnalysis) discoverVormaRegistrationCalls() ([]*discoveredVormaRegistrationCall, error) {
	state := &backendRouteDiscoveryState{
		loaderPatternSet:                    map[string]struct{}{},
		activeFunctions:                     map[*ast.FuncDecl]struct{}{},
		discoveredVormaRegistrationCallByID: map[string]*discoveredVormaRegistrationCall{},
	}

	if err := analysis.discoverRouteRegistrationsInRootFiles(state); err != nil {
		return nil, err
	}

	callIDs := make([]string, 0, len(state.discoveredVormaRegistrationCallByID))
	for callID := range state.discoveredVormaRegistrationCallByID {
		callIDs = append(callIDs, callID)
	}
	sort.Strings(callIDs)

	discoveredCalls := make([]*discoveredVormaRegistrationCall, 0, len(callIDs))
	for _, callID := range callIDs {
		discoveredCalls = append(
			discoveredCalls,
			state.discoveredVormaRegistrationCallByID[callID],
		)
	}
	return discoveredCalls, nil
}

func (analysis *backendRoutePackageAnalysis) renderDiscoveredRouteRegistrarSource(
	discoveredCalls []*discoveredVormaRegistrationCall,
) ([]byte, error) {
	requiredImports := map[string]string{
		"vormagogen": "github.com/vormadev/vorma/vormagogen",
	}
	for _, discoveredCall := range discoveredCalls {
		if err := analysis.collectRequiredImportsForDiscoveredCall(discoveredCall, requiredImports); err != nil {
			return nil, err
		}
	}

	importAliases := make([]string, 0, len(requiredImports))
	for importAlias := range requiredImports {
		importAliases = append(importAliases, importAlias)
	}
	sort.Strings(importAliases)

	var sb strings.Builder
	sb.WriteString("// Code generated by vormabuild. DO NOT EDIT.\n")
	sb.WriteString("package " + analysis.packageName + "\n\n")
	sb.WriteString("import (\n")
	for _, importAlias := range importAliases {
		sb.WriteString(
			fmt.Sprintf("\t%s %q\n", importAlias, requiredImports[importAlias]),
		)
	}
	sb.WriteString(")\n\n")
	sb.WriteString("func init() {\n")
	for _, discoveredCall := range discoveredCalls {
		callExpression, err := analysis.renderDiscoveredCallExpression(
			discoveredCall,
		)
		if err != nil {
			return nil, err
		}
		sb.WriteString("\t_ = ")
		sb.WriteString(callExpression)
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return []byte(sb.String()), nil
}

func (analysis *backendRoutePackageAnalysis) renderDiscoveredCallExpression(
	discoveredCall *discoveredVormaRegistrationCall,
) (string, error) {
	appExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.appExpression,
	)
	if err != nil {
		return "", err
	}
	patternExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.patternExpression,
	)
	if err != nil {
		return "", err
	}
	handlerExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.handlerExpression,
	)
	if err != nil {
		return "", err
	}
	decorateCtxExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.decorateCtxExpression,
	)
	if err != nil {
		return "", err
	}

	if discoveredCall.isLoader {
		return fmt.Sprintf(
			"vormagogen.RegisterLoaderDiscoveredByBuild(%s, %s, %s, %s)",
			appExpression,
			patternExpression,
			handlerExpression,
			decorateCtxExpression,
		), nil
	}

	methodExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.methodExpression,
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"vormagogen.RegisterActionDiscoveredByBuild(%s, %s, %s, %s, %s)",
		appExpression,
		methodExpression,
		patternExpression,
		handlerExpression,
		decorateCtxExpression,
	), nil
}

func (analysis *backendRoutePackageAnalysis) formatDiscoveredCallExpression(
	expression ast.Expr,
) (string, error) {
	var expressionBuffer bytes.Buffer
	if err := printer.Fprint(&expressionBuffer, analysis.goFileSet, expression); err != nil {
		return "", fmt.Errorf(
			"format discovered registration expression: %w",
			err,
		)
	}
	return strings.TrimSpace(expressionBuffer.String()), nil
}

func (analysis *backendRoutePackageAnalysis) discoveredVormaRegistrationCallID(
	discoveredCall *discoveredVormaRegistrationCall,
) (string, error) {
	if discoveredCall == nil {
		return "", fmt.Errorf("discovered registration call is nil")
	}

	renderedExpression, err := analysis.renderDiscoveredCallExpression(
		discoveredCall,
	)
	if err != nil {
		return "", err
	}

	positionKey := fmt.Sprintf("pos:%d", discoveredCall.sourcePosition)
	if analysis != nil && analysis.goFileSet != nil &&
		discoveredCall.sourcePosition != token.NoPos {
		sourceFilePosition := analysis.goFileSet.Position(
			discoveredCall.sourcePosition,
		)
		if sourceFilePosition.Filename != "" {
			lineNumber := sourceFilePosition.Line
			if lineNumber <= 0 {
				lineNumber = 1
			}
			columnNumber := sourceFilePosition.Column
			if columnNumber <= 0 {
				columnNumber = 1
			}
			positionKey = fmt.Sprintf(
				"%s:%d:%d",
				filepath.ToSlash(sourceFilePosition.Filename),
				lineNumber,
				columnNumber,
			)
		}
	}

	return positionKey + "|" + renderedExpression, nil
}

func (analysis *backendRoutePackageAnalysis) collectRequiredImportsForDiscoveredCall(
	discoveredCall *discoveredVormaRegistrationCall,
	requiredImports map[string]string,
) error {
	callExpressions := []ast.Expr{
		discoveredCall.appExpression,
		discoveredCall.patternExpression,
		discoveredCall.handlerExpression,
		discoveredCall.decorateCtxExpression,
	}
	if !discoveredCall.isLoader {
		callExpressions = append(
			callExpressions,
			discoveredCall.methodExpression,
		)
	}

	for _, callExpression := range callExpressions {
		if callExpression == nil {
			continue
		}
		if err := analysis.collectRequiredImportsForExpression(callExpression, requiredImports); err != nil {
			return err
		}
	}
	return nil
}

func (analysis *backendRoutePackageAnalysis) collectRequiredImportsForExpression(
	expression ast.Expr,
	requiredImports map[string]string,
) error {
	var importCollectionErr error
	goTypesInitializationChecked := false
	selectorExpressionIdentifierSet := map[*ast.Ident]struct{}{}
	ast.Inspect(expression, func(currentNode ast.Node) bool {
		if importCollectionErr != nil {
			return false
		}
		switch typedNode := currentNode.(type) {
		case *ast.SelectorExpr:
			selectorExpressionIdentifierSet[typedNode.Sel] = struct{}{}
			importAlias, isImportAlias := typedNode.X.(*ast.Ident)
			if isImportAlias {
				selectorExpressionIdentifierSet[importAlias] = struct{}{}
			}
			if !isImportAlias || importAlias.Obj != nil {
				return true
			}

			importPath, hasImportPath := analysis.resolveImportPathForAliasAtPosition(
				importAlias.Name,
				typedNode.Pos(),
			)
			if !hasImportPath {
				return true
			}

			existingImportPath, hasExistingImportAlias := requiredImports[importAlias.Name]
			if hasExistingImportAlias && existingImportPath != importPath {
				importCollectionErr = fmt.Errorf(
					"conflicting import alias %q for discovered route registrar generation: %q vs %q",
					importAlias.Name,
					existingImportPath,
					importPath,
				)
				return false
			}
			requiredImports[importAlias.Name] = importPath
			return true
		case *ast.Ident:
			if _, isSelectorExpressionIdentifier := selectorExpressionIdentifierSet[typedNode]; isSelectorExpressionIdentifier {
				return true
			}
			if typedNode.Obj != nil {
				return true
			}
			if analysis.isKnownPackageLevelIdentifierName(typedNode.Name) {
				return true
			}
			if isPredeclaredUnqualifiedIdentifierName(typedNode.Name) {
				return true
			}
			if !goTypesInitializationChecked {
				goTypesInitializationChecked = true
				if err := analysis.ensureGoTypesInfoInitialized(); err != nil {
					importCollectionErr = err
					return false
				}
			}
			if analysis.goTypesInfo == nil {
				return true
			}

			usedObject, hasUsedObject := analysis.goTypesInfo.Uses[typedNode]
			if !hasUsedObject || usedObject == nil {
				return true
			}
			usedObjectPackage := usedObject.Pkg()
			if usedObjectPackage == nil {
				return true
			}
			if analysis.goTypesPackagePath != "" && usedObjectPackage.Path() == analysis.goTypesPackagePath {
				return true
			}

			importCollectionErr = fmt.Errorf(
				"discovered registration expression references unqualified external symbol %q from %q; use explicit import alias qualification instead of dot-imported symbols",
				typedNode.Name,
				usedObjectPackage.Path(),
			)
			return false
		default:
			return true
		}
	})
	if importCollectionErr != nil {
		return importCollectionErr
	}
	return nil
}

func (analysis *backendRoutePackageAnalysis) resolveImportPathForAliasAtPosition(
	importAlias string,
	position token.Pos,
) (string, bool) {
	parsedServerFile, hasParsedServerFile := analysis.getParsedServerRouteFileForPosition(
		position,
	)
	if !hasParsedServerFile {
		return "", false
	}
	importPath, hasImportAlias := parsedServerFile.importAliases[importAlias]
	if !hasImportAlias {
		return "", false
	}
	return importPath, true
}

func (analysis *backendRoutePackageAnalysis) getParsedServerRouteFileForPosition(
	position token.Pos,
) (*parsedServerRouteFile, bool) {
	if position == token.NoPos {
		return nil, false
	}
	filePosition := analysis.goFileSet.Position(position)
	if filePosition.Filename == "" {
		return nil, false
	}
	filePath := filepath.ToSlash(filepath.Clean(filePosition.Filename))
	parsedServerFile, hasParsedServerFile := analysis.filesByPath[filePath]
	if !hasParsedServerFile {
		return nil, false
	}
	return parsedServerFile, true
}

type buildArtifactCleanupDependencies struct {
	walkDirForRemoval            func(string, filepath.WalkFunc) error
	removePathForCleanup         func(string) error
	readDirEntriesForCleanup     func(string) ([]os.DirEntry, error)
	removeTopLevelFileForCleanup func(string) error
	statStaticPublicOutDir       func(string) (fs.FileInfo, error)
}

type stageOnePathsWriteDependencies struct {
	marshalStageOnePathsFile         func(*runtimepaths.PathsFile) ([]byte, error)
	makeStageOnePathsOutputDirectory func(string, fs.FileMode) error
	writeStageOnePathsJSON           func(string, []byte, fs.FileMode) error
}

type buildArtifactFileSystemExecutorDependencies struct {
	buildArtifactCleanupDependencies buildArtifactCleanupDependencies
	stageOnePathsWriteDependencies   stageOnePathsWriteDependencies
}

type buildArtifactFileSystemExecutor struct {
	dependencies buildArtifactFileSystemExecutorDependencies
}

var defaultBuildArtifactFileSystemExecutor = newBuildArtifactFileSystemExecutor(
	buildArtifactFileSystemExecutorDependencies{},
)

func defaultBuildArtifactFileSystemExecutorDependencies() buildArtifactFileSystemExecutorDependencies {
	return buildArtifactFileSystemExecutorDependencies{
		buildArtifactCleanupDependencies: buildArtifactCleanupDependencies{
			walkDirForRemoval:            filepath.Walk,
			removePathForCleanup:         os.Remove,
			readDirEntriesForCleanup:     os.ReadDir,
			removeTopLevelFileForCleanup: os.Remove,
			statStaticPublicOutDir:       os.Stat,
		},
		stageOnePathsWriteDependencies: stageOnePathsWriteDependencies{
			marshalStageOnePathsFile: func(pathsFile *runtimepaths.PathsFile) ([]byte, error) {
				return json.MarshalIndent(pathsFile, "", "\t")
			},
			makeStageOnePathsOutputDirectory: os.MkdirAll,
			writeStageOnePathsJSON:           writeFileAtomically,
		},
	}
}

func normalizeBuildArtifactFileSystemExecutorDependencies(
	dependencies buildArtifactFileSystemExecutorDependencies,
) buildArtifactFileSystemExecutorDependencies {
	defaultDependencies := defaultBuildArtifactFileSystemExecutorDependencies()

	if dependencies.buildArtifactCleanupDependencies.walkDirForRemoval == nil {
		dependencies.buildArtifactCleanupDependencies.walkDirForRemoval = defaultDependencies.buildArtifactCleanupDependencies.walkDirForRemoval
	}
	if dependencies.buildArtifactCleanupDependencies.removePathForCleanup == nil {
		dependencies.buildArtifactCleanupDependencies.removePathForCleanup = defaultDependencies.buildArtifactCleanupDependencies.removePathForCleanup
	}
	if dependencies.buildArtifactCleanupDependencies.readDirEntriesForCleanup == nil {
		dependencies.buildArtifactCleanupDependencies.readDirEntriesForCleanup = defaultDependencies.buildArtifactCleanupDependencies.readDirEntriesForCleanup
	}
	if dependencies.buildArtifactCleanupDependencies.removeTopLevelFileForCleanup == nil {
		dependencies.buildArtifactCleanupDependencies.removeTopLevelFileForCleanup = defaultDependencies.buildArtifactCleanupDependencies.removeTopLevelFileForCleanup
	}
	if dependencies.buildArtifactCleanupDependencies.statStaticPublicOutDir == nil {
		dependencies.buildArtifactCleanupDependencies.statStaticPublicOutDir = defaultDependencies.buildArtifactCleanupDependencies.statStaticPublicOutDir
	}

	if dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile == nil {
		dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile = defaultDependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile
	}
	if dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory == nil {
		dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory = defaultDependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory
	}
	if dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON == nil {
		dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON = defaultDependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON
	}

	return dependencies
}

func newBuildArtifactFileSystemExecutor(
	dependencies buildArtifactFileSystemExecutorDependencies,
) buildArtifactFileSystemExecutor {
	return buildArtifactFileSystemExecutor{
		dependencies: normalizeBuildArtifactFileSystemExecutorDependencies(
			dependencies,
		),
	}
}

func cleanStaticPublicOutDir(v *vormaruntime.Vorma) error {
	return defaultBuildArtifactFileSystemExecutor.cleanStaticPublicOutDir(v)
}

func (executor buildArtifactFileSystemExecutor) cleanStaticPublicOutDir(
	v *vormaruntime.Vorma,
) error {
	staticPublicOutDir := v.Wave.StaticPublicOutDir()

	fileInfo, err := executor.dependencies.buildArtifactCleanupDependencies.statStaticPublicOutDir(
		staticPublicOutDir,
	)
	if err != nil {
		if os.IsNotExist(err) {
			v.Log.Warn(
				fmt.Sprintf(
					"static public out dir does not exist: %s",
					staticPublicOutDir,
				),
			)
			return nil
		}
		return err
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", staticPublicOutDir)
	}

	return executor.removeMatchingEntriesRecursively(
		staticPublicOutDir,
		shouldRemoveGeneratedStaticPublicFile,
	)
}

func shouldRemoveGeneratedStaticPublicFile(fileBaseName string) bool {
	return strings.HasPrefix(
		fileBaseName,
		vormaruntime.VormaVitePrehashedFilePrefix,
	) ||
		strings.HasPrefix(fileBaseName, vormaruntime.VormaRouteManifestPrefix)
}

func removeMatchingEntriesRecursively(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return defaultBuildArtifactFileSystemExecutor.removeMatchingEntriesRecursively(
		rootDir,
		shouldRemove,
	)
}

func (executor buildArtifactFileSystemExecutor) removeMatchingEntriesRecursively(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return executor.dependencies.buildArtifactCleanupDependencies.walkDirForRemoval(
		rootDir,
		func(path string, info fs.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info != nil && info.IsDir() {
				return nil
			}
			if !shouldRemove(filepath.Base(path)) {
				return nil
			}
			return executor.dependencies.buildArtifactCleanupDependencies.removePathForCleanup(
				path,
			)
		},
	)
}

func removeMatchingTopLevelFiles(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return defaultBuildArtifactFileSystemExecutor.removeMatchingTopLevelFiles(
		rootDir,
		shouldRemove,
	)
}

func (executor buildArtifactFileSystemExecutor) removeMatchingTopLevelFiles(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	entries, err := executor.dependencies.buildArtifactCleanupDependencies.readDirEntriesForCleanup(
		rootDir,
	)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !shouldRemove(name) {
			continue
		}
		if err := executor.dependencies.buildArtifactCleanupDependencies.removeTopLevelFileForCleanup(
			filepath.Join(rootDir, name),
		); err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}

	return nil
}

func writePathsToDiskStageOne(l *vormaruntime.LockedVorma) error {
	return defaultBuildArtifactFileSystemExecutor.writePathsToDiskStageOne(l)
}

func (executor buildArtifactFileSystemExecutor) writePathsToDiskStageOne(
	l *vormaruntime.LockedVorma,
) error {
	return executor.writePathsToDiskStageOneWithRouteManifest(
		l,
		l.RouteManifestFile(),
	)
}

func writePathsToDiskStageOneWithRouteManifest(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) error {
	return defaultBuildArtifactFileSystemExecutor.writePathsToDiskStageOneWithRouteManifest(
		l,
		routeManifestFile,
	)
}

func (executor buildArtifactFileSystemExecutor) writePathsToDiskStageOneWithRouteManifest(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) error {
	return executor.writeStageOnePathsFileToDisk(
		l.Vorma(),
		stageOnePathsFile(l, routeManifestFile),
	)
}

func writePathsToDiskStageOneFromRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
	routeManifestFile string,
) error {
	return defaultBuildArtifactFileSystemExecutor.writePathsToDiskStageOneFromRuntimeState(
		v,
		runtimeStateSnapshot,
		routeManifestFile,
	)
}

func (executor buildArtifactFileSystemExecutor) writePathsToDiskStageOneFromRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
	routeManifestFile string,
) error {
	return executor.writeStageOnePathsFileToDisk(
		v,
		stageOnePathsFileFromRuntimeState(
			v,
			runtimeStateSnapshot,
			routeManifestFile,
		),
	)
}

func (executor buildArtifactFileSystemExecutor) writeStageOnePathsFileToDisk(
	v *vormaruntime.Vorma,
	stageOnePathsFileData *runtimepaths.PathsFile,
) error {
	pathsJSONOut := pathsOutputPath(
		v,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)
	pathsAsJSON, err := executor.dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile(
		stageOnePathsFileData,
	)
	if err != nil {
		return fmt.Errorf("marshal stage-one paths file: %w", err)
	}
	return writePathsJSONBytesToOutputPath(
		pathsJSONOut,
		pathsAsJSON,
		pathsJSONWriteDependencies{
			makePathsOutputDirectory: executor.dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory,
			writePathsJSON:           executor.dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON,
		},
		"create stage-one paths output directory",
		"write stage-one paths JSON",
	)
}

func stageOnePathsFile(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) *runtimepaths.PathsFile {
	v := l.Vorma()
	return &runtimepaths.PathsFile{
		Stage:             "one",
		Paths:             toRuntimePathsMap(l.Paths()),
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           l.BuildID(),
		RouteManifestFile: routeManifestFile,
	}
}

func stageOnePathsFileFromRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
	routeManifestFile string,
) *runtimepaths.PathsFile {
	return &runtimepaths.PathsFile{
		Stage:             "one",
		Paths:             toRuntimePathsMap(runtimeStateSnapshot.paths),
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           runtimeStateSnapshot.buildID,
		RouteManifestFile: routeManifestFile,
	}
}

func toRuntimePathsMap(
	paths map[string]*vormaruntime.Path,
) map[string]*runtimepaths.RoutePath {
	if len(paths) == 0 {
		return map[string]*runtimepaths.RoutePath{}
	}

	convertedPaths := make(map[string]*runtimepaths.RoutePath, len(paths))
	for key, currentPath := range paths {
		if currentPath == nil {
			convertedPaths[key] = nil
			continue
		}
		convertedPaths[key] = &runtimepaths.RoutePath{
			OriginalPattern: currentPath.OriginalPattern,
			SrcPath:         currentPath.SrcPath,
			ExportKey:       currentPath.ExportKey,
			ErrorExportKey:  currentPath.ErrorExportKey,
			OutPath:         currentPath.OutPath,
			Deps:            append([]string(nil), currentPath.Deps...),
		}
	}
	return convertedPaths
}

type buildCommandOptions struct {
	runInDevelopmentMode  bool
	runHookOnly           bool
	skipGoBinaryBuildStep bool
}

type buildCommandHooks struct {
	configureBuildEnvironment func(*vormaruntime.Vorma)
	runBuildHook              func(*vormaruntime.Vorma, bool) error
	runProdHookPostProcessing func(*vormaruntime.Vorma) error
	runFullBuild              func(*vormaruntime.Vorma, bool, bool) error
}

type buildCommandExecutor struct {
	vorma *vormaruntime.Vorma
	hooks buildCommandHooks
}

func defaultBuildCommandHooks() buildCommandHooks {
	return buildCommandHooks{
		configureBuildEnvironment: func(v *vormaruntime.Vorma) {
			_ = configureBuildEnvironment(v)
		},
		runBuildHook: func(v *vormaruntime.Vorma, isDev bool) error {
			return buildInner(v, &buildInnerOptions{isDev: isDev})
		},
		runProdHookPostProcessing: runProdHookPostProcessing,
		runFullBuild:              runFullRuntimeBuild,
	}
}

func parseBuildCommandOptions(
	commandLineArgs []string,
) (buildCommandOptions, error) {
	var options buildCommandOptions

	flagSet := flag.NewFlagSet("vormabuild", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	flagSet.BoolVar(
		&options.runInDevelopmentMode,
		"dev",
		false,
		"run in development mode",
	)
	flagSet.BoolVar(
		&options.runHookOnly,
		"hook",
		false,
		"run build hook only (internal use)",
	)
	flagSet.BoolVar(
		&options.skipGoBinaryBuildStep,
		"no-binary",
		false,
		"skip go binary compilation",
	)
	if err := flagSet.Parse(commandLineArgs); err != nil {
		return buildCommandOptions{}, err
	}
	if remainingArgs := flagSet.Args(); len(remainingArgs) > 0 {
		return buildCommandOptions{}, fmt.Errorf(
			"unexpected positional arguments: %s",
			strings.Join(remainingArgs, ", "),
		)
	}

	return options, nil
}

func runBuildCommand(
	v *vormaruntime.Vorma,
	commandLineArgs []string,
	hooks buildCommandHooks,
) error {
	if v == nil {
		return errors.New("vorma runtime is required")
	}
	if err := validateBuildCommandHooks(hooks); err != nil {
		return err
	}

	options, err := parseBuildCommandOptions(commandLineArgs)
	if err != nil {
		return fmt.Errorf("parse build flags: %w", err)
	}

	commandExecutor := newBuildCommandExecutor(v, hooks)
	return commandExecutor.run(options)
}

func validateBuildCommandHooks(hooks buildCommandHooks) error {
	if hooks.configureBuildEnvironment == nil {
		return errors.New(
			"build command hook configureBuildEnvironment is required",
		)
	}
	if hooks.runBuildHook == nil {
		return errors.New("build command hook runBuildHook is required")
	}
	if hooks.runProdHookPostProcessing == nil {
		return errors.New(
			"build command hook runProdHookPostProcessing is required",
		)
	}
	if hooks.runFullBuild == nil {
		return errors.New("build command hook runFullBuild is required")
	}
	return nil
}

func newBuildCommandExecutor(
	v *vormaruntime.Vorma,
	hooks buildCommandHooks,
) buildCommandExecutor {
	return buildCommandExecutor{
		vorma: v,
		hooks: hooks,
	}
}

func (commandExecutor buildCommandExecutor) run(
	options buildCommandOptions,
) error {
	if options.runHookOnly {
		return commandExecutor.runHookOnly(options.runInDevelopmentMode)
	}

	return commandExecutor.runFullBuild(
		options.runInDevelopmentMode,
		options.skipGoBinaryBuildStep,
	)
}

func (commandExecutor buildCommandExecutor) runHookOnly(
	runInDevelopmentMode bool,
) error {
	commandExecutor.hooks.configureBuildEnvironment(commandExecutor.vorma)

	if err := commandExecutor.hooks.runBuildHook(commandExecutor.vorma, runInDevelopmentMode); err != nil {
		return fmt.Errorf("build hook failed: %w", err)
	}
	if runInDevelopmentMode {
		return nil
	}
	return commandExecutor.hooks.runProdHookPostProcessing(
		commandExecutor.vorma,
	)
}

func (commandExecutor buildCommandExecutor) runFullBuild(
	runInDevelopmentMode bool,
	skipGoBinaryBuildStep bool,
) error {
	if err := commandExecutor.hooks.runFullBuild(
		commandExecutor.vorma,
		runInDevelopmentMode,
		skipGoBinaryBuildStep,
	); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return nil
}

type frameworkBuildHookExecutionDependencies struct {
	prepareDiscoveredRouteRegistrarOverlayWithArtifactCache func(
		*vormaruntime.Vorma,
		*discoveredRouteRegistrarArtifactCache,
	) (*discoveredRouteRegistrarOverlay, error)
	runGoCommandWithContext func(context.Context, []string) error
}

type frameworkBuildHookExecutor struct {
	dependencies frameworkBuildHookExecutionDependencies
}

var defaultFrameworkBuildHookExecutor = newFrameworkBuildHookExecutor(
	frameworkBuildHookExecutionDependencies{},
)

func defaultFrameworkBuildHookExecutionDependencies() frameworkBuildHookExecutionDependencies {
	return frameworkBuildHookExecutionDependencies{
		prepareDiscoveredRouteRegistrarOverlayWithArtifactCache: prepareDiscoveredRouteRegistrarOverlayWithArtifactCache,
		runGoCommandWithContext: func(
			commandExecutionContext context.Context,
			goArguments []string,
		) error {
			if commandExecutionContext == nil {
				commandExecutionContext = context.Background()
			}

			goCommand := exec.CommandContext(
				commandExecutionContext,
				"go",
				goArguments...)
			goCommand.Stdout = os.Stdout
			goCommand.Stderr = os.Stderr
			return goCommand.Run()
		},
	}
}

func normalizeFrameworkBuildHookExecutionDependencies(
	dependencies frameworkBuildHookExecutionDependencies,
) frameworkBuildHookExecutionDependencies {
	defaultDependencies := defaultFrameworkBuildHookExecutionDependencies()
	if dependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache == nil {
		dependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache = defaultDependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache
	}
	if dependencies.runGoCommandWithContext == nil {
		dependencies.runGoCommandWithContext = defaultDependencies.runGoCommandWithContext
	}
	return dependencies
}

func newFrameworkBuildHookExecutor(
	dependencies frameworkBuildHookExecutionDependencies,
) frameworkBuildHookExecutor {
	return frameworkBuildHookExecutor{
		dependencies: normalizeFrameworkBuildHookExecutionDependencies(
			dependencies,
		),
	}
}

func registerVormaSchemaInConfig(cfg *wave.ParsedConfig) {
	if cfg == nil {
		return
	}
	if cfg.FrameworkSchemaExtensions == nil {
		cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	cfg.FrameworkSchemaExtensions["Vorma"] = vormaSchema
}

func injectFrameworkBuildHooksInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
) {
	if cfg == nil {
		return
	}
	if cfg.FrameworkDevBuildHook == "" {
		cfg.FrameworkDevBuildHook = fmt.Sprintf(
			"go run ./%s --dev --hook",
			v.Config.MainBuildEntry,
		)
	}
	if cfg.FrameworkProdBuildHook == "" {
		cfg.FrameworkProdBuildHook = fmt.Sprintf(
			"go run ./%s --hook",
			v.Config.MainBuildEntry,
		)
	}
}

func configureBuildEnvironment(v *vormaruntime.Vorma) *wave.ParsedConfig {
	return configureBuildEnvironmentInConfig(v, v.Wave.BuildtimeParsedConfig())
}

func configureBuildEnvironmentInConfig(
	v *vormaruntime.Vorma,
	cfg *wave.ParsedConfig,
) *wave.ParsedConfig {
	return configureBuildEnvironmentInConfigWithFrameworkBuildHookExecutor(
		v,
		cfg,
		defaultFrameworkBuildHookExecutor,
	)
}

func configureBuildEnvironmentInConfigWithFrameworkBuildHookExecutor(
	v *vormaruntime.Vorma,
	cfg *wave.ParsedConfig,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) *wave.ParsedConfig {
	if cfg == nil {
		return nil
	}

	discoveredRegistrarArtifactsCache := newDiscoveredRouteRegistrarArtifactCache(
		discoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
	)

	registerVormaSchemaInConfig(cfg)
	injectDefaultWatchPatternsInConfig(cfg, v)
	injectFrameworkBuildHooksInConfig(cfg, v)
	injectFrameworkBuildHookRunnerInConfig(
		cfg,
		v,
		discoveredRegistrarArtifactsCache,
		frameworkBuildHookExecutor,
	)
	injectFrameworkGoBuildOverlayPreparationInConfig(
		cfg,
		v,
		discoveredRegistrarArtifactsCache,
		frameworkBuildHookExecutor,
	)
	return cfg
}

func injectFrameworkBuildHookRunnerInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
	discoveredRegistrarArtifactsCache *discoveredRouteRegistrarArtifactCache,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) {
	if cfg == nil {
		return
	}
	if cfg.FrameworkRunBuildHook != nil {
		return
	}

	cfg.FrameworkRunBuildHook = func(
		commandExecutionContext context.Context,
		runInDevelopmentMode bool,
	) error {
		if v == nil {
			return errors.New("vorma runtime is required")
		}
		if v.Config == nil {
			return errors.New("vorma config is required")
		}
		mainBuildEntry := strings.TrimSpace(v.Config.MainBuildEntry)
		if mainBuildEntry == "" {
			return errors.New("vorma config MainBuildEntry is required")
		}
		if commandExecutionContext == nil {
			commandExecutionContext = context.Background()
		}

		goRunArgs := []string{"run"}
		discoveredRouteRegistrarOverlay, err := frameworkBuildHookExecutor.dependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
			v,
			discoveredRegistrarArtifactsCache,
		)
		if err != nil {
			return fmt.Errorf(
				"prepare discovered route registrar overlay for framework build hook: %w",
				err,
			)
		}
		if discoveredRouteRegistrarOverlay != nil &&
			strings.TrimSpace(
				discoveredRouteRegistrarOverlay.goOverlayConfigPath,
			) != "" {
			goRunArgs = append(
				goRunArgs,
				"-overlay="+discoveredRouteRegistrarOverlay.goOverlayConfigPath,
			)
		}

		trimmedMainBuildEntry := strings.TrimPrefix(mainBuildEntry, "./")
		goRunArgs = append(goRunArgs, "./"+trimmedMainBuildEntry)
		if runInDevelopmentMode {
			goRunArgs = append(goRunArgs, "--dev")
		}
		goRunArgs = append(goRunArgs, "--hook")

		runHookCommandErr := frameworkBuildHookExecutor.dependencies.runGoCommandWithContext(
			commandExecutionContext,
			goRunArgs,
		)
		var cleanupOverlayErr error
		if discoveredRouteRegistrarOverlay != nil {
			cleanupOverlayErr = discoveredRouteRegistrarOverlay.cleanup()
		}
		if runHookCommandErr != nil {
			if cleanupOverlayErr != nil {
				return fmt.Errorf(
					"run framework build hook command: %w (cleanup discovered route registrar overlay failed: %v)",
					runHookCommandErr,
					cleanupOverlayErr,
				)
			}
			return fmt.Errorf(
				"run framework build hook command: %w",
				runHookCommandErr,
			)
		}
		if cleanupOverlayErr != nil {
			return fmt.Errorf(
				"cleanup discovered route registrar overlay after framework build hook: %w",
				cleanupOverlayErr,
			)
		}
		return nil
	}
}

func injectFrameworkGoBuildOverlayPreparationInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
	discoveredRegistrarArtifactsCache *discoveredRouteRegistrarArtifactCache,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) {
	if cfg == nil {
		return
	}
	if cfg.FrameworkPrepareGoBuildOverlay != nil {
		return
	}

	cfg.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		discoveredRouteRegistrarOverlay, err := frameworkBuildHookExecutor.dependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
			v,
			discoveredRegistrarArtifactsCache,
		)
		if err != nil {
			return nil, err
		}
		if discoveredRouteRegistrarOverlay == nil {
			return nil, nil
		}

		return &wave.GoBuildOverlay{
			OverlayConfigPath: discoveredRouteRegistrarOverlay.goOverlayConfigPath,
			Cleanup: func() error {
				return discoveredRouteRegistrarOverlay.cleanup()
			},
		}, nil
	}
}

type buildInnerOptions struct {
	isDev bool
}

type buildInnerDependencies struct {
	captureBuildInnerRuntimeState             func(*vormaruntime.Vorma) buildInnerRuntimeStateSnapshot
	restoreBuildInnerRuntimeStateAfterFailure func(
		*vormaruntime.Vorma,
		buildInnerRuntimeStateSnapshot,
		string,
	) bool
	getCurrentBuildIDWithReadLock func(*vormaruntime.Vorma) string
	initializeBuildInnerState     func(*vormaruntime.Vorma, *buildInnerOptions) error
	parseAndSyncClientRoutes      func(*vormaruntime.Vorma) error
	cleanStaticPublicOutDir       func(*vormaruntime.Vorma) error
	writePublicFileMapTypeScript  func(*vormaruntime.Vorma) error
	writeRouteArtifacts           func(*vormaruntime.Vorma) error
	logBuildInnerCompletion       func(*vormaruntime.Vorma, time.Time)
}

type buildInnerExecutor struct {
	dependencies buildInnerDependencies
}

type buildInnerRuntimeStateSnapshot = buildRuntimeStateSnapshot

type buildInnerRouteSyncDependencies struct {
	parseClientRoutes                func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	parseBackendLoaderPatterns       func(*vormaruntime.Vorma) ([]string, error)
	mergeBackendLoaderPatternsInPath func(
		map[string]*vormaruntime.Path,
		[]string,
	) map[string]*vormaruntime.Path
	runRouteSyncExecution func(*vormaruntime.Vorma, routeSyncExecutionOptions) error
}

type buildInnerRouteSyncExecutor struct {
	dependencies buildInnerRouteSyncDependencies
}

type buildInnerBuildIDDependencies struct {
	generateDevBuildIDSuffix func() (string, error)
}

type buildInnerPublicFileMapDependencies struct {
	newPublicFileMapWriter func(*vormaruntime.Vorma) buildInnerPublicFileMapWriter
}

type buildInnerBuildIDExecutor struct {
	dependencies buildInnerBuildIDDependencies
}

type buildInnerPublicFileMapExecutor struct {
	dependencies buildInnerPublicFileMapDependencies
}

type buildInnerPublicFileMapWriter interface {
	WritePublicFileMapTS(string) error
	Close() error
}

func defaultBuildInnerDependencies() buildInnerDependencies {
	return buildInnerDependencies{
		captureBuildInnerRuntimeState:             captureBuildInnerRuntimeState,
		restoreBuildInnerRuntimeStateAfterFailure: restoreBuildInnerRuntimeStateAfterFailure,
		getCurrentBuildIDWithReadLock:             currentBuildIDWithReadLock,
		initializeBuildInnerState:                 initializeBuildInnerState,
		parseAndSyncClientRoutes:                  parseAndSyncClientRoutes,
		cleanStaticPublicOutDir:                   cleanStaticPublicOutDir,
		writePublicFileMapTypeScript:              writePublicFileMapTypeScript,
		writeRouteArtifacts:                       writeRouteArtifactsWithoutHoldingRuntimeLock,
		logBuildInnerCompletion:                   logBuildInnerCompletion,
	}
}

func normalizeBuildInnerDependencies(
	dependencies buildInnerDependencies,
) buildInnerDependencies {
	defaultDependencies := defaultBuildInnerDependencies()
	if dependencies.captureBuildInnerRuntimeState == nil {
		dependencies.captureBuildInnerRuntimeState = defaultDependencies.captureBuildInnerRuntimeState
	}
	if dependencies.restoreBuildInnerRuntimeStateAfterFailure == nil {
		dependencies.restoreBuildInnerRuntimeStateAfterFailure = defaultDependencies.restoreBuildInnerRuntimeStateAfterFailure
	}
	if dependencies.getCurrentBuildIDWithReadLock == nil {
		dependencies.getCurrentBuildIDWithReadLock = defaultDependencies.getCurrentBuildIDWithReadLock
	}
	if dependencies.initializeBuildInnerState == nil {
		dependencies.initializeBuildInnerState = defaultDependencies.initializeBuildInnerState
	}
	if dependencies.parseAndSyncClientRoutes == nil {
		dependencies.parseAndSyncClientRoutes = defaultDependencies.parseAndSyncClientRoutes
	}
	if dependencies.cleanStaticPublicOutDir == nil {
		dependencies.cleanStaticPublicOutDir = defaultDependencies.cleanStaticPublicOutDir
	}
	if dependencies.writePublicFileMapTypeScript == nil {
		dependencies.writePublicFileMapTypeScript = defaultDependencies.writePublicFileMapTypeScript
	}
	if dependencies.writeRouteArtifacts == nil {
		dependencies.writeRouteArtifacts = defaultDependencies.writeRouteArtifacts
	}
	if dependencies.logBuildInnerCompletion == nil {
		dependencies.logBuildInnerCompletion = defaultDependencies.logBuildInnerCompletion
	}
	return dependencies
}

func newBuildInnerExecutor(
	dependencies buildInnerDependencies,
) buildInnerExecutor {
	return buildInnerExecutor{
		dependencies: normalizeBuildInnerDependencies(dependencies),
	}
}

func defaultBuildInnerRouteSyncDependencies() buildInnerRouteSyncDependencies {
	return buildInnerRouteSyncDependencies{
		parseClientRoutes:                parseClientRoutes,
		parseBackendLoaderPatterns:       parseBackendLoaderPatterns,
		mergeBackendLoaderPatternsInPath: mergeBackendLoaderPatternsInPath,
		runRouteSyncExecution:            runRouteSyncExecution,
	}
}

func normalizeBuildInnerRouteSyncDependencies(
	dependencies buildInnerRouteSyncDependencies,
) buildInnerRouteSyncDependencies {
	defaultDependencies := defaultBuildInnerRouteSyncDependencies()
	if dependencies.parseClientRoutes == nil {
		dependencies.parseClientRoutes = defaultDependencies.parseClientRoutes
	}
	if dependencies.parseBackendLoaderPatterns == nil {
		dependencies.parseBackendLoaderPatterns = defaultDependencies.parseBackendLoaderPatterns
	}
	if dependencies.mergeBackendLoaderPatternsInPath == nil {
		dependencies.mergeBackendLoaderPatternsInPath = defaultDependencies.mergeBackendLoaderPatternsInPath
	}
	if dependencies.runRouteSyncExecution == nil {
		dependencies.runRouteSyncExecution = defaultDependencies.runRouteSyncExecution
	}
	return dependencies
}

func newBuildInnerRouteSyncExecutor(
	dependencies buildInnerRouteSyncDependencies,
) buildInnerRouteSyncExecutor {
	return buildInnerRouteSyncExecutor{
		dependencies: normalizeBuildInnerRouteSyncDependencies(dependencies),
	}
}

func defaultBuildInnerBuildIDDependencies() buildInnerBuildIDDependencies {
	return buildInnerBuildIDDependencies{
		generateDevBuildIDSuffix: func() (string, error) {
			return id.New(16)
		},
	}
}

func normalizeBuildInnerBuildIDDependencies(
	dependencies buildInnerBuildIDDependencies,
) buildInnerBuildIDDependencies {
	defaultDependencies := defaultBuildInnerBuildIDDependencies()
	if dependencies.generateDevBuildIDSuffix == nil {
		dependencies.generateDevBuildIDSuffix = defaultDependencies.generateDevBuildIDSuffix
	}
	return dependencies
}

func newBuildInnerBuildIDExecutor(
	dependencies buildInnerBuildIDDependencies,
) buildInnerBuildIDExecutor {
	return buildInnerBuildIDExecutor{
		dependencies: normalizeBuildInnerBuildIDDependencies(dependencies),
	}
}

func defaultBuildInnerPublicFileMapDependencies() buildInnerPublicFileMapDependencies {
	return buildInnerPublicFileMapDependencies{
		newPublicFileMapWriter: func(v *vormaruntime.Vorma) buildInnerPublicFileMapWriter {
			return builder.NewBuilder(
				configureBuildEnvironment(v),
				v.Wave.Logger(),
			)
		},
	}
}

func normalizeBuildInnerPublicFileMapDependencies(
	dependencies buildInnerPublicFileMapDependencies,
) buildInnerPublicFileMapDependencies {
	defaultDependencies := defaultBuildInnerPublicFileMapDependencies()
	if dependencies.newPublicFileMapWriter == nil {
		dependencies.newPublicFileMapWriter = defaultDependencies.newPublicFileMapWriter
	}
	return dependencies
}

func newBuildInnerPublicFileMapExecutor(
	dependencies buildInnerPublicFileMapDependencies,
) buildInnerPublicFileMapExecutor {
	return buildInnerPublicFileMapExecutor{
		dependencies: normalizeBuildInnerPublicFileMapDependencies(
			dependencies,
		),
	}
}

var defaultBuildInnerExecutor = newBuildInnerExecutor(
	buildInnerDependencies{},
)

func buildInner(v *vormaruntime.Vorma, opts *buildInnerOptions) error {
	return defaultBuildInnerExecutor.buildInner(v, opts)
}

func buildInnerWithDependencies(
	v *vormaruntime.Vorma,
	opts *buildInnerOptions,
	dependencies buildInnerDependencies,
) error {
	return newBuildInnerExecutor(dependencies).buildInner(v, opts)
}

func (executor buildInnerExecutor) buildInner(
	v *vormaruntime.Vorma,
	opts *buildInnerOptions,
) error {
	normalizedOptions := normalizeBuildInnerOptions(opts)
	requestedBuildMode := "prod"
	if normalizedOptions.isDev {
		requestedBuildMode = "dev"
	}

	start := time.Now()
	buildLifecycleStateMachine, err := newBuildLifecycleStateMachineWithOptions(
		buildLifecycleWorkflowFullBuild,
		v.Log,
		buildLifecycleStateMachineOptions{
			attemptInputs: []buildLifecycleAttemptInput{
				{
					Key:   "requested_mode",
					Value: requestedBuildMode,
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("configure build lifecycle state machine: %w", err)
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseStarted, "full build started"); err != nil {
		return fmt.Errorf("transition build lifecycle to started: %w", err)
	}

	initialRuntimeState := executor.dependencies.captureBuildInnerRuntimeState(
		v,
	)
	rollbackAttempted := false
	rollbackSkippedForSupersededBuildID := false
	currentAttemptCommittedBuildID := executor.dependencies.getCurrentBuildIDWithReadLock(
		v,
	)
	runBuildInnerStep := func(
		step func() error,
		stepFailureErrorContext string,
		transitionToPhase buildLifecyclePhase,
		transitionReason string,
		transitionFailureErrorContext string,
	) error {
		if err := step(); err != nil {
			if stepFailureErrorContext == "" {
				return err
			}
			return fmt.Errorf("%s: %w", stepFailureErrorContext, err)
		}
		if err := buildLifecycleStateMachine.transitionTo(transitionToPhase, transitionReason); err != nil {
			return fmt.Errorf("%s: %w", transitionFailureErrorContext, err)
		}
		return nil
	}
	buildErr := runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.initializeBuildInnerState(v, &normalizedOptions)
					},
					"",
					buildLifecyclePhaseRuntimeStateInitialized,
					"runtime state initialized",
					"transition build lifecycle to runtime-state-initialized",
				); err != nil {
					return err
				}
				currentAttemptCommittedBuildID = executor.dependencies.getCurrentBuildIDWithReadLock(
					v,
				)
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.parseAndSyncClientRoutes(v)
					},
					"parse client routes",
					buildLifecyclePhaseRoutesSynchronized,
					"client routes synchronized",
					"transition build lifecycle to routes-synchronized",
				); err != nil {
					return err
				}
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.cleanStaticPublicOutDir(v)
					},
					"clean static public out dir",
					buildLifecyclePhasePublicOutputCleaned,
					"static public output cleaned",
					"transition build lifecycle to public-output-cleaned",
				); err != nil {
					return err
				}
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.writePublicFileMapTypeScript(v)
					},
					"write public file map TS",
					buildLifecyclePhasePublicFileMapWritten,
					"public file map written",
					"transition build lifecycle to public-file-map-written",
				); err != nil {
					return err
				}
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.writeRouteArtifacts(v)
					},
					"write route artifacts",
					buildLifecyclePhaseRouteArtifactsWritten,
					"route artifacts written",
					"transition build lifecycle to route-artifacts-written",
				); err != nil {
					return err
				}
				return nil
			},
			rollbackOnFailure: func() error {
				rollbackAttempted = executor.dependencies.restoreBuildInnerRuntimeStateAfterFailure(
					v,
					initialRuntimeState,
					currentAttemptCommittedBuildID,
				)
				rollbackSkippedForSupersededBuildID = !rollbackAttempted
				return nil
			},
		},
	)
	if buildErr != nil {
		rollbackOutcome := buildLifecycleRollbackOutcomeNotAttempted
		rollbackReason := "runtime-state rollback was not attempted after full build failure"
		if rollbackAttempted {
			rollbackOutcome = buildLifecycleRollbackOutcomeSucceeded
			rollbackReason = "restored captured runtime state after full build failure"
		} else if rollbackSkippedForSupersededBuildID {
			rollbackReason = "skipped runtime-state rollback because build ID was superseded by a newer build"
		}
		if rollbackTraceErr := buildLifecycleStateMachine.recordRollback(
			buildLifecycleRollbackDecisionRequired,
			rollbackOutcome,
			rollbackReason,
			nil,
		); rollbackTraceErr != nil {
			buildErr = errors.Join(
				buildErr,
				fmt.Errorf(
					"record lifecycle rollback decision: %w",
					rollbackTraceErr,
				),
			)
		}
		if transitionErr := buildLifecycleStateMachine.transitionToFailed("full build failed", buildErr); transitionErr != nil {
			return errors.Join(
				buildErr,
				fmt.Errorf(
					"transition build lifecycle to failed: %w",
					transitionErr,
				),
			)
		}
		return buildErr
	}
	if rollbackTraceErr := buildLifecycleStateMachine.recordRollback(
		buildLifecycleRollbackDecisionNotRequired,
		buildLifecycleRollbackOutcomeNotRequired,
		"build completed successfully without requiring rollback",
		nil,
	); rollbackTraceErr != nil {
		return fmt.Errorf(
			"record lifecycle rollback decision: %w",
			rollbackTraceErr,
		)
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseCompleted, "full build completed"); err != nil {
		return fmt.Errorf("transition build lifecycle to completed: %w", err)
	}

	executor.dependencies.logBuildInnerCompletion(v, start)
	return nil
}

func normalizeBuildInnerOptions(opts *buildInnerOptions) buildInnerOptions {
	if opts == nil {
		return buildInnerOptions{}
	}
	return *opts
}

func captureBuildInnerRuntimeState(
	v *vormaruntime.Vorma,
) buildInnerRuntimeStateSnapshot {
	var runtimeStateSnapshot buildInnerRuntimeStateSnapshot
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		runtimeStateSnapshot = captureBuildRuntimeState(l)
	})
	return runtimeStateSnapshot
}

func restoreBuildInnerRuntimeStateAfterFailure(
	v *vormaruntime.Vorma,
	state buildInnerRuntimeStateSnapshot,
	currentAttemptCommittedBuildID string,
) bool {
	restored := false
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		if !shouldRollbackBuildInnerRuntimeStateAfterFailure(
			l.BuildID(),
			currentAttemptCommittedBuildID,
		) {
			return
		}
		restoreBuildRuntimeState(l, state)
		restored = true
	})
	return restored
}

func shouldRollbackBuildInnerRuntimeStateAfterFailure(
	currentBuildID string,
	currentAttemptCommittedBuildID string,
) bool {
	return shouldRestoreRuntimeStateSnapshotForAttemptBuildID(
		currentBuildID,
		currentAttemptCommittedBuildID,
	)
}

func initializeBuildInnerState(
	v *vormaruntime.Vorma,
	opts *buildInnerOptions,
) error {
	return initializeBuildInnerStateWithBuildIDDependencies(
		v,
		opts,
		buildInnerBuildIDDependencies{},
	)
}

func initializeBuildInnerStateWithBuildIDDependencies(
	v *vormaruntime.Vorma,
	opts *buildInnerOptions,
	dependencies buildInnerBuildIDDependencies,
) error {
	if !opts.isDev {
		commitRuntimeState(
			v,
			runtimeStateCommitInput{
				shouldCommitIsDev: true,
				isDev:             false,
			},
		)
		v.Log.Info("START building Vorma (PROD)")
		return nil
	}

	buildID, err := newDevBuildIDWithDependencies(dependencies)
	if err != nil {
		return err
	}

	commitRuntimeState(
		v,
		runtimeStateCommitInput{
			shouldCommitIsDev:   true,
			isDev:               true,
			shouldCommitBuildID: true,
			buildID:             buildID,
		},
	)
	v.Log.Info("START building Vorma (DEV)")
	return nil
}

func newDevBuildIDWithDependencies(
	dependencies buildInnerBuildIDDependencies,
) (string, error) {
	return newBuildInnerBuildIDExecutor(dependencies).newDevBuildID()
}

func (executor buildInnerBuildIDExecutor) newDevBuildID() (string, error) {
	return generateBuildIDWithPrefix(
		"dev_",
		executor.dependencies.generateDevBuildIDSuffix,
	)
}

func parseAndSyncClientRoutes(v *vormaruntime.Vorma) error {
	return parseAndSyncClientRoutesWithDependencies(
		v,
		buildInnerRouteSyncDependencies{},
	)
}

func parseAndSyncClientRoutesWithDependencies(
	v *vormaruntime.Vorma,
	dependencies buildInnerRouteSyncDependencies,
) error {
	return newBuildInnerRouteSyncExecutor(
		dependencies,
	).parseAndSyncClientRoutes(v)
}

func (executor buildInnerRouteSyncExecutor) parseAndSyncClientRoutes(
	v *vormaruntime.Vorma,
) error {
	return executor.dependencies.runRouteSyncExecution(
		v,
		routeSyncExecutionOptions{
			parseClientRoutes: executor.parseClientAndBackendRoutesForSync,
		},
	)
}

func parseClientAndBackendRoutesForSyncWithDependencies(
	v *vormaruntime.Vorma,
	dependencies buildInnerRouteSyncDependencies,
) (map[string]*vormaruntime.Path, error) {
	return newBuildInnerRouteSyncExecutor(
		dependencies,
	).parseClientAndBackendRoutesForSync(v)
}

func (executor buildInnerRouteSyncExecutor) parseClientAndBackendRoutesForSync(
	v *vormaruntime.Vorma,
) (map[string]*vormaruntime.Path, error) {
	clientPaths, err := executor.dependencies.parseClientRoutes(v)
	if err != nil {
		return nil, err
	}

	backendLoaderPatterns, err := executor.dependencies.parseBackendLoaderPatterns(
		v,
	)
	if err != nil {
		return nil, err
	}

	return executor.dependencies.mergeBackendLoaderPatternsInPath(
		clientPaths,
		backendLoaderPatterns,
	), nil
}

func mergeBackendLoaderPatternsInPath(
	clientPaths map[string]*vormaruntime.Path,
	backendLoaderPatterns []string,
) map[string]*vormaruntime.Path {
	if len(backendLoaderPatterns) == 0 {
		return clientPaths
	}

	if clientPaths == nil {
		clientPaths = map[string]*vormaruntime.Path{}
	}
	for _, backendLoaderPattern := range backendLoaderPatterns {
		if _, hasClientPath := clientPaths[backendLoaderPattern]; hasClientPath {
			continue
		}
		clientPaths[backendLoaderPattern] = &vormaruntime.Path{
			OriginalPattern: backendLoaderPattern,
			SrcPath:         "",
			ExportKey:       "default",
		}
	}
	return clientPaths
}

func writePublicFileMapTypeScript(v *vormaruntime.Vorma) error {
	return writePublicFileMapTypeScriptWithDependencies(
		v,
		buildInnerPublicFileMapDependencies{},
	)
}

func writePublicFileMapTypeScriptWithDependencies(
	v *vormaruntime.Vorma,
	dependencies buildInnerPublicFileMapDependencies,
) error {
	return newBuildInnerPublicFileMapExecutor(
		dependencies,
	).writePublicFileMapTypeScript(v)
}

func (executor buildInnerPublicFileMapExecutor) writePublicFileMapTypeScript(
	v *vormaruntime.Vorma,
) error {
	return executor.runWithPublicFileMapWriter(
		v,
		func(writer buildInnerPublicFileMapWriter) error {
			return writer.WritePublicFileMapTS(v.Config.TSGenOutDir)
		},
	)
}

func (executor buildInnerPublicFileMapExecutor) runWithPublicFileMapWriter(
	v *vormaruntime.Vorma,
	runWithWriter func(buildInnerPublicFileMapWriter) error,
) (operationErr error) {
	writer := executor.dependencies.newPublicFileMapWriter(v)
	return runWithClosableResource(
		writer,
		"close wave builder",
		runWithWriter,
	)
}

func logBuildInnerCompletion(v *vormaruntime.Vorma, start time.Time) {
	v.Log.Info("DONE building Vorma",
		"buildID", v.BuildID(),
		"routes found", len(v.Paths()),
		"duration", time.Since(start),
	)
}

type buildLifecycleWorkflow string

const (
	buildLifecycleWorkflowFullBuild        buildLifecycleWorkflow = "full-build"
	buildLifecycleWorkflowFastRouteRebuild buildLifecycleWorkflow = "fast-route-rebuild"
)

type buildLifecyclePhase string

const (
	buildLifecyclePhaseIdle                    buildLifecyclePhase = "idle"
	buildLifecyclePhaseStarted                 buildLifecyclePhase = "started"
	buildLifecyclePhaseRuntimeStateInitialized buildLifecyclePhase = "runtime-state-initialized"
	buildLifecyclePhaseRoutesSynchronized      buildLifecyclePhase = "routes-synchronized"
	buildLifecyclePhasePublicOutputCleaned     buildLifecyclePhase = "public-output-cleaned"
	buildLifecyclePhasePublicFileMapWritten    buildLifecyclePhase = "public-file-map-written"
	buildLifecyclePhaseRouteArtifactsWritten   buildLifecyclePhase = "route-artifacts-written"
	buildLifecyclePhaseCompleted               buildLifecyclePhase = "completed"
	buildLifecyclePhaseFailed                  buildLifecyclePhase = "failed"
)

type buildLifecycleTransitionRecord struct {
	AttemptID string
	AtUTC     string
	Sequence  uint64
	Workflow  buildLifecycleWorkflow
	From      buildLifecyclePhase
	To        buildLifecyclePhase
	Reason    string
	Error     string
}

type buildLifecycleTransitionObserver func(buildLifecycleTransitionRecord)

type buildLifecycleAttemptInput struct {
	Key   string
	Value string
}

type buildLifecycleRollbackDecision string

const (
	buildLifecycleRollbackDecisionRequired    buildLifecycleRollbackDecision = "required"
	buildLifecycleRollbackDecisionNotRequired buildLifecycleRollbackDecision = "not-required"
)

type buildLifecycleRollbackOutcome string

const (
	buildLifecycleRollbackOutcomeSucceeded    buildLifecycleRollbackOutcome = "succeeded"
	buildLifecycleRollbackOutcomeFailed       buildLifecycleRollbackOutcome = "failed"
	buildLifecycleRollbackOutcomeNotRequired  buildLifecycleRollbackOutcome = "not-required"
	buildLifecycleRollbackOutcomeNotAttempted buildLifecycleRollbackOutcome = "not-attempted"
)

type buildLifecycleRollbackRecord struct {
	AttemptID string
	AtUTC     string
	Sequence  uint64
	Workflow  buildLifecycleWorkflow
	Decision  buildLifecycleRollbackDecision
	Outcome   buildLifecycleRollbackOutcome
	Reason    string
	Error     string
}

type buildLifecycleStateMachineOptions struct {
	transitionObserver buildLifecycleTransitionObserver
	attemptInputs      []buildLifecycleAttemptInput
	dependencies       buildLifecycleStateMachineDependencies
}

type buildLifecycleStateMachineDependencies struct {
	nowUTC        func() time.Time
	nextAttemptID func(buildLifecycleWorkflow) string
}

var buildLifecycleTraceAttemptSequence atomic.Uint64

func defaultBuildLifecycleStateMachineDependencies() buildLifecycleStateMachineDependencies {
	return buildLifecycleStateMachineDependencies{
		nowUTC: time.Now().UTC,
		nextAttemptID: func(workflow buildLifecycleWorkflow) string {
			attemptSequence := buildLifecycleTraceAttemptSequence.Add(1)
			return fmt.Sprintf("%s-attempt-%d", workflow, attemptSequence)
		},
	}
}

func normalizeBuildLifecycleStateMachineDependencies(
	dependencies buildLifecycleStateMachineDependencies,
) buildLifecycleStateMachineDependencies {
	defaultDependencies := defaultBuildLifecycleStateMachineDependencies()
	if dependencies.nowUTC == nil {
		dependencies.nowUTC = defaultDependencies.nowUTC
	}
	if dependencies.nextAttemptID == nil {
		dependencies.nextAttemptID = defaultDependencies.nextAttemptID
	}
	return dependencies
}

type buildLifecycleStateMachine struct {
	workflow           buildLifecycleWorkflow
	dependencies       buildLifecycleStateMachineDependencies
	attemptID          string
	attemptInputs      []buildLifecycleAttemptInput
	currentPhase       buildLifecyclePhase
	transitionSeq      uint64
	rollbackSeq        uint64
	logger             *slog.Logger
	allowedTransitions map[buildLifecyclePhase]map[buildLifecyclePhase]struct{}
	transitionObserver buildLifecycleTransitionObserver
	transitionHistory  []buildLifecycleTransitionRecord
	rollbackHistory    []buildLifecycleRollbackRecord
}

func newBuildLifecycleStateMachine(
	workflow buildLifecycleWorkflow,
	logger *slog.Logger,
	transitionObserver buildLifecycleTransitionObserver,
) (*buildLifecycleStateMachine, error) {
	return newBuildLifecycleStateMachineWithOptions(
		workflow,
		logger,
		buildLifecycleStateMachineOptions{
			transitionObserver: transitionObserver,
		},
	)
}

func newBuildLifecycleStateMachineWithOptions(
	workflow buildLifecycleWorkflow,
	logger *slog.Logger,
	options buildLifecycleStateMachineOptions,
) (*buildLifecycleStateMachine, error) {
	allowedTransitions, err := buildLifecycleAllowedTransitions(workflow)
	if err != nil {
		return nil, err
	}

	dependencies := normalizeBuildLifecycleStateMachineDependencies(
		options.dependencies,
	)
	attemptInputs := normalizeBuildLifecycleAttemptInputs(options.attemptInputs)
	attemptID := strings.TrimSpace(dependencies.nextAttemptID(workflow))
	if attemptID == "" {
		return nil, errors.New("build lifecycle attempt ID is required")
	}

	return &buildLifecycleStateMachine{
		workflow:           workflow,
		dependencies:       dependencies,
		attemptID:          attemptID,
		attemptInputs:      attemptInputs,
		currentPhase:       buildLifecyclePhaseIdle,
		logger:             logger,
		allowedTransitions: allowedTransitions,
		transitionObserver: options.transitionObserver,
	}, nil
}

func (buildLifecycleMachine *buildLifecycleStateMachine) currentPhaseValue() buildLifecyclePhase {
	return buildLifecycleMachine.currentPhase
}

func (buildLifecycleMachine *buildLifecycleStateMachine) attemptIDValue() string {
	return buildLifecycleMachine.attemptID
}

func (buildLifecycleMachine *buildLifecycleStateMachine) attemptInputValues() []buildLifecycleAttemptInput {
	if len(buildLifecycleMachine.attemptInputs) == 0 {
		return nil
	}
	return append(
		[]buildLifecycleAttemptInput(nil),
		buildLifecycleMachine.attemptInputs...)
}

func (buildLifecycleMachine *buildLifecycleStateMachine) transitionHistoryEntries() []buildLifecycleTransitionRecord {
	if len(buildLifecycleMachine.transitionHistory) == 0 {
		return nil
	}
	return append(
		[]buildLifecycleTransitionRecord(nil),
		buildLifecycleMachine.transitionHistory...)
}

func (buildLifecycleMachine *buildLifecycleStateMachine) rollbackHistoryEntries() []buildLifecycleRollbackRecord {
	if len(buildLifecycleMachine.rollbackHistory) == 0 {
		return nil
	}
	return append(
		[]buildLifecycleRollbackRecord(nil),
		buildLifecycleMachine.rollbackHistory...)
}

func (buildLifecycleMachine *buildLifecycleStateMachine) transitionTo(
	nextPhase buildLifecyclePhase,
	reason string,
) error {
	return buildLifecycleMachine.transition(nextPhase, reason, "")
}

func (buildLifecycleMachine *buildLifecycleStateMachine) transitionToFailed(
	reason string,
	buildErr error,
) error {
	if buildErr == nil {
		return errors.New(
			"build error is required for failed lifecycle transition",
		)
	}
	return buildLifecycleMachine.transition(
		nextPhaseForFailedTransition(),
		reason,
		buildErr.Error(),
	)
}

func (buildLifecycleMachine *buildLifecycleStateMachine) recordRollback(
	decision buildLifecycleRollbackDecision,
	outcome buildLifecycleRollbackOutcome,
	reason string,
	rollbackErr error,
) error {
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return errors.New("rollback reason is required")
	}

	buildLifecycleMachine.rollbackSeq++
	rollbackRecord := buildLifecycleRollbackRecord{
		AttemptID: buildLifecycleMachine.attemptID,
		AtUTC: buildLifecycleMachine.dependencies.nowUTC().
			Format(time.RFC3339Nano),
		Sequence: buildLifecycleMachine.rollbackSeq,
		Workflow: buildLifecycleMachine.workflow,
		Decision: decision,
		Outcome:  outcome,
		Reason:   trimmedReason,
	}
	if rollbackErr != nil {
		rollbackRecord.Error = rollbackErr.Error()
	}

	buildLifecycleMachine.rollbackHistory = append(
		buildLifecycleMachine.rollbackHistory,
		rollbackRecord,
	)

	if buildLifecycleMachine.logger != nil {
		buildLifecycleMachine.logger.Debug(
			"Vorma build lifecycle rollback decision",
			"attempt_id",
			rollbackRecord.AttemptID,
			"at_utc",
			rollbackRecord.AtUTC,
			"seq",
			rollbackRecord.Sequence,
			"workflow",
			rollbackRecord.Workflow,
			"decision",
			rollbackRecord.Decision,
			"outcome",
			rollbackRecord.Outcome,
			"reason",
			rollbackRecord.Reason,
			"error",
			rollbackRecord.Error,
		)
	}

	return nil
}

func (buildLifecycleMachine *buildLifecycleStateMachine) transition(
	nextPhase buildLifecyclePhase,
	reason string,
	errorText string,
) error {
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return errors.New("lifecycle transition reason is required")
	}

	currentPhase := buildLifecycleMachine.currentPhase
	allowedNextPhases, hasPhase := buildLifecycleMachine.allowedTransitions[currentPhase]
	if !hasPhase {
		return fmt.Errorf(
			"workflow %q is terminal at phase %q and cannot transition to %q",
			buildLifecycleMachine.workflow,
			currentPhase,
			nextPhase,
		)
	}

	if _, transitionAllowed := allowedNextPhases[nextPhase]; !transitionAllowed {
		return fmt.Errorf(
			"invalid workflow %q lifecycle transition %q -> %q (allowed: %s)",
			buildLifecycleMachine.workflow,
			currentPhase,
			nextPhase,
			strings.Join(
				sortedBuildLifecyclePhaseNames(allowedNextPhases),
				", ",
			),
		)
	}

	buildLifecycleMachine.transitionSeq++
	buildLifecycleMachine.currentPhase = nextPhase

	transitionRecord := buildLifecycleTransitionRecord{
		AttemptID: buildLifecycleMachine.attemptID,
		AtUTC: buildLifecycleMachine.dependencies.nowUTC().
			Format(time.RFC3339Nano),
		Sequence: buildLifecycleMachine.transitionSeq,
		Workflow: buildLifecycleMachine.workflow,
		From:     currentPhase,
		To:       nextPhase,
		Reason:   trimmedReason,
		Error:    errorText,
	}

	buildLifecycleMachine.transitionHistory = append(
		buildLifecycleMachine.transitionHistory,
		transitionRecord,
	)

	if buildLifecycleMachine.logger != nil {
		buildLifecycleMachine.logger.Debug(
			"Vorma build lifecycle transition",
			"attempt_id",
			transitionRecord.AttemptID,
			"at_utc",
			transitionRecord.AtUTC,
			"seq",
			transitionRecord.Sequence,
			"workflow",
			transitionRecord.Workflow,
			"from",
			transitionRecord.From,
			"to",
			transitionRecord.To,
			"reason",
			transitionRecord.Reason,
			"error",
			transitionRecord.Error,
		)
	}
	if buildLifecycleMachine.transitionObserver != nil {
		buildLifecycleMachine.transitionObserver(transitionRecord)
	}
	return nil
}

func normalizeBuildLifecycleAttemptInputs(
	attemptInputs []buildLifecycleAttemptInput,
) []buildLifecycleAttemptInput {
	if len(attemptInputs) == 0 {
		return nil
	}

	normalizedAttemptInputs := make(
		[]buildLifecycleAttemptInput,
		0,
		len(attemptInputs),
	)
	for _, attemptInput := range attemptInputs {
		trimmedKey := strings.TrimSpace(attemptInput.Key)
		if trimmedKey == "" {
			continue
		}

		normalizedAttemptInputs = append(
			normalizedAttemptInputs,
			buildLifecycleAttemptInput{
				Key:   trimmedKey,
				Value: strings.TrimSpace(attemptInput.Value),
			},
		)
	}
	sort.Slice(normalizedAttemptInputs, func(i int, j int) bool {
		return normalizedAttemptInputs[i].Key < normalizedAttemptInputs[j].Key
	})
	return normalizedAttemptInputs
}

func buildLifecycleAllowedTransitions(
	workflow buildLifecycleWorkflow,
) (map[buildLifecyclePhase]map[buildLifecyclePhase]struct{}, error) {
	switch workflow {
	case buildLifecycleWorkflowFullBuild:
		return map[buildLifecyclePhase]map[buildLifecyclePhase]struct{}{
			buildLifecyclePhaseIdle: {
				buildLifecyclePhaseStarted: {},
			},
			buildLifecyclePhaseStarted: {
				buildLifecyclePhaseRuntimeStateInitialized: {},
				buildLifecyclePhaseFailed:                  {},
			},
			buildLifecyclePhaseRuntimeStateInitialized: {
				buildLifecyclePhaseRoutesSynchronized: {},
				buildLifecyclePhaseFailed:             {},
			},
			buildLifecyclePhaseRoutesSynchronized: {
				buildLifecyclePhasePublicOutputCleaned: {},
				buildLifecyclePhaseFailed:              {},
			},
			buildLifecyclePhasePublicOutputCleaned: {
				buildLifecyclePhasePublicFileMapWritten: {},
				buildLifecyclePhaseFailed:               {},
			},
			buildLifecyclePhasePublicFileMapWritten: {
				buildLifecyclePhaseRouteArtifactsWritten: {},
				buildLifecyclePhaseFailed:                {},
			},
			buildLifecyclePhaseRouteArtifactsWritten: {
				buildLifecyclePhaseCompleted: {},
				buildLifecyclePhaseFailed:    {},
			},
		}, nil
	case buildLifecycleWorkflowFastRouteRebuild:
		return map[buildLifecyclePhase]map[buildLifecyclePhase]struct{}{
			buildLifecyclePhaseIdle: {
				buildLifecyclePhaseStarted: {},
			},
			buildLifecyclePhaseStarted: {
				buildLifecyclePhaseRoutesSynchronized: {},
				buildLifecyclePhaseFailed:             {},
			},
			buildLifecyclePhaseRoutesSynchronized: {
				buildLifecyclePhaseRouteArtifactsWritten: {},
				buildLifecyclePhaseFailed:                {},
			},
			buildLifecyclePhaseRouteArtifactsWritten: {
				buildLifecyclePhaseCompleted: {},
				buildLifecyclePhaseFailed:    {},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown build lifecycle workflow %q", workflow)
	}
}

func nextPhaseForFailedTransition() buildLifecyclePhase {
	return buildLifecyclePhaseFailed
}

func sortedBuildLifecyclePhaseNames(
	allowedPhases map[buildLifecyclePhase]struct{},
) []string {
	phaseNames := make([]string, 0, len(allowedPhases))
	for allowedPhase := range allowedPhases {
		phaseNames = append(phaseNames, string(allowedPhase))
	}
	sort.Strings(phaseNames)
	return phaseNames
}

const (
	reloadTriggerRouteDefinitionsWatch = "route-definitions-watch"
	reloadTriggerHTMLTemplateWatch     = "html-template-watch"
)

type frameworkReloadActionResolver func(
	v *vormaruntime.Vorma,
	reloadEndpoint string,
	warnMessage string,
	reloadTrigger string,
	hookContext *wave.HookContext,
) *wave.RefreshAction

func injectDefaultWatchPatterns(v *vormaruntime.Vorma) *wave.ParsedConfig {
	cfg := v.Wave.BuildtimeParsedConfig()
	injectDefaultWatchPatternsInConfig(cfg, v)
	return cfg
}

func injectDefaultWatchPatternsInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
) {
	if !shouldInjectDefaultWatchPatterns(v) {
		return
	}
	if cfg == nil {
		return
	}

	patterns := getDefaultWatchPatterns(v)
	appendMissingFrameworkWatchPatterns(cfg, patterns)

	if v.Config.TSGenOutDir != "" {
		injectGeneratedOutputPathsForDefaultWatchPatterns(
			cfg,
			v.Config.TSGenOutDir,
		)
	}
}

func appendMissingFrameworkWatchPatterns(
	cfg *wave.ParsedConfig,
	defaultPatterns []wave.WatchedFile,
) {
	for _, defaultPattern := range defaultPatterns {
		if hasFrameworkWatchPattern(
			cfg.FrameworkWatchPatterns,
			defaultPattern.Pattern,
		) {
			continue
		}
		cfg.FrameworkWatchPatterns = append(
			cfg.FrameworkWatchPatterns,
			defaultPattern,
		)
	}
}

func hasFrameworkWatchPattern(
	existingPatterns []wave.WatchedFile,
	pattern string,
) bool {
	normalizedExpectedPattern := normalizeFrameworkWatchPatternPath(pattern)
	for _, existingPattern := range existingPatterns {
		if existingPattern.Pattern == pattern {
			return true
		}
		if normalizeFrameworkWatchPatternPath(existingPattern.Pattern) ==
			normalizedExpectedPattern {
			return true
		}
	}
	return false
}

func getDefaultWatchPatterns(v *vormaruntime.Vorma) []wave.WatchedFile {
	var patterns []wave.WatchedFile

	patterns = append(patterns, routeDefinitionWatchPatterns(v)...)

	htmlTemplatePattern := htmlTemplateWatchPattern(v)
	if htmlTemplatePattern != nil {
		patterns = append(patterns, *htmlTemplatePattern)
	}

	patterns = append(patterns, goFilesWatchPattern())

	return patterns
}

func routeDefinitionWatchPatterns(v *vormaruntime.Vorma) []wave.WatchedFile {
	normalizedRouteDefinitionPatterns, err := normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ClientRouteDefinitionPatterns,
	)
	if err != nil {
		panic(
			fmt.Sprintf("normalize client route definition patterns: %v", err),
		)
	}
	if len(normalizedRouteDefinitionPatterns) == 0 {
		return nil
	}

	onChangeCallback := routeDefinitionsOnChangeCallback(v)
	watchPatterns := make(
		[]wave.WatchedFile,
		0,
		len(normalizedRouteDefinitionPatterns),
	)
	for _, routeDefinitionPattern := range normalizedRouteDefinitionPatterns {
		normalizedWatchPattern := normalizeFrameworkWatchPatternPath(
			routeDefinitionPattern,
		)
		watchPatterns = append(
			watchPatterns,
			runOnChangeOnlyWatchPattern(
				normalizedWatchPattern,
				onChangeCallback,
				true,
			),
		)
	}
	return watchPatterns
}

func htmlTemplateWatchPattern(v *vormaruntime.Vorma) *wave.WatchedFile {
	htmlTemplateLocation := v.Config.HTMLTemplateLocation
	privateStaticDir := v.Wave.PrivateStaticDir()
	if htmlTemplateLocation == "" || privateStaticDir == "" {
		return nil
	}

	templatePath := normalizeFrameworkWatchPatternPath(
		filepath.Join(privateStaticDir, htmlTemplateLocation),
	)
	watchPattern := wave.WatchedFile{
		Pattern: templatePath,
		OnChangeHooks: []wave.OnChangeHook{{
			Timing:   wave.OnChangeStrategyPost,
			Callback: htmlTemplateOnChangeCallback(v),
		}},
	}
	return &watchPattern
}

func normalizeFrameworkWatchPatternPath(
	pathPattern string,
) string {
	if pathPattern == "" {
		return ""
	}

	cleanedPathPattern := filepath.Clean(pathPattern)
	if filepath.IsAbs(cleanedPathPattern) {
		return cleanedPathPattern
	}

	absolutePathPattern, absolutePathError := filepath.Abs(cleanedPathPattern)
	if absolutePathError != nil {
		return cleanedPathPattern
	}

	return filepath.Clean(absolutePathPattern)
}

func goFilesWatchPattern() wave.WatchedFile {
	return wave.WatchedFile{
		Pattern: "**/*.go",
		OnChangeHooks: []wave.OnChangeHook{{
			RunCombinedDevBuildHookCommands: true,
			Timing:                          wave.OnChangeStrategyConcurrent,
		}},
	}
}

func routeDefinitionsOnChangeCallback(
	v *vormaruntime.Vorma,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return routeDefinitionsOnChangeCallbackWithReloadActionResolver(
		v,
		getDeferredFrameworkRuntimeReloadAction,
	)
}

func routeDefinitionsOnChangeCallbackWithReloadActionResolver(
	v *vormaruntime.Vorma,
	resolveReloadAction frameworkReloadActionResolver,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return watchReloadCallback(
		v,
		v.DevReloadRoutesEndpointPath(),
		"route reload endpoint path is invalid, falling back to restart",
		reloadTriggerRouteDefinitionsWatch,
		rebuildRoutesOnly,
		resolveReloadAction,
	)
}

func htmlTemplateOnChangeCallback(
	v *vormaruntime.Vorma,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return htmlTemplateOnChangeCallbackWithReloadActionResolver(
		v,
		getDeferredFrameworkRuntimeReloadAction,
	)
}

func htmlTemplateOnChangeCallbackWithReloadActionResolver(
	v *vormaruntime.Vorma,
	resolveReloadAction frameworkReloadActionResolver,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return watchReloadCallback(
		v,
		v.DevReloadTemplateEndpointPath(),
		"template reload endpoint path is invalid, falling back to restart",
		reloadTriggerHTMLTemplateWatch,
		nil,
		resolveReloadAction,
	)
}

func watchReloadCallback(
	v *vormaruntime.Vorma,
	reloadEndpoint string,
	reloadEndpointFailureWarnMessage string,
	reloadTrigger string,
	preReloadAction func(*vormaruntime.Vorma) error,
	resolveReloadAction frameworkReloadActionResolver,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	if resolveReloadAction == nil {
		resolveReloadAction = getDeferredFrameworkRuntimeReloadAction
	}

	return func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
		if preReloadAction != nil {
			if err := preReloadAction(v); err != nil {
				return nil, fmt.Errorf(
					"run pre-reload action for trigger %q: %w",
					reloadTrigger,
					err,
				)
			}
		}

		if ctx != nil && ctx.AppStoppedForBatch {
			if v.Log != nil {
				v.Log.Debug(
					"watch reload callback skipped",
					"reload_endpoint",
					reloadEndpoint,
					"reload_trigger",
					reloadTrigger,
					"skip_reason",
					"app-stopped-for-batch",
				)
			}
			return nil, nil
		}

		return resolveReloadAction(
			v,
			reloadEndpoint,
			reloadEndpointFailureWarnMessage,
			reloadTrigger,
			ctx,
		), nil
	}
}

func shouldInjectDefaultWatchPatterns(v *vormaruntime.Vorma) bool {
	if v.Config.IncludeDefaults == nil {
		return true
	}
	return *v.Config.IncludeDefaults
}

func injectGeneratedOutputPathsForDefaultWatchPatterns(
	cfg *wave.ParsedConfig,
	tsGenOutDir string,
) {
	if cfg.FrameworkPublicFileMapOutDir == "" {
		cfg.FrameworkPublicFileMapOutDir = tsGenOutDir
	}

	appendMissingFrameworkIgnoredPatterns(cfg,
		filepath.Join(tsGenOutDir, wave.GeneratedTSFileName),
		filepath.Join(tsGenOutDir, wave.PublicFileMapTSName),
		filepath.Join(tsGenOutDir, wave.PublicFileMapJSONName),
	)
}

func appendMissingFrameworkIgnoredPatterns(
	cfg *wave.ParsedConfig,
	defaultIgnoredPatterns ...string,
) {
	for _, defaultIgnoredPattern := range defaultIgnoredPatterns {
		if hasString(cfg.FrameworkIgnoredPatterns, defaultIgnoredPattern) {
			continue
		}
		cfg.FrameworkIgnoredPatterns = append(
			cfg.FrameworkIgnoredPatterns,
			defaultIgnoredPattern,
		)
	}
}

func hasString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func runOnChangeOnlyWatchPattern(
	pattern string,
	callback func(*wave.HookContext) (*wave.RefreshAction, error),
	skipRebuildingNotification bool,
) wave.WatchedFile {
	return wave.WatchedFile{
		Pattern:         pattern,
		RunOnChangeOnly: true,
		OnChangeHooks: []wave.OnChangeHook{{
			Callback: callback,
		}},
		SkipRebuildingNotification: skipRebuildingNotification,
	}
}

var vormaSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: "Vorma framework configuration.",
	RequiredChildren: []string{
		"MainBuildEntry",
		"UIVariant",
		"HTMLTemplateLocation",
		"ClientEntry",
		"ClientRouteDefinitionPatterns",
		"TSGenOutDir",
	},
	Properties: struct {
		IncludeDefaults               jsonschema.Entry
		MainBuildEntry                jsonschema.Entry
		UIVariant                     jsonschema.Entry
		HTMLTemplateLocation          jsonschema.Entry
		ClientEntry                   jsonschema.Entry
		ClientRouteDefinitionPatterns jsonschema.Entry
		ServerRouteDefinitionPatterns jsonschema.Entry
		TSGenOutDir                   jsonschema.Entry
		BuildtimePublicURLFuncName    jsonschema.Entry
		UnresolvedRoutePolicy         jsonschema.Entry
		DevReloadRoutesEndpointPath   jsonschema.Entry
		DevReloadTemplateEndpointPath jsonschema.Entry
		TemplateDataKeyHeadElements   jsonschema.Entry
		TemplateDataKeyBodyScripts    jsonschema.Entry
		TemplateDataKeySSRScript      jsonschema.Entry
		TemplateDataKeySSRScriptHash  jsonschema.Entry
		TemplateDataKeyRootElementID  jsonschema.Entry
		ClientRootElementID           jsonschema.Entry
	}{
		IncludeDefaults:               includeDefaultsSchema,
		MainBuildEntry:                mainBuildEntrySchema,
		UIVariant:                     uiVariantSchema,
		HTMLTemplateLocation:          htmlTemplateLocationSchema,
		ClientEntry:                   clientEntrySchema,
		ClientRouteDefinitionPatterns: clientRouteDefinitionPatternsSchema,
		ServerRouteDefinitionPatterns: serverRouteDefinitionPatternsSchema,
		TSGenOutDir:                   tsGenOutDirSchema,
		BuildtimePublicURLFuncName:    buildtimePublicURLFuncNameSchema,
		UnresolvedRoutePolicy:         unresolvedRoutePolicySchema,
		DevReloadRoutesEndpointPath:   devReloadRoutesEndpointPathSchema,
		DevReloadTemplateEndpointPath: devReloadTemplateEndpointPathSchema,
		TemplateDataKeyHeadElements:   templateDataKeyHeadElementsSchema,
		TemplateDataKeyBodyScripts:    templateDataKeyBodyScriptsSchema,
		TemplateDataKeySSRScript:      templateDataKeySSRScriptSchema,
		TemplateDataKeySSRScriptHash:  templateDataKeySSRScriptHashSchema,
		TemplateDataKeyRootElementID:  templateDataKeyRootElementIDSchema,
		ClientRootElementID:           clientRootElementIDSchema,
	},
})

var includeDefaultsSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true (default), Vorma injects default watch patterns for routes, templates, and Go files.`,
	Default:     true,
})

var mainBuildEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to the Vorma build command entry point.`,
	Examples:    []string{"backend/cmd/build", "cmd/build"},
})

var uiVariantSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `The UI framework to use for client-side rendering.`,
	Enum:        []string{"react", "preact", "solid"},
})

var htmlTemplateLocationSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your HTML template file, relative to the private static directory.`,
	Examples:    []string{"entry.go.html"},
})

var clientEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your client-side entry file.`,
	Examples:    []string{"frontend/src/vorma.entry.tsx"},
})

var clientRouteDefinitionPatternsSchema = jsonschema.RequiredArray(
	jsonschema.Def{
		Description: `Glob patterns that resolve to client route definition files.`,
		Items:       jsonschema.RequiredString(jsonschema.Def{}),
		Examples: []string{
			"frontend/src/**/*vorma.routes.ts",
			"frontend/src/routes/core.vorma.routes.ts",
		},
	},
)

var serverRouteDefinitionPatternsSchema = jsonschema.OptionalArray(
	jsonschema.Def{
		Description: `Optional backend route definition patterns to merge into the route manifest for server-only handlers.`,
		Items:       jsonschema.OptionalString(jsonschema.Def{}),
		Examples:    []string{"backend/src/**/*vorma.routes.go"},
	},
)

var tsGenOutDirSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Directory where Vorma generates TypeScript route artifacts and filemap outputs.`,
	Examples:    []string{"frontend/src/vorma.gen"},
})

var buildtimePublicURLFuncNameSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Name of the global function injected by the Vite plugin for resolving public asset URLs at build time.`,
	Default:     "waveBuildtimeURL",
	Examples:    []string{"waveBuildtimeURL", "getAssetURL"},
})

var unresolvedRoutePolicySchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `How unresolved route module expressions are handled. Defaults to "warn" in dev and "error" in production.`,
	Enum:        []string{"warn", "error"},
	Examples:    []string{"warn", "error"},
})

var devReloadRoutesEndpointPathSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Dev-only endpoint path that triggers in-process route reload.`,
		Default:     "/__vorma_internal/reload-routes",
		Examples:    []string{"/__vorma_internal/reload-routes"},
	},
)

var devReloadTemplateEndpointPathSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Dev-only endpoint path that triggers in-process template reload.`,
		Default:     "/__vorma_internal/reload-template",
		Examples:    []string{"/__vorma_internal/reload-template"},
	},
)

var templateDataKeyHeadElementsSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Template data key for serialized head elements.`,
		Default:     "VormaHeadEls",
		Examples:    []string{"VormaHeadEls"},
	},
)

var templateDataKeyBodyScriptsSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Template data key for serialized body script tags.`,
	Default:     "VormaBodyScripts",
	Examples:    []string{"VormaBodyScripts"},
})

var templateDataKeySSRScriptSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Template data key for the SSR bootstrap script.`,
	Default:     "VormaSSRScript",
	Examples:    []string{"VormaSSRScript"},
})

var templateDataKeySSRScriptHashSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Template data key for the SSR script CSP hash.`,
		Default:     "VormaSSRScriptSha256Hash",
		Examples:    []string{"VormaSSRScriptSha256Hash"},
	},
)

var templateDataKeyRootElementIDSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Template data key for the root element ID placeholder.`,
		Default:     "VormaRootID",
		Examples:    []string{"VormaRootID"},
	},
)

var clientRootElementIDSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Client root element ID used for hydration/mount.`,
	Default:     "vorma-root",
	Examples:    []string{"vorma-root"},
})

type fastRouteRebuildDependencies struct {
	parseClientRoutes             func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	newFastRebuildID              func() (string, error)
	runRouteSyncExecution         func(*vormaruntime.Vorma, routeSyncExecutionOptions) error
	logFastRouteRebuildCompletion func(*vormaruntime.Vorma, time.Time)
}

type fastRouteRebuildArtifactDependencies struct {
	cleanRouteManifestsOnly       func(*vormaruntime.Vorma) error
	writeRouteArtifacts           func(*vormaruntime.Vorma) error
	readRouteManifestArtifact     func(string) ([]byte, error)
	writeRouteManifestArtifact    func(string, []byte, os.FileMode) error
	removeRouteManifestArtifact   func(string) error
	getCurrentBuildIDWithReadLock func(*vormaruntime.Vorma) string
}

type fastRouteRebuildExecutor struct {
	dependencies     fastRouteRebuildDependencies
	artifactExecutor fastRouteRebuildArtifactExecutor
}

type fastRouteRebuildArtifactExecutor struct {
	dependencies fastRouteRebuildArtifactDependencies
}

type fastRouteRebuildBuildIDDependencies struct {
	generateFastRebuildIDSuffix func() (string, error)
}

type fastRouteRebuildIDExecutor struct {
	dependencies fastRouteRebuildBuildIDDependencies
}

func defaultFastRouteRebuildDependencies() fastRouteRebuildDependencies {
	return fastRouteRebuildDependencies{
		parseClientRoutes:             parseClientRoutes,
		newFastRebuildID:              newFastRebuildID,
		runRouteSyncExecution:         runRouteSyncExecution,
		logFastRouteRebuildCompletion: logFastRouteRebuildCompletion,
	}
}

func normalizeFastRouteRebuildDependencies(
	dependencies fastRouteRebuildDependencies,
) fastRouteRebuildDependencies {
	defaultDependencies := defaultFastRouteRebuildDependencies()
	if dependencies.parseClientRoutes == nil {
		dependencies.parseClientRoutes = defaultDependencies.parseClientRoutes
	}
	if dependencies.newFastRebuildID == nil {
		dependencies.newFastRebuildID = defaultDependencies.newFastRebuildID
	}
	if dependencies.runRouteSyncExecution == nil {
		dependencies.runRouteSyncExecution = defaultDependencies.runRouteSyncExecution
	}
	if dependencies.logFastRouteRebuildCompletion == nil {
		dependencies.logFastRouteRebuildCompletion = defaultDependencies.logFastRouteRebuildCompletion
	}
	return dependencies
}

func defaultFastRouteRebuildArtifactDependencies() fastRouteRebuildArtifactDependencies {
	return fastRouteRebuildArtifactDependencies{
		cleanRouteManifestsOnly:       cleanRouteManifestsOnly,
		writeRouteArtifacts:           writeRouteArtifactsWithoutHoldingRuntimeLock,
		readRouteManifestArtifact:     os.ReadFile,
		writeRouteManifestArtifact:    writeFileAtomically,
		removeRouteManifestArtifact:   os.Remove,
		getCurrentBuildIDWithReadLock: currentBuildIDWithReadLock,
	}
}

func normalizeFastRouteRebuildArtifactDependencies(
	dependencies fastRouteRebuildArtifactDependencies,
) fastRouteRebuildArtifactDependencies {
	defaultDependencies := defaultFastRouteRebuildArtifactDependencies()
	if dependencies.cleanRouteManifestsOnly == nil {
		dependencies.cleanRouteManifestsOnly = defaultDependencies.cleanRouteManifestsOnly
	}
	if dependencies.writeRouteArtifacts == nil {
		dependencies.writeRouteArtifacts = defaultDependencies.writeRouteArtifacts
	}
	if dependencies.readRouteManifestArtifact == nil {
		dependencies.readRouteManifestArtifact = defaultDependencies.readRouteManifestArtifact
	}
	if dependencies.writeRouteManifestArtifact == nil {
		dependencies.writeRouteManifestArtifact = defaultDependencies.writeRouteManifestArtifact
	}
	if dependencies.removeRouteManifestArtifact == nil {
		dependencies.removeRouteManifestArtifact = defaultDependencies.removeRouteManifestArtifact
	}
	if dependencies.getCurrentBuildIDWithReadLock == nil {
		dependencies.getCurrentBuildIDWithReadLock = defaultDependencies.getCurrentBuildIDWithReadLock
	}
	return dependencies
}

func newFastRouteRebuildArtifactExecutor(
	dependencies fastRouteRebuildArtifactDependencies,
) fastRouteRebuildArtifactExecutor {
	return fastRouteRebuildArtifactExecutor{
		dependencies: normalizeFastRouteRebuildArtifactDependencies(
			dependencies,
		),
	}
}

func newFastRouteRebuildExecutor(
	dependencies fastRouteRebuildDependencies,
	artifactDependencies fastRouteRebuildArtifactDependencies,
) fastRouteRebuildExecutor {
	return fastRouteRebuildExecutor{
		dependencies: normalizeFastRouteRebuildDependencies(dependencies),
		artifactExecutor: newFastRouteRebuildArtifactExecutor(
			artifactDependencies,
		),
	}
}

var defaultFastRouteRebuildIDExecutor = newFastRouteRebuildIDExecutor(
	fastRouteRebuildBuildIDDependencies{},
)

var defaultFastRouteRebuildExecutor = newFastRouteRebuildExecutor(
	fastRouteRebuildDependencies{},
	fastRouteRebuildArtifactDependencies{},
)

func defaultFastRouteRebuildBuildIDDependencies() fastRouteRebuildBuildIDDependencies {
	return fastRouteRebuildBuildIDDependencies{
		generateFastRebuildIDSuffix: func() (string, error) {
			return id.New(16)
		},
	}
}

func normalizeFastRouteRebuildBuildIDDependencies(
	dependencies fastRouteRebuildBuildIDDependencies,
) fastRouteRebuildBuildIDDependencies {
	defaultDependencies := defaultFastRouteRebuildBuildIDDependencies()
	if dependencies.generateFastRebuildIDSuffix == nil {
		dependencies.generateFastRebuildIDSuffix = defaultDependencies.generateFastRebuildIDSuffix
	}
	return dependencies
}

func newFastRouteRebuildIDExecutor(
	dependencies fastRouteRebuildBuildIDDependencies,
) fastRouteRebuildIDExecutor {
	return fastRouteRebuildIDExecutor{
		dependencies: normalizeFastRouteRebuildBuildIDDependencies(
			dependencies,
		),
	}
}

// rebuildRoutesOnly is the fast path for rebuilding when only vorma.routes.ts changes.
// Runs in Process A (Dev Server), which has handlers registered for type reflection.
//
// Flow:
//  1. Parse client routes with esbuild
//  2. Generate TypeScript using live reflection (Process A has handlers)
//  3. Write all artifacts to disk
//  4. Wave calls Process B's reload endpoint to sync from disk
//
// Performance: ~50ms vs ~1.5s for full rebuild
func rebuildRoutesOnly(v *vormaruntime.Vorma) error {
	return defaultFastRouteRebuildExecutor.rebuildRoutesOnly(v)
}

func rebuildRoutesOnlyWithDependencies(
	v *vormaruntime.Vorma,
	dependencies fastRouteRebuildDependencies,
	artifactDependencies fastRouteRebuildArtifactDependencies,
) error {
	return newFastRouteRebuildExecutor(
		dependencies,
		artifactDependencies,
	).rebuildRoutesOnly(v)
}

func (executor fastRouteRebuildExecutor) rebuildRoutesOnly(
	v *vormaruntime.Vorma,
) error {
	start := time.Now()

	if !v.IsDevMode() {
		return errors.New("rebuildRoutesOnly should only be called in dev mode")
	}

	buildLifecycleStateMachine, err := newBuildLifecycleStateMachineWithOptions(
		buildLifecycleWorkflowFastRouteRebuild,
		v.Log,
		buildLifecycleStateMachineOptions{
			attemptInputs: []buildLifecycleAttemptInput{
				{
					Key:   "is_dev_mode",
					Value: "true",
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("configure build lifecycle state machine: %w", err)
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseStarted, "fast route rebuild started"); err != nil {
		return fmt.Errorf("transition build lifecycle to started: %w", err)
	}

	v.Log.Info("START fast route rebuild")

	err = executor.dependencies.runRouteSyncExecution(
		v,
		routeSyncExecutionOptions{
			parseClientRoutes:          executor.dependencies.parseClientRoutes,
			generateBuildID:            executor.dependencies.newFastRebuildID,
			parseClientRoutesErrorText: "parse client routes",
			postSyncHook: func(v *vormaruntime.Vorma) error {
				if err := buildLifecycleStateMachine.transitionTo(
					buildLifecyclePhaseRoutesSynchronized,
					"client routes synchronized",
				); err != nil {
					return fmt.Errorf(
						"transition build lifecycle to routes-synchronized: %w",
						err,
					)
				}
				if err := executor.artifactExecutor.writeFastRebuildArtifactsAfterRouteSync(v); err != nil {
					return err
				}
				if err := buildLifecycleStateMachine.transitionTo(
					buildLifecyclePhaseRouteArtifactsWritten,
					"route artifacts written",
				); err != nil {
					return fmt.Errorf(
						"transition build lifecycle to route-artifacts-written: %w",
						err,
					)
				}
				return nil
			},
		},
	)
	if err != nil {
		if rollbackTraceErr := buildLifecycleStateMachine.recordRollback(
			buildLifecycleRollbackDecisionRequired,
			buildLifecycleRollbackOutcomeNotAttempted,
			"fast rebuild failure path requires route-manifest rollback in artifact writer",
			nil,
		); rollbackTraceErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf(
					"record lifecycle rollback decision: %w",
					rollbackTraceErr,
				),
			)
		}
		if transitionErr := buildLifecycleStateMachine.transitionToFailed("fast route rebuild failed", err); transitionErr != nil {
			return errors.Join(
				err,
				fmt.Errorf(
					"transition build lifecycle to failed: %w",
					transitionErr,
				),
			)
		}
		return err
	}
	if rollbackTraceErr := buildLifecycleStateMachine.recordRollback(
		buildLifecycleRollbackDecisionNotRequired,
		buildLifecycleRollbackOutcomeNotRequired,
		"fast route rebuild completed successfully without requiring rollback",
		nil,
	); rollbackTraceErr != nil {
		return fmt.Errorf(
			"record lifecycle rollback decision: %w",
			rollbackTraceErr,
		)
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseCompleted, "fast route rebuild completed"); err != nil {
		return fmt.Errorf("transition build lifecycle to completed: %w", err)
	}

	executor.dependencies.logFastRouteRebuildCompletion(v, start)
	return nil
}

func newFastRebuildID() (string, error) {
	return defaultFastRouteRebuildIDExecutor.newFastRebuildID()
}

func (executor fastRouteRebuildIDExecutor) newFastRebuildID() (string, error) {
	return generateBuildIDWithPrefix(
		"dev_fast_",
		executor.dependencies.generateFastRebuildIDSuffix,
	)
}

func writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
	v *vormaruntime.Vorma,
	dependencies fastRouteRebuildArtifactDependencies,
) error {
	return newFastRouteRebuildArtifactExecutor(
		dependencies,
	).writeFastRebuildArtifactsAfterRouteSync(v)
}

func (executor fastRouteRebuildArtifactExecutor) writeFastRebuildArtifactsAfterRouteSync(
	v *vormaruntime.Vorma,
) error {
	expectedBuildID := executor.dependencies.getCurrentBuildIDWithReadLock(v)

	var previousRouteManifestFile string
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		previousRouteManifestFile = l.RouteManifestFile()
	})
	previousRouteManifestSnapshot, err := executor.captureFastRebuildRouteManifestArtifact(
		v,
		previousRouteManifestFile,
	)
	if err != nil {
		return fmt.Errorf("snapshot current route manifest artifact: %w", err)
	}

	skipRollbackForSupersededBuildID := false
	return runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if !shouldRunFastRebuildArtifactWriteForBuildID(
					executor.dependencies.getCurrentBuildIDWithReadLock(v),
					expectedBuildID,
				) {
					skipRollbackForSupersededBuildID = true
					return nil
				}

				if err := executor.dependencies.cleanRouteManifestsOnly(v); err != nil {
					return fmt.Errorf("clean route manifests: %w", err)
				}

				if !shouldRunFastRebuildArtifactWriteForBuildID(
					executor.dependencies.getCurrentBuildIDWithReadLock(v),
					expectedBuildID,
				) {
					skipRollbackForSupersededBuildID = true
					return nil
				}

				return executor.dependencies.writeRouteArtifacts(v)
			},
			rollbackOnFailure: func() error {
				if skipRollbackForSupersededBuildID {
					return nil
				}
				if !shouldRestoreFastRebuildManifestSnapshotForBuildID(
					executor.dependencies.getCurrentBuildIDWithReadLock(v),
					expectedBuildID,
				) {
					return nil
				}
				return executor.restoreFastRebuildRouteManifestArtifact(
					v,
					previousRouteManifestFile,
					previousRouteManifestSnapshot,
				)
			},
			rollbackErrorContext: "restore route manifest artifact",
			logRollbackFailureAfterPanic: func(rollbackErr error) {
				if v.Log != nil {
					v.Log.Error(
						"restore route manifest artifact after panic failed",
						"error",
						rollbackErr,
					)
				}
			},
		},
	)
}

func shouldRunFastRebuildArtifactWriteForBuildID(
	currentBuildID string,
	expectedBuildID string,
) bool {
	return currentBuildID == expectedBuildID
}

func shouldRestoreFastRebuildManifestSnapshotForBuildID(
	currentBuildID string,
	expectedBuildID string,
) bool {
	return currentBuildID == expectedBuildID
}

type fastRebuildRouteManifestArtifactSnapshot = buildArtifactFileSnapshot

func captureFastRebuildRouteManifestArtifactSnapshotWithDependencies(
	v *vormaruntime.Vorma,
	manifestFile string,
	dependencies fastRouteRebuildArtifactDependencies,
) (fastRebuildRouteManifestArtifactSnapshot, error) {
	return newFastRouteRebuildArtifactExecutor(
		dependencies,
	).captureFastRebuildRouteManifestArtifact(v, manifestFile)
}

func (executor fastRouteRebuildArtifactExecutor) captureFastRebuildRouteManifestArtifact(
	v *vormaruntime.Vorma,
	manifestFile string,
) (fastRebuildRouteManifestArtifactSnapshot, error) {
	if manifestFile == "" {
		return fastRebuildRouteManifestArtifactSnapshot{}, nil
	}

	manifestFilePath := filepath.Join(v.Wave.StaticPublicOutDir(), manifestFile)
	return captureBuildArtifactFile(
		manifestFilePath,
		executor.dependencies.readRouteManifestArtifact,
	)
}

func restoreFastRebuildRouteManifestArtifactSnapshotWithDependencies(
	v *vormaruntime.Vorma,
	manifestFile string,
	snapshot fastRebuildRouteManifestArtifactSnapshot,
	dependencies fastRouteRebuildArtifactDependencies,
) error {
	return newFastRouteRebuildArtifactExecutor(
		dependencies,
	).restoreFastRebuildRouteManifestArtifact(v, manifestFile, snapshot)
}

func (executor fastRouteRebuildArtifactExecutor) restoreFastRebuildRouteManifestArtifact(
	v *vormaruntime.Vorma,
	manifestFile string,
	snapshot fastRebuildRouteManifestArtifactSnapshot,
) error {
	if manifestFile == "" {
		return nil
	}

	manifestFilePath := filepath.Join(v.Wave.StaticPublicOutDir(), manifestFile)
	return restoreBuildArtifactFile(
		manifestFilePath,
		snapshot,
		executor.dependencies.writeRouteManifestArtifact,
		executor.dependencies.removeRouteManifestArtifact,
	)
}

func logFastRouteRebuildCompletion(v *vormaruntime.Vorma, start time.Time) {
	v.Log.Info("DONE fast route rebuild",
		"buildID", v.BuildID(),
		"routes", len(v.Paths()),
		"duration", time.Since(start),
	)
}

func cleanRouteManifestsOnly(v *vormaruntime.Vorma) error {
	staticPublicOutDir := v.Wave.StaticPublicOutDir()
	err := removeMatchingTopLevelFiles(
		staticPublicOutDir,
		isGeneratedRouteManifestFilename,
	)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func isGeneratedRouteManifestFilename(fileName string) bool {
	return len(fileName) > len(vormaruntime.VormaRouteManifestPrefix) &&
		strings.HasPrefix(fileName, vormaruntime.VormaRouteManifestPrefix)
}

type reloadActionDependencies struct {
	nextReloadAttemptID func() string
}

type reloadActionExecutor struct {
	dependencies reloadActionDependencies
}

var reloadAttemptSequence atomic.Uint64

var defaultReloadActionExecutor = newReloadActionExecutor(
	reloadActionDependencies{
		nextReloadAttemptID: nextReloadAttemptID,
	},
)

func nextReloadAttemptID() string {
	attemptSequence := reloadAttemptSequence.Add(1)
	return fmt.Sprintf("reload-%d", attemptSequence)
}

func normalizeReloadActionDependencies(
	dependencies reloadActionDependencies,
) reloadActionDependencies {
	if dependencies.nextReloadAttemptID == nil {
		dependencies.nextReloadAttemptID = nextReloadAttemptID
	}
	return dependencies
}

func newReloadActionExecutor(
	dependencies reloadActionDependencies,
) reloadActionExecutor {
	return reloadActionExecutor{
		dependencies: normalizeReloadActionDependencies(dependencies),
	}
}

func getDeferredFrameworkRuntimeReloadAction(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
	reloadTrigger string,
	hookContext *wave.HookContext,
) *wave.RefreshAction {
	return defaultReloadActionExecutor.getDeferredFrameworkRuntimeReloadAction(
		v,
		endpoint,
		warnMessage,
		reloadTrigger,
		hookContext,
	)
}

func (executor reloadActionExecutor) getDeferredFrameworkRuntimeReloadAction(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
	reloadTrigger string,
	_ *wave.HookContext,
) *wave.RefreshAction {
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		if v != nil && v.Log != nil {
			v.Log.Warn(
				warnMessage,
				"error",
				errors.New("reload endpoint path is required"),
				"fallback_action",
				"restart-without-recompile",
				"fallback_reason",
				"reload-endpoint-path-missing",
			)
		}
		return newRestartWithoutRecompileAction()
	}
	if !strings.HasPrefix(trimmedEndpoint, "/") {
		trimmedEndpoint = "/" + trimmedEndpoint
	}
	expectedBuildID := ""
	if v != nil {
		expectedBuildID = strings.TrimSpace(v.BuildID())
	}

	reloadAction := newReloadBrowserAndWaitAction()
	reloadAction.FrameworkRuntimeReloadRequest = &wave.FrameworkRuntimeReloadRequest{
		EndpointPath:    trimmedEndpoint,
		ReloadAttemptID: executor.dependencies.nextReloadAttemptID(),
		ExpectedBuildID: expectedBuildID,
		ReloadTrigger:   strings.TrimSpace(reloadTrigger),
	}
	return reloadAction
}

func newReloadBrowserAndWaitAction() *wave.RefreshAction {
	return &wave.RefreshAction{
		ReloadBrowser: true,
		WaitForApp:    true,
		WaitForVite:   true,
	}
}

func newRestartWithoutRecompileAction() *wave.RefreshAction {
	return &wave.RefreshAction{
		TriggerRestart: true,
		RecompileGo:    false,
	}
}

type rollbackTransactionOptions struct {
	run                          func() error
	rollbackOnFailure            func() error
	rollbackErrorContext         string
	logRollbackFailureAfterPanic func(error)
}

func runWithRollbackOnFailureAndPanic(
	options rollbackTransactionOptions,
) (operationErr error) {
	if options.run == nil {
		return errors.New("rollback transaction run step is required")
	}

	defer func() {
		recoveredPanicValue := recover()
		if recoveredPanicValue == nil {
			return
		}

		rollbackPanicValue, rollbackErr := runRollbackIfConfigured(
			options.rollbackOnFailure,
		)
		if rollbackErr != nil && options.logRollbackFailureAfterPanic != nil {
			options.logRollbackFailureAfterPanic(rollbackErr)
		}
		if rollbackPanicValue != nil &&
			options.logRollbackFailureAfterPanic != nil {
			options.logRollbackFailureAfterPanic(
				fmt.Errorf("rollback panic: %v", rollbackPanicValue),
			)
		}

		panic(recoveredPanicValue)
	}()

	operationErr = options.run()
	if operationErr == nil {
		return nil
	}

	rollbackPanicValue, rollbackErr := runRollbackIfConfigured(
		options.rollbackOnFailure,
	)
	if rollbackPanicValue != nil {
		panic(rollbackPanicValue)
	}
	if rollbackErr == nil {
		return operationErr
	}

	if options.rollbackErrorContext != "" {
		rollbackErr = fmt.Errorf(
			"%s: %w",
			options.rollbackErrorContext,
			rollbackErr,
		)
	}

	return errors.Join(operationErr, rollbackErr)
}

func runRollbackIfConfigured(
	rollbackStep func() error,
) (rollbackPanicValue any, rollbackErr error) {
	if rollbackStep == nil {
		return nil, nil
	}

	defer func() {
		recoveredPanicValue := recover()
		if recoveredPanicValue != nil {
			rollbackPanicValue = recoveredPanicValue
		}
	}()

	rollbackErr = rollbackStep()
	return nil, rollbackErr
}

type routeCall struct {
	Pattern  string
	Module   string
	Key      string
	ErrorKey string
}

// unresolvedRouteCall represents a route() call where the module path could not
// be statically determined. This happens when the module argument is a variable,
// function call, or other dynamic expression.
type unresolvedRouteCall struct {
	Pattern       string
	RawModuleExpr string
	Reason        string
}

type routeCallVisitor struct {
	routeFuncNames    map[string]bool
	trackedModuleVars map[string]string
	routes            []routeCall
	unresolvedRoutes  []unresolvedRouteCall
}

func (rv *routeCallVisitor) Enter(n js.INode) js.IVisitor {
	call, isCall := n.(*js.CallExpr)
	if !isCall {
		return rv
	}

	ident, isIdent := call.X.(*js.Var)
	if !isIdent {
		return rv
	}

	if _, isRouteFunc := rv.routeFuncNames[string(ident.Data)]; !isRouteFunc {
		return rv
	}

	routeCall, unresolvedRoute := rv.extractRouteCall(call.Args.List)
	if unresolvedRoute != nil {
		rv.unresolvedRoutes = append(rv.unresolvedRoutes, *unresolvedRoute)
		return rv
	}
	if routeCall == nil {
		return rv
	}

	rv.routes = append(rv.routes, *routeCall)
	return rv
}

func (rv *routeCallVisitor) Exit(n js.INode) {
	_ = n
}

func (rv *routeCallVisitor) extractRouteCall(
	argsList []js.Arg,
) (*routeCall, *unresolvedRouteCall) {
	route := routeCall{Key: "default"}

	pattern, ok := extractStaticStringArg(argsList, 0)
	if !ok {
		return nil, nil
	}
	route.Pattern = pattern

	if len(argsList) > 1 {
		modulePath, unresolvedModule := rv.resolveModuleArgument(
			route.Pattern,
			argsList[1].Value,
		)
		if unresolvedModule != nil {
			return nil, unresolvedModule
		}
		route.Module = modulePath
	}

	if key, ok := extractStaticStringArg(argsList, 2); ok {
		route.Key = key
	}
	if errorKey, ok := extractStaticStringArg(argsList, 3); ok {
		route.ErrorKey = errorKey
	}

	return &route, nil
}

func (rv *routeCallVisitor) resolveModuleArgument(
	routePattern string,
	moduleExpr js.IExpr,
) (string, *unresolvedRouteCall) {
	if varRef, ok := moduleExpr.(*js.Var); ok {
		varName := string(varRef.Data)
		if trackedModulePath, exists := rv.trackedModuleVars[varName]; exists {
			return trackedModulePath, nil
		}
		return "", unresolvedModuleArgument(
			routePattern,
			varName,
			fmt.Sprintf(
				"variable '%s' is not a tracked import or const string",
				varName,
			),
		)
	}

	if functionCall, ok := moduleExpr.(*js.CallExpr); ok {
		return resolveModuleArgumentFromFunctionCall(routePattern, functionCall)
	}

	modulePath, ok := extractStaticStringLiteral(moduleExpr)
	if !ok {
		return "", unresolvedModuleArgument(
			routePattern,
			"<expression>",
			"module argument is not a static string, variable, or import() call",
		)
	}
	return modulePath, nil
}

func resolveModuleArgumentFromFunctionCall(
	routePattern string,
	functionCall *js.CallExpr,
) (string, *unresolvedRouteCall) {
	if functionCallTargetsJSImport(functionCall) {
		if modulePath, ok := extractStaticStringArg(functionCall.Args.List, 0); ok {
			return modulePath, nil
		}
		return "", unresolvedModuleArgument(
			routePattern,
			"import(...)",
			"dynamic import() argument is not a static string",
		)
	}

	functionName := "<unknown>"
	if functionIdent, ok := functionCall.X.(*js.Var); ok {
		functionName = string(functionIdent.Data)
	}
	return "", unresolvedModuleArgument(
		routePattern,
		functionName+"(...)",
		"module argument is a function call, which cannot be statically analyzed",
	)
}

func unresolvedModuleArgument(
	routePattern string,
	rawModuleExpr string,
	reason string,
) *unresolvedRouteCall {
	return &unresolvedRouteCall{
		Pattern:       routePattern,
		RawModuleExpr: rawModuleExpr,
		Reason:        reason,
	}
}

func functionCallTargetsJSImport(functionCall *js.CallExpr) bool {
	functionIdent, ok := functionCall.X.(*js.Var)
	return ok && string(functionIdent.Data) == "import"
}

func extractStaticStringArg(args []js.Arg, idx int) (string, bool) {
	if idx >= len(args) {
		return "", false
	}
	return extractStaticStringLiteral(args[idx].Value)
}

func extractStaticStringLiteral(expr js.IExpr) (string, bool) {
	strLit, ok := expr.(*js.LiteralExpr)
	if !ok || strLit.TokenType != js.StringToken {
		return "", false
	}
	unquoted, err := strconv.Unquote(string(strLit.Data))
	if err != nil {
		return "", false
	}
	return unquoted, true
}

func extractRouteCalls(
	code string,
) ([]routeCall, []unresolvedRouteCall, error) {
	parsedAST, err := js.Parse(parse.NewInputString(code), js.Options{})
	if err != nil {
		return nil, nil, fmt.Errorf("parse JS/TS: %w", err)
	}

	metadata := collectRouteParsingMetadata(parsedAST)

	visitor := &routeCallVisitor{
		routeFuncNames:    metadata.routeFuncNames,
		trackedModuleVars: metadata.trackedModuleVars,
	}
	js.Walk(visitor, parsedAST)

	return visitor.routes, visitor.unresolvedRoutes, nil
}

var importRegex = regexp.MustCompile(
	`import\((` + "`" + `[^` + "`" + `]+` + "`" + `|'[^']+'|"[^"]+")\)`,
)

type routeParsingExecutorDependencies struct {
	resolveClientRouteDefinitionFiles    func(*vormaruntime.Vorma) ([]string, error)
	parseRouteDefinitionFileIntoCalls    func(*vormaruntime.Vorma, string) (parsedRouteDefinitionsCode, error)
	handleUnresolvedRouteCalls           func(*vormaruntime.Vorma, string, []unresolvedRouteCall) error
	mergeRouteCallsIntoPaths             func(*vormaruntime.Vorma, map[string]*vormaruntime.Path, string, []routeCall) error
	transformRouteDefinitionsCode        func(*vormaruntime.Vorma, []byte) (string, error)
	extractRouteCallsFromTransformedCode func(string) ([]routeCall, []unresolvedRouteCall, error)
	computeRelativeModulePath            func(string, string) (string, error)
	statRouteModulePath                  func(string) (fs.FileInfo, error)
	expandRouteDefinitionPattern         func(string) ([]string, error)
	statRouteDefinitionPath              func(string) (fs.FileInfo, error)
	readRouteDefinitionFile              func(string) ([]byte, error)
}

type routeParsingExecutor struct {
	dependencies routeParsingExecutorDependencies
}

var defaultRouteParsingExecutor = newRouteParsingExecutor(
	routeParsingExecutorDependencies{},
)

func defaultRouteParsingExecutorDependencies() routeParsingExecutorDependencies {
	return routeParsingExecutorDependencies{
		transformRouteDefinitionsCode:        transformRouteDefinitionsCode,
		extractRouteCallsFromTransformedCode: extractRouteCalls,
		computeRelativeModulePath:            filepath.Rel,
		statRouteModulePath:                  os.Stat,
		expandRouteDefinitionPattern:         expandRouteDefinitionPatternWithDoublestar,
		statRouteDefinitionPath:              os.Stat,
		handleUnresolvedRouteCalls:           handleUnresolvedRouteCalls,
		readRouteDefinitionFile:              os.ReadFile,
	}
}

func withDefaultRouteParsingExecutorDependencies(
	dependencies routeParsingExecutorDependencies,
) routeParsingExecutorDependencies {
	defaultDependencies := defaultRouteParsingExecutorDependencies()

	if dependencies.transformRouteDefinitionsCode == nil {
		dependencies.transformRouteDefinitionsCode = defaultDependencies.transformRouteDefinitionsCode
	}
	if dependencies.extractRouteCallsFromTransformedCode == nil {
		dependencies.extractRouteCallsFromTransformedCode = defaultDependencies.extractRouteCallsFromTransformedCode
	}

	if dependencies.computeRelativeModulePath == nil {
		dependencies.computeRelativeModulePath = defaultDependencies.computeRelativeModulePath
	}
	if dependencies.statRouteModulePath == nil {
		dependencies.statRouteModulePath = defaultDependencies.statRouteModulePath
	}

	if dependencies.expandRouteDefinitionPattern == nil {
		dependencies.expandRouteDefinitionPattern = defaultDependencies.expandRouteDefinitionPattern
	}
	if dependencies.statRouteDefinitionPath == nil {
		dependencies.statRouteDefinitionPath = defaultDependencies.statRouteDefinitionPath
	}

	if dependencies.handleUnresolvedRouteCalls == nil {
		dependencies.handleUnresolvedRouteCalls = handleUnresolvedRouteCalls
	}

	if dependencies.readRouteDefinitionFile == nil {
		dependencies.readRouteDefinitionFile = defaultDependencies.readRouteDefinitionFile
	}

	return dependencies
}

func newRouteParsingExecutor(
	dependencies routeParsingExecutorDependencies,
) routeParsingExecutor {
	return routeParsingExecutor{
		dependencies: withDefaultRouteParsingExecutorDependencies(dependencies),
	}
}

func (executor routeParsingExecutor) resolveClientRouteDefinitionFilesStep() func(*vormaruntime.Vorma) ([]string, error) {
	if executor.dependencies.resolveClientRouteDefinitionFiles != nil {
		return executor.dependencies.resolveClientRouteDefinitionFiles
	}
	return executor.resolveClientRouteDefinitionFiles
}

func (executor routeParsingExecutor) parseRouteDefinitionFileIntoCallsStep() func(*vormaruntime.Vorma, string) (parsedRouteDefinitionsCode, error) {
	if executor.dependencies.parseRouteDefinitionFileIntoCalls != nil {
		return executor.dependencies.parseRouteDefinitionFileIntoCalls
	}
	return executor.parseRouteDefinitionFileIntoCalls
}

func (executor routeParsingExecutor) handleUnresolvedRouteCallsStep() func(*vormaruntime.Vorma, string, []unresolvedRouteCall) error {
	return executor.dependencies.handleUnresolvedRouteCalls
}

func (executor routeParsingExecutor) mergeRouteCallsIntoPathsStep() func(*vormaruntime.Vorma, map[string]*vormaruntime.Path, string, []routeCall) error {
	if executor.dependencies.mergeRouteCallsIntoPaths != nil {
		return executor.dependencies.mergeRouteCallsIntoPaths
	}
	return executor.mergeRouteCallsIntoPaths
}

type parsedRouteDefinitionsCode struct {
	routeCalls       []routeCall
	unresolvedRoutes []unresolvedRouteCall
}

func parseClientRoutes(
	v *vormaruntime.Vorma,
) (map[string]*vormaruntime.Path, error) {
	return defaultRouteParsingExecutor.parseClientRoutes(v)
}

func (executor routeParsingExecutor) parseClientRoutes(
	v *vormaruntime.Vorma,
) (map[string]*vormaruntime.Path, error) {
	resolveRouteDefinitionFiles := executor.resolveClientRouteDefinitionFilesStep()
	parseRouteDefinitionFile := executor.parseRouteDefinitionFileIntoCallsStep()
	handleUnresolvedRoutes := executor.handleUnresolvedRouteCallsStep()
	mergeRouteCalls := executor.mergeRouteCallsIntoPathsStep()

	routeDefinitionFiles, err := resolveRouteDefinitionFiles(v)
	if err != nil {
		return nil, err
	}

	paths := make(map[string]*vormaruntime.Path)
	for _, routeDefinitionFile := range routeDefinitionFiles {
		parsedRouteDefinitions, err := parseRouteDefinitionFile(
			v,
			routeDefinitionFile,
		)
		if err != nil {
			return nil, err
		}

		if err := handleUnresolvedRoutes(
			v,
			routeDefinitionFile,
			parsedRouteDefinitions.unresolvedRoutes,
		); err != nil {
			return nil, err
		}

		if err := mergeRouteCalls(
			v,
			paths,
			routeDefinitionFile,
			parsedRouteDefinitions.routeCalls,
		); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func resolveClientRouteDefinitionFiles(
	v *vormaruntime.Vorma,
) ([]string, error) {
	return defaultRouteParsingExecutor.resolveClientRouteDefinitionFiles(v)
}

func (executor routeParsingExecutor) resolveClientRouteDefinitionFiles(
	v *vormaruntime.Vorma,
) ([]string, error) {
	if v == nil {
		return nil, errors.New("vorma runtime is required")
	}
	if v.Config == nil {
		return nil, errors.New("vorma config is required")
	}
	if len(v.Config.ClientRouteDefinitionPatterns) == 0 {
		return nil, errors.New(
			"Vorma.ClientRouteDefinitionPatterns is required",
		)
	}

	normalizedRouteDefinitionPatterns, err := normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ClientRouteDefinitionPatterns,
	)
	if err != nil {
		return nil, err
	}

	matchedFilesByPath := make(map[string]struct{})
	for _, routeDefinitionPattern := range normalizedRouteDefinitionPatterns {
		if patternContainsGlobMeta(routeDefinitionPattern) {
			routeDefinitionMatches, err := executor.dependencies.expandRouteDefinitionPattern(
				routeDefinitionPattern,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"expand route definition pattern %q: %w",
					routeDefinitionPattern,
					err,
				)
			}
			for _, routeDefinitionMatch := range routeDefinitionMatches {
				routeDefinitionInfo, err := executor.dependencies.statRouteDefinitionPath(
					routeDefinitionMatch,
				)
				if err != nil {
					return nil, fmt.Errorf(
						"stat route definition path %q: %w",
						routeDefinitionMatch,
						err,
					)
				}
				if routeDefinitionInfo.IsDir() {
					continue
				}
				matchedFilesByPath[filepath.Clean(routeDefinitionMatch)] = struct{}{}
			}
			continue
		}

		routeDefinitionInfo, err := executor.dependencies.statRouteDefinitionPath(
			routeDefinitionPattern,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"stat route definition path %q: %w",
				routeDefinitionPattern,
				err,
			)
		}
		if routeDefinitionInfo.IsDir() {
			return nil, fmt.Errorf(
				"route definition path %q is a directory",
				routeDefinitionPattern,
			)
		}
		matchedFilesByPath[filepath.Clean(routeDefinitionPattern)] = struct{}{}
	}

	if len(matchedFilesByPath) == 0 {
		return nil, fmt.Errorf(
			"no route definition files matched patterns: %s",
			strings.Join(normalizedRouteDefinitionPatterns, ", "),
		)
	}

	routeDefinitionFiles := make([]string, 0, len(matchedFilesByPath))
	for routeDefinitionFile := range matchedFilesByPath {
		routeDefinitionFiles = append(
			routeDefinitionFiles,
			filepath.ToSlash(routeDefinitionFile),
		)
	}
	sort.Strings(routeDefinitionFiles)
	return routeDefinitionFiles, nil
}

func patternContainsGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[{")
}

func expandRouteDefinitionPatternWithDoublestar(
	pattern string,
) ([]string, error) {
	return doublestar.FilepathGlob(pattern)
}

func parseRouteDefinitionFileIntoCalls(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
) (parsedRouteDefinitionsCode, error) {
	return defaultRouteParsingExecutor.parseRouteDefinitionFileIntoCalls(
		v,
		routeDefinitionFile,
	)
}

func (executor routeParsingExecutor) parseRouteDefinitionFileIntoCalls(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
) (parsedRouteDefinitionsCode, error) {
	code, err := executor.dependencies.readRouteDefinitionFile(
		routeDefinitionFile,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf(
			"read route definitions file %q: %w",
			routeDefinitionFile,
			err,
		)
	}
	parsedRouteDefinitions, err := executor.parseRouteDefinitionsCodeIntoCalls(
		v,
		code,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf(
			"parse route definitions file %q: %w",
			routeDefinitionFile,
			err,
		)
	}
	return parsedRouteDefinitions, nil
}

func (executor routeParsingExecutor) parseRouteDefinitionsCodeIntoCalls(
	v *vormaruntime.Vorma,
	code []byte,
) (parsedRouteDefinitionsCode, error) {
	transformedCode, err := executor.dependencies.transformRouteDefinitionsCode(
		v,
		code,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, err
	}

	routeCalls, unresolvedRoutes, err := executor.dependencies.extractRouteCallsFromTransformedCode(
		transformedCode,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf(
			"extract route calls: %w",
			err,
		)
	}

	return parsedRouteDefinitionsCode{
		routeCalls:       routeCalls,
		unresolvedRoutes: unresolvedRoutes,
	}, nil
}

func transformRouteDefinitionsCode(
	v *vormaruntime.Vorma,
	code []byte,
) (string, error) {
	transformResult := esbuild.Transform(string(code), esbuild.TransformOptions{
		Format:            esbuild.FormatESModule,
		Platform:          esbuild.PlatformNode,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		MinifyIdentifiers: false,
		Loader:            esbuild.LoaderTSX,
		Target:            esbuild.ES2020,
	})

	if len(transformResult.Errors) > 0 {
		logEsbuildTransformErrors(v, transformResult.Errors)
		return "", errors.New("esbuild transform failed")
	}

	return importRegex.ReplaceAllString(string(transformResult.Code), "$1"), nil
}

func logEsbuildTransformErrors(
	v *vormaruntime.Vorma,
	messages []esbuild.Message,
) {
	for _, message := range messages {
		v.Log.Error(fmt.Sprintf("esbuild error: %s", message.Text))
	}
}

func handleUnresolvedRouteCalls(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
	unresolvedRoutes []unresolvedRouteCall,
) error {
	if len(unresolvedRoutes) == 0 {
		return nil
	}

	unresolvedRoutePolicy, err := resolveUnresolvedRoutePolicy(v)
	if err != nil {
		return err
	}

	if unresolvedRoutePolicy == vormaruntime.UnresolvedRoutePolicyWarn {
		logUnresolvedRouteCallsAsWarnings(
			v,
			routeDefinitionFile,
			unresolvedRoutes,
		)
		return nil
	}

	return buildUnresolvedRouteCallsError(routeDefinitionFile, unresolvedRoutes)
}

func resolveUnresolvedRoutePolicy(v *vormaruntime.Vorma) (string, error) {
	if v == nil {
		return "", errors.New(
			"vorma runtime is required to resolve unresolved route policy",
		)
	}
	if v.Config == nil {
		return "", errors.New(
			"vorma config is required to resolve unresolved route policy",
		)
	}

	configuredPolicy := strings.TrimSpace(v.Config.UnresolvedRoutePolicy)
	if configuredPolicy != "" {
		switch configuredPolicy {
		case vormaruntime.UnresolvedRoutePolicyWarn:
			return vormaruntime.UnresolvedRoutePolicyWarn, nil
		case vormaruntime.UnresolvedRoutePolicyError:
			return vormaruntime.UnresolvedRoutePolicyError, nil
		default:
			return "", fmt.Errorf(
				"Vorma.UnresolvedRoutePolicy must be %q or %q",
				vormaruntime.UnresolvedRoutePolicyWarn,
				vormaruntime.UnresolvedRoutePolicyError,
			)
		}
	}

	return vormaruntime.UnresolvedRoutePolicyError, nil
}

func logUnresolvedRouteCallsAsWarnings(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
	unresolvedRoutes []unresolvedRouteCall,
) {
	for _, unresolved := range unresolvedRoutes {
		v.Log.Warn(
			fmt.Sprintf(
				"Route pattern %q has a module path that cannot be statically resolved",
				unresolved.Pattern,
			),
			"file",
			routeDefinitionFile,
			"expression",
			unresolved.RawModuleExpr,
			"reason",
			unresolved.Reason,
		)
		v.Log.Warn(
			"This route will be ignored. Use a static string path or a const variable assigned to a string literal.",
		)
	}
}

func buildUnresolvedRouteCallsError(
	routeDefinitionFile string,
	unresolvedRoutes []unresolvedRouteCall,
) error {
	unresolvedRouteErrors := make([]error, 0, len(unresolvedRoutes))
	for _, unresolvedRoute := range unresolvedRoutes {
		unresolvedRouteErrors = append(
			unresolvedRouteErrors,
			fmt.Errorf(
				"pattern %q has unresolved module expression %q (%s)",
				unresolvedRoute.Pattern,
				unresolvedRoute.RawModuleExpr,
				unresolvedRoute.Reason,
			),
		)
	}

	return fmt.Errorf(
		"unresolved route calls are not allowed in %q: %w",
		routeDefinitionFile,
		errors.Join(unresolvedRouteErrors...),
	)
}

func mergeRouteCallsIntoPaths(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
	routeDefinitionFile string,
	routeCalls []routeCall,
) error {
	return defaultRouteParsingExecutor.mergeRouteCallsIntoPaths(
		v,
		paths,
		routeDefinitionFile,
		routeCalls,
	)
}

func (executor routeParsingExecutor) mergeRouteCallsIntoPaths(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
	routeDefinitionFile string,
	routeCalls []routeCall,
) error {
	for _, routeCall := range routeCalls {
		if routeCall.Module == "" {
			return fmt.Errorf(
				"component module is required for pattern: %s",
				routeCall.Pattern,
			)
		}

		if _, hasExistingPattern := paths[routeCall.Pattern]; hasExistingPattern {
			return fmt.Errorf("duplicate route pattern: %s", routeCall.Pattern)
		}

		modulePath := executor.resolveRouteModulePath(
			v,
			routeDefinitionFile,
			routeCall,
		)
		if err := executor.ensureRouteModuleExists(modulePath, routeCall.Pattern); err != nil {
			return err
		}

		paths[routeCall.Pattern] = &vormaruntime.Path{
			OriginalPattern: routeCall.Pattern,
			SrcPath:         modulePath,
			ExportKey:       routeCall.Key,
			ErrorExportKey:  routeCall.ErrorKey,
		}
	}
	return nil
}

func (executor routeParsingExecutor) resolveRouteModulePath(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
	routeCall routeCall,
) string {
	routeDefinitionsDirectory := filepath.Dir(routeDefinitionFile)
	resolvedModulePath, err := executor.dependencies.computeRelativeModulePath(
		".",
		filepath.Join(routeDefinitionsDirectory, routeCall.Module),
	)
	if err != nil {
		v.Log.Warn(fmt.Sprintf("could not make module path relative: %s", err))
		resolvedModulePath = routeCall.Module
	}
	return filepath.ToSlash(resolvedModulePath)
}

func ensureRouteModuleExists(modulePath string, pattern string) error {
	return defaultRouteParsingExecutor.ensureRouteModuleExists(
		modulePath,
		pattern,
	)
}

func (executor routeParsingExecutor) ensureRouteModuleExists(
	modulePath string,
	pattern string,
) error {
	fileInfo, err := executor.dependencies.statRouteModulePath(modulePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"component module does not exist: %s (pattern: %s)",
				modulePath,
				pattern,
			)
		}
		return fmt.Errorf("access component module %s: %w", modulePath, err)
	}

	if fileInfo != nil && fileInfo.IsDir() {
		return fmt.Errorf(
			"component module is a directory: %s (pattern: %s)",
			modulePath,
			pattern,
		)
	}

	return nil
}

type routeRegistryBuildDependencies struct {
	marshalRouteManifestJSON                  func(any) ([]byte, error)
	writeRouteManifestJSON                    func(string, []byte, os.FileMode) error
	removeRouteManifestJSON                   func(string) error
	readStageOnePathsArtifact                 func(string) ([]byte, error)
	writeStageOnePathsArtifact                func(string, []byte, os.FileMode) error
	removeStageOnePathsArtifact               func(string) error
	writeGeneratedTypeScript                  func(*vormaruntime.LockedVorma) error
	writeStageOnePathsJSONForRuntimeState     func(*vormaruntime.Vorma, routeBuildRuntimeStateSnapshot, string) error
	writeGeneratedTypeScriptForRuntimeState   func(*vormaruntime.Vorma, routeBuildRuntimeStateSnapshot) error
	captureRouteBuildRuntimeStateWithReadLock func(*vormaruntime.Vorma) routeBuildRuntimeStateSnapshot
	readRouteManifestFileWithRuntimeLock      func(*vormaruntime.Vorma) string
	generateRouteManifestFromPaths            func(map[string]*vormaruntime.Path, *nestedmux.Router) map[string]int
	isRouteBuildRuntimeStateSnapshotCurrent   func(*vormaruntime.Vorma, routeBuildRuntimeStateSnapshot) bool
	commitRouteManifestFileWithRuntimeLock    func(*vormaruntime.Vorma, routeManifestCommitInput) bool
}

type routeRegistryBuildExecutor struct {
	dependencies routeRegistryBuildDependencies
}

var defaultRouteRegistryBuildExecutor = newRouteRegistryBuildExecutor(
	routeRegistryBuildDependencies{},
)

func defaultRouteRegistryBuildDependencies() routeRegistryBuildDependencies {
	return routeRegistryBuildDependencies{
		marshalRouteManifestJSON:    json.Marshal,
		writeRouteManifestJSON:      writeFileAtomically,
		removeRouteManifestJSON:     os.Remove,
		readStageOnePathsArtifact:   os.ReadFile,
		writeStageOnePathsArtifact:  writeFileAtomically,
		removeStageOnePathsArtifact: os.Remove,
		writeGeneratedTypeScript: func(l *vormaruntime.LockedVorma) error {
			return tsgenruntime.WriteGeneratedTS(
				l,
				tsgenruntime.WriteDependencies{
					WriteGeneratedTSFile: writeFileAtomically,
				},
			)
		},
		writeStageOnePathsJSONForRuntimeState: func(
			v *vormaruntime.Vorma,
			runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
			routeManifestFile string,
		) error {
			return writePathsToDiskStageOneFromRuntimeState(
				v,
				runtimeStateSnapshot,
				routeManifestFile,
			)
		},
		writeGeneratedTypeScriptForRuntimeState: func(
			v *vormaruntime.Vorma,
			runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
		) error {
			return tsgenruntime.WriteGeneratedTSForRuntimeState(
				v,
				tsgenruntime.RuntimeStateSnapshot{
					Paths:   runtimeStateSnapshot.paths,
					BuildID: runtimeStateSnapshot.buildID,
				},
				tsgenruntime.WriteDependencies{
					WriteGeneratedTSFile: writeFileAtomically,
				},
			)
		},
		captureRouteBuildRuntimeStateWithReadLock: func(v *vormaruntime.Vorma) routeBuildRuntimeStateSnapshot {
			var runtimeStateSnapshot routeBuildRuntimeStateSnapshot
			v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
				runtimeStateSnapshot = captureRouteBuildRuntimeState(l)
			})
			return runtimeStateSnapshot
		},
		readRouteManifestFileWithRuntimeLock: func(v *vormaruntime.Vorma) string {
			var previousRouteManifestFile string
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				previousRouteManifestFile = l.RouteManifestFile()
			})
			return previousRouteManifestFile
		},
		generateRouteManifestFromPaths:          generateRouteManifestFromPaths,
		isRouteBuildRuntimeStateSnapshotCurrent: routeBuildRuntimeStateSnapshotIsCurrent,
		commitRouteManifestFileWithRuntimeLock: func(
			v *vormaruntime.Vorma,
			commitInput routeManifestCommitInput,
		) bool {
			manifestCommitted := false
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				if !shouldCommitRouteManifestFileForRuntimeState(
					l.BuildID(),
					commitInput.expectedBuildID,
				) {
					return
				}
				commitRuntimeStateWithLock(
					l,
					runtimeStateCommitInput{
						shouldCommitRouteManifestFile: true,
						routeManifestFile:             commitInput.routeManifestFile,
					},
				)
				manifestCommitted = true
			})
			return manifestCommitted
		},
	}
}

func normalizeRouteRegistryBuildDependencies(
	dependencies routeRegistryBuildDependencies,
) routeRegistryBuildDependencies {
	defaultDependencies := defaultRouteRegistryBuildDependencies()

	if dependencies.marshalRouteManifestJSON == nil {
		dependencies.marshalRouteManifestJSON = defaultDependencies.marshalRouteManifestJSON
	}
	if dependencies.writeRouteManifestJSON == nil {
		dependencies.writeRouteManifestJSON = defaultDependencies.writeRouteManifestJSON
	}
	if dependencies.removeRouteManifestJSON == nil {
		dependencies.removeRouteManifestJSON = defaultDependencies.removeRouteManifestJSON
	}
	if dependencies.readStageOnePathsArtifact == nil {
		dependencies.readStageOnePathsArtifact = defaultDependencies.readStageOnePathsArtifact
	}
	if dependencies.writeStageOnePathsArtifact == nil {
		dependencies.writeStageOnePathsArtifact = defaultDependencies.writeStageOnePathsArtifact
	}
	if dependencies.removeStageOnePathsArtifact == nil {
		dependencies.removeStageOnePathsArtifact = defaultDependencies.removeStageOnePathsArtifact
	}
	if dependencies.writeGeneratedTypeScript == nil {
		dependencies.writeGeneratedTypeScript = defaultDependencies.writeGeneratedTypeScript
	}
	if dependencies.writeStageOnePathsJSONForRuntimeState == nil {
		dependencies.writeStageOnePathsJSONForRuntimeState = defaultDependencies.writeStageOnePathsJSONForRuntimeState
	}
	if dependencies.writeGeneratedTypeScriptForRuntimeState == nil {
		dependencies.writeGeneratedTypeScriptForRuntimeState = defaultDependencies.writeGeneratedTypeScriptForRuntimeState
	}
	if dependencies.captureRouteBuildRuntimeStateWithReadLock == nil {
		dependencies.captureRouteBuildRuntimeStateWithReadLock = defaultDependencies.captureRouteBuildRuntimeStateWithReadLock
	}
	if dependencies.readRouteManifestFileWithRuntimeLock == nil {
		dependencies.readRouteManifestFileWithRuntimeLock = defaultDependencies.readRouteManifestFileWithRuntimeLock
	}
	if dependencies.generateRouteManifestFromPaths == nil {
		dependencies.generateRouteManifestFromPaths = defaultDependencies.generateRouteManifestFromPaths
	}
	if dependencies.isRouteBuildRuntimeStateSnapshotCurrent == nil {
		dependencies.isRouteBuildRuntimeStateSnapshotCurrent = defaultDependencies.isRouteBuildRuntimeStateSnapshotCurrent
	}
	if dependencies.commitRouteManifestFileWithRuntimeLock == nil {
		dependencies.commitRouteManifestFileWithRuntimeLock = defaultDependencies.commitRouteManifestFileWithRuntimeLock
	}

	return dependencies
}

func newRouteRegistryBuildExecutor(
	dependencies routeRegistryBuildDependencies,
) routeRegistryBuildExecutor {
	return routeRegistryBuildExecutor{
		dependencies: normalizeRouteRegistryBuildDependencies(dependencies),
	}
}

// writeRouteArtifacts writes all route-related artifacts to disk.
// Includes manifest, paths JSON, and TypeScript generation.
func writeRouteArtifacts(l *vormaruntime.LockedVorma) error {
	return defaultRouteRegistryBuildExecutor.writeRouteArtifacts(l)
}

func (executor routeRegistryBuildExecutor) writeRouteArtifacts(
	l *vormaruntime.LockedVorma,
) error {
	v := l.Vorma()
	previousRouteManifestFile := l.RouteManifestFile()
	stageOnePathsArtifactPath := stageOnePathsArtifactOutputPath(v)

	stageOnePathsArtifactSnapshot, err := executor.captureStageOnePathsArtifact(
		stageOnePathsArtifactPath,
	)
	if err != nil {
		return fmt.Errorf("snapshot stage-one paths artifact: %w", err)
	}

	manifestFile, err := executor.writeRouteManifestArtifact(l)
	if err != nil {
		return fmt.Errorf("write route manifest: %w", err)
	}

	transactionErr := runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if err := writePathsToDiskStageOneWithRouteManifest(l, manifestFile); err != nil {
					return fmt.Errorf("write paths JSON: %w", err)
				}

				if err := executor.dependencies.writeGeneratedTypeScript(l); err != nil {
					return fmt.Errorf("write generated TypeScript: %w", err)
				}
				return nil
			},
			rollbackOnFailure: func() error {
				return executor.cleanupRouteArtifactsAfterWriteFailure(
					v,
					manifestFile,
					previousRouteManifestFile,
					stageOnePathsArtifactPath,
					stageOnePathsArtifactSnapshot,
				)
			},
			logRollbackFailureAfterPanic: func(rollbackErr error) {
				if v.Log != nil {
					v.Log.Error(
						"cleanup route artifacts after panic failed",
						"error",
						rollbackErr,
					)
				}
			},
		},
	)
	if transactionErr != nil {
		return transactionErr
	}

	commitRuntimeStateWithLock(
		l,
		runtimeStateCommitInput{
			shouldCommitRouteManifestFile: true,
			routeManifestFile:             manifestFile,
		},
	)
	return nil
}

func writeAndSetRouteManifest(l *vormaruntime.LockedVorma) error {
	return defaultRouteRegistryBuildExecutor.writeAndSetRouteManifest(l)
}

func (executor routeRegistryBuildExecutor) writeAndSetRouteManifest(
	l *vormaruntime.LockedVorma,
) error {
	manifestFile, err := executor.writeRouteManifestArtifact(l)
	if err != nil {
		return err
	}

	commitRuntimeStateWithLock(
		l,
		runtimeStateCommitInput{
			shouldCommitRouteManifestFile: true,
			routeManifestFile:             manifestFile,
		},
	)
	return nil
}

func (executor routeRegistryBuildExecutor) writeRouteManifestArtifact(
	l *vormaruntime.LockedVorma,
) (string, error) {
	v := l.Vorma()
	manifest := generateRouteManifest(l, v.LoadersRouter().NestedRouter)
	manifestFile, err := executor.writeRouteManifestToDisk(v, manifest)
	if err != nil {
		return "", err
	}
	return manifestFile, nil
}

func writeRouteManifestToDisk(
	v *vormaruntime.Vorma,
	manifest map[string]int,
) (string, error) {
	return defaultRouteRegistryBuildExecutor.writeRouteManifestToDisk(
		v,
		manifest,
	)
}

func (executor routeRegistryBuildExecutor) writeRouteManifestToDisk(
	v *vormaruntime.Vorma,
	manifest map[string]int,
) (string, error) {
	manifestJSON, err := executor.dependencies.marshalRouteManifestJSON(
		manifest,
	)
	if err != nil {
		return "", fmt.Errorf("marshal route manifest: %w", err)
	}

	filename := routeManifestFilename(manifestJSON)

	outPath := filepath.Join(v.Wave.StaticPublicOutDir(), filename)
	if err := executor.dependencies.writeRouteManifestJSON(
		outPath,
		manifestJSON,
		buildArtifactFileMode,
	); err != nil {
		return "", fmt.Errorf("write route manifest: %w", err)
	}

	return filename, nil
}

type stageOnePathsArtifactSnapshot = buildArtifactFileSnapshot

type routeManifestStateSnapshot struct {
	previousRouteManifestFile string
	routeManifest             map[string]int
}

type routeManifestCommitInput struct {
	expectedBuildID   string
	routeManifestFile string
}

type routeArtifactWritePlan struct {
	runtimeStateSnapshot      routeBuildRuntimeStateSnapshot
	manifestStateSnapshot     routeManifestStateSnapshot
	stageOnePathsArtifactPath string
	stageOnePathsSnapshot     stageOnePathsArtifactSnapshot
}

type routeManifestStateReader interface {
	Vorma() *vormaruntime.Vorma
	Paths() map[string]*vormaruntime.Path
}

var errRouteBuildRuntimeStateSuperseded = errors.New(
	"route build runtime state superseded by newer build",
)

func writeRouteArtifactsWithoutHoldingRuntimeLock(v *vormaruntime.Vorma) error {
	return defaultRouteRegistryBuildExecutor.writeRouteArtifactsWithoutHoldingRuntimeLock(
		v,
	)
}

func (executor routeRegistryBuildExecutor) writeRouteArtifactsWithoutHoldingRuntimeLock(
	v *vormaruntime.Vorma,
) error {
	plannedWrite, err := executor.planRouteArtifactWrite(v)
	if err != nil {
		return err
	}

	manifestFile, shouldCommit, err := executor.stageRouteArtifactWrite(
		v,
		plannedWrite,
	)
	if err != nil {
		return err
	}
	if !shouldCommit {
		return nil
	}

	executor.commitRouteArtifactWrite(v, plannedWrite, manifestFile)
	return nil
}

func (executor routeRegistryBuildExecutor) planRouteArtifactWrite(
	v *vormaruntime.Vorma,
) (routeArtifactWritePlan, error) {
	runtimeStateSnapshot := executor.dependencies.captureRouteBuildRuntimeStateWithReadLock(
		v,
	)
	manifestStateSnapshot := routeManifestStateSnapshot{
		previousRouteManifestFile: executor.dependencies.readRouteManifestFileWithRuntimeLock(
			v,
		),
		routeManifest: executor.dependencies.generateRouteManifestFromPaths(
			runtimeStateSnapshot.paths,
			v.LoadersRouter().NestedRouter,
		),
	}

	stageOnePathsArtifactPath := stageOnePathsArtifactOutputPath(v)
	stageOnePathsSnapshot, err := executor.captureStageOnePathsArtifact(
		stageOnePathsArtifactPath,
	)
	if err != nil {
		return routeArtifactWritePlan{}, fmt.Errorf(
			"snapshot stage-one paths artifact: %w",
			err,
		)
	}

	return routeArtifactWritePlan{
		runtimeStateSnapshot:      runtimeStateSnapshot,
		manifestStateSnapshot:     manifestStateSnapshot,
		stageOnePathsArtifactPath: stageOnePathsArtifactPath,
		stageOnePathsSnapshot:     stageOnePathsSnapshot,
	}, nil
}

func (executor routeRegistryBuildExecutor) stageRouteArtifactWrite(
	v *vormaruntime.Vorma,
	plannedWrite routeArtifactWritePlan,
) (string, bool, error) {
	manifestFile, err := executor.writeRouteManifestToDisk(
		v,
		plannedWrite.manifestStateSnapshot.routeManifest,
	)
	if err != nil {
		return "", false, fmt.Errorf("write route manifest: %w", err)
	}

	skipRollbackForSupersededRuntimeState := false
	transactionErr := runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if !executor.dependencies.isRouteBuildRuntimeStateSnapshotCurrent(
					v,
					plannedWrite.runtimeStateSnapshot,
				) {
					skipRollbackForSupersededRuntimeState = true
					return errRouteBuildRuntimeStateSuperseded
				}

				if err := executor.dependencies.writeStageOnePathsJSONForRuntimeState(
					v,
					plannedWrite.runtimeStateSnapshot,
					manifestFile,
				); err != nil {
					return fmt.Errorf("write paths JSON: %w", err)
				}
				if !executor.dependencies.isRouteBuildRuntimeStateSnapshotCurrent(
					v,
					plannedWrite.runtimeStateSnapshot,
				) {
					skipRollbackForSupersededRuntimeState = true
					return errRouteBuildRuntimeStateSuperseded
				}

				if err := executor.dependencies.writeGeneratedTypeScriptForRuntimeState(
					v,
					plannedWrite.runtimeStateSnapshot,
				); err != nil {
					return fmt.Errorf("write generated TypeScript: %w", err)
				}
				if !executor.dependencies.isRouteBuildRuntimeStateSnapshotCurrent(
					v,
					plannedWrite.runtimeStateSnapshot,
				) {
					skipRollbackForSupersededRuntimeState = true
					return errRouteBuildRuntimeStateSuperseded
				}
				return nil
			},
			rollbackOnFailure: func() error {
				if skipRollbackForSupersededRuntimeState {
					return nil
				}
				return executor.cleanupRouteArtifactsAfterWriteFailure(
					v,
					manifestFile,
					plannedWrite.manifestStateSnapshot.previousRouteManifestFile,
					plannedWrite.stageOnePathsArtifactPath,
					plannedWrite.stageOnePathsSnapshot,
				)
			},
			logRollbackFailureAfterPanic: func(rollbackErr error) {
				if v.Log != nil {
					v.Log.Error(
						"cleanup route artifacts after panic failed",
						"error",
						rollbackErr,
					)
				}
			},
		},
	)
	if transactionErr != nil {
		if errors.Is(transactionErr, errRouteBuildRuntimeStateSuperseded) {
			return "", false, nil
		}
		return "", false, transactionErr
	}

	return manifestFile, true, nil
}

func (executor routeRegistryBuildExecutor) commitRouteArtifactWrite(
	v *vormaruntime.Vorma,
	plannedWrite routeArtifactWritePlan,
	manifestFile string,
) {
	executor.dependencies.commitRouteManifestFileWithRuntimeLock(
		v,
		routeManifestCommitInput{
			expectedBuildID:   plannedWrite.runtimeStateSnapshot.buildID,
			routeManifestFile: manifestFile,
		},
	)
}

func routeBuildRuntimeStateSnapshotIsCurrent(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
) bool {
	var currentBuildID string
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		currentBuildID = l.BuildID()
	})
	return currentBuildID == runtimeStateSnapshot.buildID
}

func shouldCommitRouteManifestFileForRuntimeState(
	currentBuildID string,
	expectedBuildID string,
) bool {
	return currentBuildID == expectedBuildID
}

func stageOnePathsArtifactOutputPath(v *vormaruntime.Vorma) string {
	return pathsOutputPath(v, runtimepaths.VormaPathsStageOneJSONFileName)
}

func (executor routeRegistryBuildExecutor) captureStageOnePathsArtifact(
	stageOnePathsArtifactPath string,
) (stageOnePathsArtifactSnapshot, error) {
	return captureBuildArtifactFile(
		stageOnePathsArtifactPath,
		executor.dependencies.readStageOnePathsArtifact,
	)
}

func (executor routeRegistryBuildExecutor) cleanupRouteArtifactsAfterWriteFailure(
	v *vormaruntime.Vorma,
	manifestFile string,
	previousRouteManifestFile string,
	stageOnePathsArtifactPath string,
	stageOnePathsSnapshot stageOnePathsArtifactSnapshot,
) error {
	artifactCleanupErrors := make([]error, 0, 2)
	if shouldRemoveRouteManifestArtifactAfterWriteFailure(
		manifestFile,
		previousRouteManifestFile,
	) {
		if err := executor.removeRouteManifestArtifactFile(v, manifestFile); err != nil {
			artifactCleanupErrors = append(
				artifactCleanupErrors,
				fmt.Errorf("cleanup route manifest artifact: %w", err),
			)
		}
	}
	if err := executor.restoreStageOnePathsArtifactFromState(
		stageOnePathsArtifactPath,
		stageOnePathsSnapshot,
	); err != nil {
		artifactCleanupErrors = append(
			artifactCleanupErrors,
			fmt.Errorf("cleanup stage-one paths artifact: %w", err),
		)
	}

	return errors.Join(artifactCleanupErrors...)
}

func shouldRemoveRouteManifestArtifactAfterWriteFailure(
	manifestFile string,
	previousRouteManifestFile string,
) bool {
	return manifestFile != "" && manifestFile != previousRouteManifestFile
}

func (executor routeRegistryBuildExecutor) removeRouteManifestArtifactFile(
	v *vormaruntime.Vorma,
	manifestFile string,
) error {
	manifestFilePath := filepath.Join(v.Wave.StaticPublicOutDir(), manifestFile)
	err := executor.dependencies.removeRouteManifestJSON(manifestFilePath)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func (executor routeRegistryBuildExecutor) restoreStageOnePathsArtifactFromState(
	stageOnePathsArtifactPath string,
	stageOnePathsSnapshot stageOnePathsArtifactSnapshot,
) error {
	return restoreBuildArtifactFile(
		stageOnePathsArtifactPath,
		stageOnePathsSnapshot,
		executor.dependencies.writeStageOnePathsArtifact,
		executor.dependencies.removeStageOnePathsArtifact,
	)
}

func generateRouteManifest(
	l routeManifestStateReader,
	nestedRouter *nestedmux.Router,
) map[string]int {
	return generateRouteManifestFromPaths(l.Paths(), nestedRouter)
}

func generateRouteManifestFromPaths(
	paths map[string]*vormaruntime.Path,
	nestedRouter *nestedmux.Router,
) map[string]int {
	manifest := make(map[string]int)
	for _, currentPath := range paths {
		manifest[currentPath.OriginalPattern] = routeManifestServerLoaderFlag(
			nestedRouter,
			currentPath.OriginalPattern,
		)
	}

	return manifest
}

func routeManifestFilename(manifestJSON []byte) string {
	hash := cryptoutil.Sha256Hash(manifestJSON)
	hashStr := base64.RawURLEncoding.EncodeToString(hash[:8])
	return fmt.Sprintf(
		"%s%s.json",
		vormaruntime.VormaRouteManifestPrefix,
		hashStr,
	)
}

func routeManifestServerLoaderFlag(
	nestedRouter *nestedmux.Router,
	pattern string,
) int {
	if nestedRouter.HasTaskHandler(pattern) {
		return 1
	}
	return 0
}

type postRouteSyncHook func(*vormaruntime.Vorma) error

type routeSyncExecutionOptions struct {
	parseClientRoutes          func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	generateBuildID            func() (string, error)
	parseClientRoutesErrorText string
	postSyncHook               postRouteSyncHook
}

func prepareParsedRouteSyncInput(
	v *vormaruntime.Vorma,
	parseClientRoutes func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error),
	generateBuildID func() (string, error),
	parseClientRoutesErrorContext string,
) (map[string]*vormaruntime.Path, string, error) {
	clientPaths, err := parseClientRoutes(v)
	if err != nil {
		if parseClientRoutesErrorContext != "" {
			return nil, "", fmt.Errorf(
				"%s: %w",
				parseClientRoutesErrorContext,
				err,
			)
		}
		return nil, "", err
	}

	buildID := ""
	if generateBuildID != nil {
		buildID, err = generateBuildID()
		if err != nil {
			return nil, "", err
		}
	}

	return clientPaths, buildID, nil
}

func runRouteSyncExecution(
	v *vormaruntime.Vorma,
	options routeSyncExecutionOptions,
) error {
	if options.parseClientRoutes == nil {
		return errors.New("route sync parse function is required")
	}

	clientPaths, buildID, err := prepareParsedRouteSyncInput(
		v,
		options.parseClientRoutes,
		options.generateBuildID,
		options.parseClientRoutesErrorText,
	)
	if err != nil {
		return err
	}

	return syncClientRoutesFromParsedPathsWithLock(
		v,
		clientPaths,
		buildID,
		options.postSyncHook,
	)
}

func syncClientRoutesFromParsedPathsWithLock(
	v *vormaruntime.Vorma,
	clientPaths map[string]*vormaruntime.Path,
	buildID string,
	postSyncHook postRouteSyncHook,
) error {
	var previousRuntimeState buildRuntimeStateSnapshot
	var currentAttemptCommittedBuildID string
	return runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				v.WithLock(func(l *vormaruntime.LockedVorma) {
					previousRuntimeState = captureBuildRuntimeState(l)
					commitRuntimeStateWithLock(
						l,
						runtimeStateCommitInput{
							shouldCommitBuildID:  buildID != "",
							buildID:              buildID,
							routePaths:           clientPaths,
							routePathsUpdateMode: runtimeStateRoutePathsUpdateModeSyncFromDevReload,
						},
					)
					currentAttemptCommittedBuildID = l.BuildID()
				})

				if postSyncHook != nil {
					return postSyncHook(v)
				}
				return nil
			},
			rollbackOnFailure: func() error {
				v.WithLock(func(l *vormaruntime.LockedVorma) {
					rollbackRouteSyncStateAfterPostSyncFailure(
						l,
						previousRuntimeState,
						currentAttemptCommittedBuildID,
					)
				})
				return nil
			},
		},
	)
}

func rollbackRouteSyncStateAfterPostSyncFailure(
	l *vormaruntime.LockedVorma,
	previousRuntimeState buildRuntimeStateSnapshot,
	currentAttemptCommittedBuildID string,
) {
	if !shouldRollbackRouteSyncStateAfterPostSyncFailure(
		l.BuildID(),
		currentAttemptCommittedBuildID,
	) {
		return
	}
	restoreBuildRuntimeState(l, previousRuntimeState)
}

func shouldRollbackRouteSyncStateAfterPostSyncFailure(
	currentBuildID string,
	currentAttemptCommittedBuildID string,
) bool {
	return shouldRestoreRuntimeStateSnapshotForAttemptBuildID(
		currentBuildID,
		currentAttemptCommittedBuildID,
	)
}

type runtimeStateRoutePathsUpdateMode uint8

const (
	runtimeStateRoutePathsUpdateModeNoPathMutation runtimeStateRoutePathsUpdateMode = iota
	runtimeStateRoutePathsUpdateModeReplaceParsedPathsForInit
	runtimeStateRoutePathsUpdateModeSyncFromDevReload
)

type runtimeStateCommitInput struct {
	shouldCommitIsDev             bool
	isDev                         bool
	shouldCommitBuildID           bool
	buildID                       string
	shouldCommitRouteManifestFile bool
	routeManifestFile             string
	routePaths                    map[string]*vormaruntime.Path
	routePathsUpdateMode          runtimeStateRoutePathsUpdateMode
	shouldRebuildNestedRouter     bool
}

func commitRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateCommitInput runtimeStateCommitInput,
) {
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		commitRuntimeStateWithLock(l, runtimeStateCommitInput)
	})
}

func commitRuntimeStateWithLock(
	l *vormaruntime.LockedVorma,
	runtimeStateCommitInput runtimeStateCommitInput,
) {
	if runtimeStateCommitInput.shouldCommitIsDev {
		l.SetIsDev(runtimeStateCommitInput.isDev)
	}

	switch runtimeStateCommitInput.routePathsUpdateMode {
	case runtimeStateRoutePathsUpdateModeNoPathMutation:
		// no-op
	case runtimeStateRoutePathsUpdateModeReplaceParsedPathsForInit:
		l.Routes().ReplaceParsedPathsForInit(
			runtimeStateCommitInput.routePaths,
			runtimeStateCommitInput.shouldRebuildNestedRouter,
		)
	case runtimeStateRoutePathsUpdateModeSyncFromDevReload:
		l.Routes().SyncFromDevReload(runtimeStateCommitInput.routePaths)
	default:
		panic(fmt.Sprintf(
			"unsupported runtime state route paths update mode: %d",
			runtimeStateCommitInput.routePathsUpdateMode,
		))
	}

	if runtimeStateCommitInput.shouldCommitBuildID {
		l.SetBuildID(runtimeStateCommitInput.buildID)
	}

	if runtimeStateCommitInput.shouldCommitRouteManifestFile {
		l.SetRouteManifestFile(runtimeStateCommitInput.routeManifestFile)
	}
}

func shouldRebuildNestedRouterFromCurrentRuntimeState(
	l *vormaruntime.LockedVorma,
) bool {
	return l.Vorma().LoadersRouter() != nil &&
		l.Vorma().LoadersRouter().NestedRouter != nil
}

func currentBuildIDWithReadLock(v *vormaruntime.Vorma) string {
	var currentBuildID string
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		currentBuildID = l.BuildID()
	})
	return currentBuildID
}

func shouldRestoreRuntimeStateSnapshotForAttemptBuildID(
	currentBuildID string,
	currentAttemptCommittedBuildID string,
) bool {
	if currentAttemptCommittedBuildID == "" {
		return true
	}
	return currentBuildID == currentAttemptCommittedBuildID
}

type buildRuntimeStateSnapshot struct {
	isDev                  bool
	routeBuildRuntimeState routeBuildRuntimeStateSnapshot
}

type routeBuildRuntimeStateSnapshot struct {
	paths             map[string]*vormaruntime.Path
	buildID           string
	routeManifestFile string
}

type buildRuntimeStateReader interface {
	IsDev() bool
	Paths() map[string]*vormaruntime.Path
	BuildID() string
	RouteManifestFile() string
}

type routeBuildRuntimeStateReader interface {
	Paths() map[string]*vormaruntime.Path
	BuildID() string
	RouteManifestFile() string
}

func captureBuildRuntimeState(
	l buildRuntimeStateReader,
) buildRuntimeStateSnapshot {
	return buildRuntimeStateSnapshot{
		isDev:                  l.IsDev(),
		routeBuildRuntimeState: captureRouteBuildRuntimeState(l),
	}
}

func captureRouteBuildRuntimeState(
	l routeBuildRuntimeStateReader,
) routeBuildRuntimeStateSnapshot {
	return routeBuildRuntimeStateSnapshot{
		paths:             cloneRouteBuildRuntimePathsMap(l.Paths()),
		buildID:           l.BuildID(),
		routeManifestFile: l.RouteManifestFile(),
	}
}

func restoreBuildRuntimeState(
	l *vormaruntime.LockedVorma,
	snapshot buildRuntimeStateSnapshot,
) {
	commitRuntimeStateWithLock(
		l,
		runtimeStateCommitInput{
			shouldCommitIsDev:             true,
			isDev:                         snapshot.isDev,
			shouldCommitBuildID:           true,
			buildID:                       snapshot.routeBuildRuntimeState.buildID,
			shouldCommitRouteManifestFile: true,
			routeManifestFile:             snapshot.routeBuildRuntimeState.routeManifestFile,
			routePaths:                    snapshot.routeBuildRuntimeState.paths,
			routePathsUpdateMode:          runtimeStateRoutePathsUpdateModeReplaceParsedPathsForInit,
			shouldRebuildNestedRouter: shouldRebuildNestedRouterFromCurrentRuntimeState(
				l,
			),
		},
	)
}

func restoreRouteBuildRuntimeState(
	l *vormaruntime.LockedVorma,
	snapshot routeBuildRuntimeStateSnapshot,
) {
	commitRuntimeStateWithLock(
		l,
		runtimeStateCommitInput{
			shouldCommitBuildID:           true,
			buildID:                       snapshot.buildID,
			shouldCommitRouteManifestFile: true,
			routeManifestFile:             snapshot.routeManifestFile,
			routePaths:                    snapshot.paths,
			routePathsUpdateMode:          runtimeStateRoutePathsUpdateModeReplaceParsedPathsForInit,
			shouldRebuildNestedRouter: shouldRebuildNestedRouterFromCurrentRuntimeState(
				l,
			),
		},
	)
}

func buildRuntimeStateSnapshotMatches(
	l buildRuntimeStateReader,
	snapshot buildRuntimeStateSnapshot,
) bool {
	if l.IsDev() != snapshot.isDev {
		return false
	}
	return routeBuildRuntimeStateSnapshotMatches(
		l,
		snapshot.routeBuildRuntimeState,
	)
}

func routeBuildRuntimeStateSnapshotMatches(
	l routeBuildRuntimeStateReader,
	snapshot routeBuildRuntimeStateSnapshot,
) bool {
	if l.BuildID() != snapshot.buildID {
		return false
	}
	if l.RouteManifestFile() != snapshot.routeManifestFile {
		return false
	}
	return routeBuildRuntimePathsMapMatches(l.Paths(), snapshot.paths)
}

func routeBuildRuntimePathsMapMatches(
	currentPaths map[string]*vormaruntime.Path,
	expectedPaths map[string]*vormaruntime.Path,
) bool {
	if len(currentPaths) != len(expectedPaths) {
		return false
	}
	for routePattern, currentPath := range currentPaths {
		expectedPath, hasExpectedPath := expectedPaths[routePattern]
		if !hasExpectedPath {
			return false
		}
		if !routeBuildRuntimePathMatches(currentPath, expectedPath) {
			return false
		}
	}
	return true
}

func routeBuildRuntimePathMatches(
	currentPath *vormaruntime.Path,
	expectedPath *vormaruntime.Path,
) bool {
	if currentPath == nil || expectedPath == nil {
		return currentPath == expectedPath
	}
	if currentPath.OriginalPattern != expectedPath.OriginalPattern {
		return false
	}
	if currentPath.SrcPath != expectedPath.SrcPath {
		return false
	}
	if currentPath.ExportKey != expectedPath.ExportKey {
		return false
	}
	if currentPath.ErrorExportKey != expectedPath.ErrorExportKey {
		return false
	}
	if currentPath.OutPath != expectedPath.OutPath {
		return false
	}
	if len(currentPath.Deps) != len(expectedPath.Deps) {
		return false
	}
	for depIndex := range currentPath.Deps {
		if currentPath.Deps[depIndex] != expectedPath.Deps[depIndex] {
			return false
		}
	}
	return true
}

func cloneRouteBuildRuntimePathsMap(
	paths map[string]*vormaruntime.Path,
) map[string]*vormaruntime.Path {
	if paths == nil {
		return nil
	}

	clonedPaths := make(map[string]*vormaruntime.Path, len(paths))
	for pattern, path := range paths {
		clonedPaths[pattern] = cloneRouteBuildRuntimePath(path)
	}
	return clonedPaths
}

func cloneRouteBuildRuntimePath(path *vormaruntime.Path) *vormaruntime.Path {
	if path == nil {
		return nil
	}

	clonedPath := *path
	if path.Deps != nil {
		clonedPath.Deps = append([]string(nil), path.Deps...)
	}
	return &clonedPath
}

func generateBuildIDWithPrefix(
	prefix string,
	generateBuildIDSuffix func() (string, error),
) (string, error) {
	buildIDSuffix, err := generateBuildIDSuffix()
	if err != nil {
		return "", fmt.Errorf("generate build ID: %w", err)
	}

	return prefix + buildIDSuffix, nil
}

const buildArtifactFileMode fs.FileMode = 0o644

type pathsJSONWriteDependencies struct {
	makePathsOutputDirectory func(string, fs.FileMode) error
	writePathsJSON           func(string, []byte, fs.FileMode) error
}

func writePathsJSONBytesToOutputPath(
	outputPath string,
	pathsAsJSON []byte,
	pathsWriteDeps pathsJSONWriteDependencies,
	makeOutputDirectoryErrorContext string,
	writePathsErrorContext string,
) error {
	if err := pathsWriteDeps.makePathsOutputDirectory(filepath.Dir(outputPath), os.ModePerm); err != nil {
		return fmt.Errorf("%s: %w", makeOutputDirectoryErrorContext, err)
	}

	if err := pathsWriteDeps.writePathsJSON(outputPath, pathsAsJSON, buildArtifactFileMode); err != nil {
		return fmt.Errorf("%s: %w", writePathsErrorContext, err)
	}
	return nil
}

type closableResource interface {
	Close() error
}

func runWithClosableResource[T closableResource](
	resource T,
	closeErrorContext string,
	runWithResource func(T) error,
) (operationErr error) {
	defer func() {
		operationErr = joinOperationErrorWithCloseError(
			operationErr,
			closeErrorContext,
			resource.Close(),
		)
	}()

	return runWithResource(resource)
}

func joinOperationErrorWithCloseError(
	operationErr error,
	closeErrorContext string,
	closeErr error,
) error {
	if closeErr == nil {
		return operationErr
	}

	closeErrWithContext := fmt.Errorf("%s: %w", closeErrorContext, closeErr)
	if operationErr == nil {
		return closeErrWithContext
	}
	return errors.Join(operationErr, closeErrWithContext)
}

type buildArtifactFileSnapshot struct {
	existed bool
	content []byte
}

func captureBuildArtifactFile(
	artifactPath string,
	readArtifactFile func(string) ([]byte, error),
) (buildArtifactFileSnapshot, error) {
	artifactContent, err := readArtifactFile(artifactPath)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR) {
			return buildArtifactFileSnapshot{}, nil
		}
		return buildArtifactFileSnapshot{}, err
	}

	return buildArtifactFileSnapshot{
		existed: true,
		content: artifactContent,
	}, nil
}

func restoreBuildArtifactFile(
	artifactPath string,
	snapshot buildArtifactFileSnapshot,
	writeArtifactFile func(string, []byte, os.FileMode) error,
	removeArtifactFile func(string) error,
) error {
	if snapshot.existed {
		return writeArtifactFile(
			artifactPath,
			snapshot.content,
			buildArtifactFileMode,
		)
	}

	err := removeArtifactFile(artifactPath)
	if err == nil || os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR) {
		return nil
	}
	return err
}

func normalizeRouteDefinitionPatternsInInputOrder(
	routeDefinitionPatterns []string,
) ([]string, error) {
	normalizedPatterns := make([]string, 0, len(routeDefinitionPatterns))
	seenPatterns := make(map[string]struct{}, len(routeDefinitionPatterns))
	for index, routeDefinitionPattern := range routeDefinitionPatterns {
		trimmedRouteDefinitionPattern := strings.TrimSpace(
			routeDefinitionPattern,
		)
		if trimmedRouteDefinitionPattern == "" {
			return nil, fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d] cannot be empty or whitespace",
				index,
			)
		}
		if trimmedRouteDefinitionPattern != routeDefinitionPattern {
			return nil, fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d]=%q must not contain surrounding whitespace",
				index,
				routeDefinitionPattern,
			)
		}
		if _, hasSeenPattern := seenPatterns[trimmedRouteDefinitionPattern]; hasSeenPattern {
			return nil, fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d]=%q duplicates an earlier pattern",
				index,
				trimmedRouteDefinitionPattern,
			)
		}

		seenPatterns[trimmedRouteDefinitionPattern] = struct{}{}
		normalizedPatterns = append(
			normalizedPatterns,
			trimmedRouteDefinitionPattern,
		)
	}
	return normalizedPatterns, nil
}

type fsFileSummary struct {
	path string
	size int64
}

func getFSSummaryHash(fsys fs.FS) ([]byte, error) {
	var fileSummaries []fsFileSummary
	err := fs.WalkDir(
		fsys,
		".",
		func(path string, dirEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == "." || dirEntry.IsDir() {
				return nil
			}
			info, err := dirEntry.Info()
			if err != nil {
				return err
			}
			fileSummaries = append(fileSummaries, fsFileSummary{
				path: path,
				size: info.Size(),
			})
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	sort.Slice(fileSummaries, func(i int, j int) bool {
		return fileSummaries[i].path < fileSummaries[j].path
	})

	hash := sha256.New()
	for _, fileSummary := range fileSummaries {
		hash.Write([]byte(fileSummary.path))
		hash.Write([]byte{0})

		sizeBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(sizeBytes, uint64(fileSummary.size))
		hash.Write(sizeBytes)
		hash.Write([]byte{0})

		fileContents, err := fs.ReadFile(fsys, fileSummary.path)
		if err != nil {
			return nil, fmt.Errorf(
				"read %s for FS summary hash: %w",
				fileSummary.path,
				err,
			)
		}
		hash.Write(fileContents)
		hash.Write([]byte{0})
	}
	return hash.Sum(nil), nil
}

type postViteProdBuildDependencies struct {
	toPathsFileStageTwo      func(*vormaruntime.Vorma) (*runtimepaths.PathsFile, error)
	writePathsToDiskStageTwo func(*vormaruntime.Vorma, *runtimepaths.PathsFile) error
	applyBuildIDToVorma      func(*vormaruntime.Vorma, string)
}

func defaultPostViteProdBuildDependencies() postViteProdBuildDependencies {
	return postViteProdBuildDependencies{
		toPathsFileStageTwo:      toPathsFileStageTwo,
		writePathsToDiskStageTwo: writePathsToDiskStageTwo,
		applyBuildIDToVorma:      applyBuildIDToVorma,
	}
}

func normalizePostViteProdBuildDependencies(
	dependencies postViteProdBuildDependencies,
) postViteProdBuildDependencies {
	defaultDependencies := defaultPostViteProdBuildDependencies()

	if dependencies.toPathsFileStageTwo == nil {
		dependencies.toPathsFileStageTwo = defaultDependencies.toPathsFileStageTwo
	}
	if dependencies.writePathsToDiskStageTwo == nil {
		dependencies.writePathsToDiskStageTwo = defaultDependencies.writePathsToDiskStageTwo
	}
	if dependencies.applyBuildIDToVorma == nil {
		dependencies.applyBuildIDToVorma = defaultDependencies.applyBuildIDToVorma
	}

	return dependencies
}

func postViteProdBuild(v *vormaruntime.Vorma) error {
	return postViteProdBuildWithDependencies(v, postViteProdBuildDependencies{})
}

func postViteProdBuildWithDependencies(
	v *vormaruntime.Vorma,
	dependencies postViteProdBuildDependencies,
) error {
	dependencies = normalizePostViteProdBuildDependencies(dependencies)

	pathsFile, err := dependencies.toPathsFileStageTwo(v)
	if err != nil {
		return fmt.Errorf("convert paths to stage two: %w", err)
	}

	if err := dependencies.writePathsToDiskStageTwo(v, pathsFile); err != nil {
		return fmt.Errorf("write stage-two paths: %w", err)
	}

	dependencies.applyBuildIDToVorma(v, pathsFile.BuildID)
	return nil
}

type routeParsingMetadata struct {
	routeFuncNames    map[string]bool
	trackedModuleVars map[string]string
}

func collectRouteParsingMetadata(parsedAST *js.AST) routeParsingMetadata {
	routeFuncNames := make(map[string]bool)
	trackedModuleVars := make(map[string]string)

	for _, statement := range parsedAST.BlockStmt.List {
		if importStmt, isImportStmt := statement.(*js.ImportStmt); isImportStmt {
			if !isBuildtimeImportStatement(importStmt) {
				continue
			}
			for _, alias := range importStmt.List {
				if !isRouteImportAlias(alias) {
					continue
				}
				routeFuncName := routeImportAliasBinding(alias)
				if routeFuncName != "" {
					routeFuncNames[routeFuncName] = true
				}
			}
			continue
		}

		varDecl, isVarDecl := statement.(*js.VarDecl)
		if !isVarDecl {
			continue
		}
		for _, binding := range varDecl.List {
			varBinding, ok := binding.Binding.(*js.Var)
			if !ok {
				continue
			}
			modulePath, ok := extractStaticStringLiteral(binding.Default)
			if !ok {
				continue
			}
			trackedModuleVars[string(varBinding.Data)] = modulePath
		}
	}
	return routeParsingMetadata{
		routeFuncNames:    routeFuncNames,
		trackedModuleVars: trackedModuleVars,
	}
}

func isBuildtimeImportStatement(importStmt *js.ImportStmt) bool {
	return strings.Trim(
		string(importStmt.Module),
		`"'`+"`",
	) == "vorma/buildtime"
}

func isRouteImportAlias(alias js.Alias) bool {
	aliasName := string(alias.Name)
	aliasBinding := string(alias.Binding)
	return aliasName == "route" || (aliasName == "" && aliasBinding == "route")
}

func routeImportAliasBinding(alias js.Alias) string {
	if len(alias.Binding) > 0 {
		return string(alias.Binding)
	}
	return string(alias.Name)
}

type viteManifestApplicationResult struct {
	clientEntryOut    string
	clientEntryDeps   []string
	depToCSSBundleMap map[string][]string
}

func applyViteManifestToPaths(
	viteManifest viteutil.Manifest,
	paths map[string]*vormaruntime.Path,
	cleanClientEntry string,
) viteManifestApplicationResult {
	result := viteManifestApplicationResult{
		clientEntryDeps:   []string{},
		depToCSSBundleMap: make(map[string][]string),
	}
	pathsBySourcePath := indexPathsBySourcePath(paths)

	for key, chunk := range viteManifest {
		cleanChunkOutPath := filepath.Base(chunk.File)

		if len(chunk.CSS) > 0 {
			result.depToCSSBundleMap[cleanChunkOutPath] = collectCSSBundleFileNames(
				chunk.CSS,
			)
		}

		dependencies := viteutil.FindAllDependencies(viteManifest, key)

		if chunk.IsEntry && cleanClientEntry == chunk.Src {
			result.clientEntryOut = cleanChunkOutPath
			result.clientEntryDeps = removeDependency(
				dependencies,
				result.clientEntryOut,
			)
			continue
		}

		updateRoutePathsForChunk(
			pathsBySourcePath,
			chunk.Src,
			cleanChunkOutPath,
			dependencies,
		)
	}

	return result
}

func collectCSSBundleFileNames(cssFiles []string) []string {
	cssBundleFileNames := make([]string, 0, len(cssFiles))
	for _, cssFile := range cssFiles {
		cssBundleFileNames = append(
			cssBundleFileNames,
			filepath.Base(cssFile),
		)
	}
	return cssBundleFileNames
}

func removeDependency(
	dependencies []string,
	dependencyToRemove string,
) []string {
	dependenciesWithoutTarget := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency == dependencyToRemove {
			continue
		}
		dependenciesWithoutTarget = append(
			dependenciesWithoutTarget,
			dependency,
		)
	}
	return dependenciesWithoutTarget
}

func updateRoutePathsForChunk(
	pathsBySourcePath map[string][]*vormaruntime.Path,
	chunkSourcePath string,
	chunkOutPath string,
	chunkDependencies []string,
) {
	for _, currentPath := range pathsBySourcePath[chunkSourcePath] {
		currentPath.OutPath = chunkOutPath
		currentPath.Deps = chunkDependencies
	}
}

func indexPathsBySourcePath(
	paths map[string]*vormaruntime.Path,
) map[string][]*vormaruntime.Path {
	pathsBySourcePath := make(map[string][]*vormaruntime.Path)
	for _, currentPath := range paths {
		pathsBySourcePath[currentPath.SrcPath] = append(
			pathsBySourcePath[currentPath.SrcPath],
			currentPath,
		)
	}
	return pathsBySourcePath
}

type stageTwoBuildIDDependencies struct {
	readHTMLTemplate  func(string) ([]byte, error)
	marshalPathsFile  func(any) ([]byte, error)
	summarizePublicFS func(fs.FS) ([]byte, error)
}

type stageTwoBuildIDExecutor struct {
	dependencies stageTwoBuildIDDependencies
}

var defaultStageTwoBuildIDExecutor = newStageTwoBuildIDExecutor(
	stageTwoBuildIDDependencies{},
)

func defaultStageTwoBuildIDDependencies() stageTwoBuildIDDependencies {
	return stageTwoBuildIDDependencies{
		readHTMLTemplate:  os.ReadFile,
		marshalPathsFile:  json.Marshal,
		summarizePublicFS: getFSSummaryHash,
	}
}

func normalizeStageTwoBuildIDDependencies(
	dependencies stageTwoBuildIDDependencies,
) stageTwoBuildIDDependencies {
	defaultDependencies := defaultStageTwoBuildIDDependencies()

	if dependencies.readHTMLTemplate == nil {
		dependencies.readHTMLTemplate = defaultDependencies.readHTMLTemplate
	}
	if dependencies.marshalPathsFile == nil {
		dependencies.marshalPathsFile = defaultDependencies.marshalPathsFile
	}
	if dependencies.summarizePublicFS == nil {
		dependencies.summarizePublicFS = defaultDependencies.summarizePublicFS
	}

	return dependencies
}

func newStageTwoBuildIDExecutor(
	dependencies stageTwoBuildIDDependencies,
) stageTwoBuildIDExecutor {
	return stageTwoBuildIDExecutor{
		dependencies: normalizeStageTwoBuildIDDependencies(dependencies),
	}
}

func computeStageTwoBuildID(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) (string, error) {
	return defaultStageTwoBuildIDExecutor.computeStageTwoBuildID(v, pathsFile)
}

func (executor stageTwoBuildIDExecutor) computeStageTwoBuildID(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) (string, error) {
	htmlTemplateContent, err := executor.dependencies.readHTMLTemplate(
		path.Join(v.Wave.PrivateStaticDir(), v.Config.HTMLTemplateLocation),
	)
	if err != nil {
		return "", fmt.Errorf("read HTML template: %w", err)
	}
	htmlContentHash := cryptoutil.Sha256Hash(htmlTemplateContent)

	asJSON, err := executor.dependencies.marshalPathsFile(pathsFile)
	if err != nil {
		return "", fmt.Errorf("marshal paths file: %w", err)
	}
	pathsFileJSONHash := cryptoutil.Sha256Hash(asJSON)

	publicFSSummaryHash, err := executor.dependencies.summarizePublicFS(
		os.DirFS(v.Wave.StaticPublicOutDir()),
	)
	if err != nil {
		return "", fmt.Errorf("get FS summary hash: %w", err)
	}

	fullHash := sha256.New()
	fullHash.Write(htmlContentHash)
	fullHash.Write(pathsFileJSONHash)
	fullHash.Write(publicFSSummaryHash)
	return base64.RawURLEncoding.EncodeToString(fullHash.Sum(nil)[:16]), nil
}

type stageTwoPathsWriteDependencies struct {
	marshalStageTwoPathsFile func(*runtimepaths.PathsFile) ([]byte, error)
	writeStageTwoPathsJSON   func(*vormaruntime.Vorma, []byte) error
}

type stageTwoPathsWriteExecutor struct {
	dependencies stageTwoPathsWriteDependencies
}

var defaultStageTwoPathsWriteExecutor = newStageTwoPathsWriteExecutor(
	stageTwoPathsWriteDependencies{},
)

func defaultStageTwoPathsWriteDependencies() stageTwoPathsWriteDependencies {
	return stageTwoPathsWriteDependencies{
		marshalStageTwoPathsFile: func(pathsFile *runtimepaths.PathsFile) ([]byte, error) {
			return json.MarshalIndent(pathsFile, "", "\t")
		},
		writeStageTwoPathsJSON: func(v *vormaruntime.Vorma, pathsAsJSON []byte) error {
			return writePathsJSONBytesToOutputPath(
				pathsOutputPath(v, runtimepaths.VormaPathsStageTwoJSONFileName),
				pathsAsJSON,
				pathsJSONWriteDependencies{
					makePathsOutputDirectory: os.MkdirAll,
					writePathsJSON:           writeFileAtomically,
				},
				"create stage-two paths output directory",
				"write stage-two paths JSON",
			)
		},
	}
}

func normalizeStageTwoPathsWriteDependencies(
	dependencies stageTwoPathsWriteDependencies,
) stageTwoPathsWriteDependencies {
	defaultDependencies := defaultStageTwoPathsWriteDependencies()

	if dependencies.marshalStageTwoPathsFile == nil {
		dependencies.marshalStageTwoPathsFile = defaultDependencies.marshalStageTwoPathsFile
	}
	if dependencies.writeStageTwoPathsJSON == nil {
		dependencies.writeStageTwoPathsJSON = defaultDependencies.writeStageTwoPathsJSON
	}

	return dependencies
}

func newStageTwoPathsWriteExecutor(
	dependencies stageTwoPathsWriteDependencies,
) stageTwoPathsWriteExecutor {
	return stageTwoPathsWriteExecutor{
		dependencies: normalizeStageTwoPathsWriteDependencies(dependencies),
	}
}

func toPathsFileStageTwo(
	v *vormaruntime.Vorma,
) (*runtimepaths.PathsFile, error) {
	viteManifest, err := viteutil.ReadManifest(v.Wave.ViteManifestLocation())
	if err != nil {
		return nil, fmt.Errorf("read vite manifest: %w", err)
	}

	paths := v.Paths()
	cleanClientEntry := filepath.Clean(v.Config.ClientEntry)
	manifestApplicationResult := applyViteManifestToPaths(
		viteManifest,
		paths,
		cleanClientEntry,
	)
	if err := validateStageTwoManifestCoverage(
		paths,
		cleanClientEntry,
		manifestApplicationResult,
	); err != nil {
		return nil, err
	}

	pathsFile := buildStageTwoPathsFile(
		v,
		paths,
		manifestApplicationResult,
	)

	buildID, err := computeStageTwoBuildID(v, pathsFile)
	if err != nil {
		return nil, err
	}

	applyBuildIDToPathsFile(pathsFile, buildID)
	return pathsFile, nil
}

func writePathsToDiskStageTwo(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) error {
	return defaultStageTwoPathsWriteExecutor.writePathsToDiskStageTwo(
		v,
		pathsFile,
	)
}

func (executor stageTwoPathsWriteExecutor) writePathsToDiskStageTwo(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) error {
	pathsAsJSON, err := executor.dependencies.marshalStageTwoPathsFile(
		pathsFile,
	)
	if err != nil {
		return fmt.Errorf("marshal stage-two paths file: %w", err)
	}

	if err := executor.dependencies.writeStageTwoPathsJSON(v, pathsAsJSON); err != nil {
		return fmt.Errorf("write stage-two paths JSON: %w", err)
	}
	return nil
}

func pathsOutputPath(v *vormaruntime.Vorma, fileName string) string {
	return filepath.Join(
		v.Wave.StaticPrivateOutDir(),
		runtimepaths.VormaOutDirname,
		fileName,
	)
}

func applyBuildIDToPathsFile(
	pathsFile *runtimepaths.PathsFile,
	buildID string,
) {
	pathsFile.BuildID = buildID
}

func applyBuildIDToVorma(v *vormaruntime.Vorma, buildID string) {
	commitRuntimeState(
		v,
		runtimeStateCommitInput{
			shouldCommitBuildID: true,
			buildID:             buildID,
		},
	)
}

func buildStageTwoPathsFile(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
	manifestApplicationResult viteManifestApplicationResult,
) *runtimepaths.PathsFile {
	return &runtimepaths.PathsFile{
		Stage:             "two",
		DepToCSSBundleMap: manifestApplicationResult.depToCSSBundleMap,
		Paths:             toRuntimePathsMap(paths),
		ClientEntrySrc:    v.Config.ClientEntry,
		ClientEntryOut:    manifestApplicationResult.clientEntryOut,
		ClientEntryDeps:   manifestApplicationResult.clientEntryDeps,
		RouteManifestFile: v.RouteManifestFile(),
	}
}

func validateStageTwoManifestCoverage(
	paths map[string]*vormaruntime.Path,
	cleanClientEntry string,
	manifestApplicationResult viteManifestApplicationResult,
) error {
	if strings.TrimSpace(manifestApplicationResult.clientEntryOut) == "" {
		return fmt.Errorf(
			"client entry chunk missing from Vite manifest for %q",
			cleanClientEntry,
		)
	}

	for routePattern, routePath := range paths {
		if routePath == nil || strings.TrimSpace(routePath.SrcPath) == "" {
			continue
		}
		if strings.TrimSpace(routePath.OutPath) != "" {
			continue
		}
		return fmt.Errorf(
			"route chunk missing from Vite manifest for %q (source: %q)",
			routePattern,
			routePath.SrcPath,
		)
	}

	return nil
}

type toolingBuildOptions = builder.BuildOpts

type runtimeBuildDependencies struct {
	runWaveViteProductionBuild func(*vormaruntime.Vorma) error
	runPostViteProductionBuild func(*vormaruntime.Vorma) error
	setWaveModeToDev           func()
	runWaveDevelopmentServer   func(*vormaruntime.Vorma) error
	runWaveProductionBuild     func(*vormaruntime.Vorma, toolingBuildOptions) error
}

type runtimeBuildToolingDependencies struct {
	newWaveBuilder         func(*vormaruntime.Vorma) runtimeWaveBuilder
	runWaveDevelopmentMode func(*vormaruntime.Vorma) error
}

type runtimeWaveBuilder interface {
	ViteProdBuild() error
	Build(toolingBuildOptions) error
	Close() error
}

type buildEntrypointDependencies struct {
	runBuildCommandFromCLI func(*vormaruntime.Vorma, []string, buildCommandHooks) error
	fatalfForBuildCommand  func(string, ...any)
}

type runtimeBuildToolingExecutor struct {
	dependencies runtimeBuildToolingDependencies
}

type runtimeBuildOperationExecutor struct {
	dependencies runtimeBuildDependencies
}

type buildEntrypointExecutor struct {
	dependencies buildEntrypointDependencies
}

type runtimeBuildExecutor struct {
	vorma                  *vormaruntime.Vorma
	runtimeBuildOperations runtimeBuildOperationExecutor
}

func defaultRuntimeBuildToolingDependencies() runtimeBuildToolingDependencies {
	return runtimeBuildToolingDependencies{
		newWaveBuilder: func(v *vormaruntime.Vorma) runtimeWaveBuilder {
			return builder.NewBuilder(
				configureBuildEnvironment(v),
				v.Wave.Logger(),
			)
		},
		runWaveDevelopmentMode: func(v *vormaruntime.Vorma) error {
			return devserver.RunDev(
				configureBuildEnvironment(v),
				v.Wave.Logger(),
			)
		},
	}
}

func normalizeRuntimeBuildToolingDependencies(
	dependencies runtimeBuildToolingDependencies,
) runtimeBuildToolingDependencies {
	defaultDependencies := defaultRuntimeBuildToolingDependencies()
	if dependencies.newWaveBuilder == nil {
		dependencies.newWaveBuilder = defaultDependencies.newWaveBuilder
	}
	if dependencies.runWaveDevelopmentMode == nil {
		dependencies.runWaveDevelopmentMode = defaultDependencies.runWaveDevelopmentMode
	}
	return dependencies
}

func newRuntimeBuildToolingExecutor(
	dependencies runtimeBuildToolingDependencies,
) runtimeBuildToolingExecutor {
	return runtimeBuildToolingExecutor{
		dependencies: normalizeRuntimeBuildToolingDependencies(dependencies),
	}
}

func defaultRuntimeBuildDependencies() runtimeBuildDependencies {
	return runtimeBuildDependencies{
		runWaveViteProductionBuild: func(v *vormaruntime.Vorma) error {
			return defaultRuntimeBuildToolingExecutor.runWaveViteProductionBuild(
				v,
			)
		},
		runPostViteProductionBuild: postViteProdBuild,
		setWaveModeToDev:           wave.SetModeToDev,
		runWaveDevelopmentServer: func(v *vormaruntime.Vorma) error {
			return defaultRuntimeBuildToolingExecutor.runWaveDevelopmentServer(
				v,
			)
		},
		runWaveProductionBuild: func(
			v *vormaruntime.Vorma,
			options toolingBuildOptions,
		) error {
			return defaultRuntimeBuildToolingExecutor.runWaveProductionBuild(
				v,
				options,
			)
		},
	}
}

func normalizeRuntimeBuildDependencies(
	dependencies runtimeBuildDependencies,
) runtimeBuildDependencies {
	defaultDependencies := defaultRuntimeBuildDependencies()
	if dependencies.runWaveViteProductionBuild == nil {
		dependencies.runWaveViteProductionBuild = defaultDependencies.runWaveViteProductionBuild
	}
	if dependencies.runPostViteProductionBuild == nil {
		dependencies.runPostViteProductionBuild = defaultDependencies.runPostViteProductionBuild
	}
	if dependencies.setWaveModeToDev == nil {
		dependencies.setWaveModeToDev = defaultDependencies.setWaveModeToDev
	}
	if dependencies.runWaveDevelopmentServer == nil {
		dependencies.runWaveDevelopmentServer = defaultDependencies.runWaveDevelopmentServer
	}
	if dependencies.runWaveProductionBuild == nil {
		dependencies.runWaveProductionBuild = defaultDependencies.runWaveProductionBuild
	}
	return dependencies
}

func newRuntimeBuildOperationExecutor(
	dependencies runtimeBuildDependencies,
) runtimeBuildOperationExecutor {
	return runtimeBuildOperationExecutor{
		dependencies: normalizeRuntimeBuildDependencies(dependencies),
	}
}

func defaultBuildEntrypointDependencies() buildEntrypointDependencies {
	return buildEntrypointDependencies{
		runBuildCommandFromCLI: runBuildCommand,
		fatalfForBuildCommand:  log.Fatalf,
	}
}

func normalizeBuildEntrypointDependencies(
	dependencies buildEntrypointDependencies,
) buildEntrypointDependencies {
	defaultDependencies := defaultBuildEntrypointDependencies()
	if dependencies.runBuildCommandFromCLI == nil {
		dependencies.runBuildCommandFromCLI = defaultDependencies.runBuildCommandFromCLI
	}
	if dependencies.fatalfForBuildCommand == nil {
		dependencies.fatalfForBuildCommand = defaultDependencies.fatalfForBuildCommand
	}
	return dependencies
}

func newBuildEntrypointExecutor(
	dependencies buildEntrypointDependencies,
) buildEntrypointExecutor {
	return buildEntrypointExecutor{
		dependencies: normalizeBuildEntrypointDependencies(dependencies),
	}
}

var defaultRuntimeBuildToolingExecutor = newRuntimeBuildToolingExecutor(
	runtimeBuildToolingDependencies{},
)

var defaultRuntimeBuildOperationExecutor = newRuntimeBuildOperationExecutor(
	runtimeBuildDependencies{},
)

var defaultBuildEntrypointExecutor = newBuildEntrypointExecutor(
	buildEntrypointDependencies{},
)

// Build parses flags and runs the build or dev server.
func Build(v *vorma.Vorma) {
	defaultBuildEntrypointExecutor.runBuildCommand(
		v.UnsafeRuntimeForFrameworkInternals(),
		os.Args[1:],
		defaultBuildCommandHooks(),
	)
}

func runBuildEntrypointWithDependencies(
	v *vormaruntime.Vorma,
	commandLineArgs []string,
	hooks buildCommandHooks,
	dependencies buildEntrypointDependencies,
) {
	newBuildEntrypointExecutor(
		dependencies,
	).runBuildCommand(v, commandLineArgs, hooks)
}

// runFullRuntimeBuild performs a full Vorma build.
func runFullRuntimeBuild(
	v *vormaruntime.Vorma,
	isDev bool,
	noBinary bool,
) error {
	return newRuntimeBuildExecutor(v).run(isDev, noBinary)
}

func buildWithRuntimeBuildDependencies(
	v *vormaruntime.Vorma,
	isDev bool,
	noBinary bool,
	dependencies runtimeBuildDependencies,
) error {
	return newRuntimeBuildExecutorWithDependencies(
		v,
		dependencies,
	).run(isDev, noBinary)
}

func (entrypointExecutor buildEntrypointExecutor) runBuildCommand(
	v *vormaruntime.Vorma,
	commandLineArgs []string,
	hooks buildCommandHooks,
) {
	if err := entrypointExecutor.dependencies.runBuildCommandFromCLI(v, commandLineArgs, hooks); err != nil {
		entrypointExecutor.dependencies.fatalfForBuildCommand("%v", err)
	}
}

func runWaveViteProductionBuildWithToolingDependencies(
	v *vormaruntime.Vorma,
	dependencies runtimeBuildToolingDependencies,
) error {
	return newRuntimeBuildToolingExecutor(
		dependencies,
	).runWaveViteProductionBuild(v)
}

func runWaveDevelopmentServerWithToolingDependencies(
	v *vormaruntime.Vorma,
	dependencies runtimeBuildToolingDependencies,
) error {
	return newRuntimeBuildToolingExecutor(
		dependencies,
	).runWaveDevelopmentServer(v)
}

func runWaveProductionBuildWithToolingDependencies(
	v *vormaruntime.Vorma,
	options toolingBuildOptions,
	dependencies runtimeBuildToolingDependencies,
) error {
	return newRuntimeBuildToolingExecutor(
		dependencies,
	).runWaveProductionBuild(v, options)
}

func (toolingExecutor runtimeBuildToolingExecutor) runWaveViteProductionBuild(
	v *vormaruntime.Vorma,
) error {
	return toolingExecutor.runWithRuntimeWaveBuilder(
		v,
		func(builder runtimeWaveBuilder) error {
			return builder.ViteProdBuild()
		},
	)
}

func (toolingExecutor runtimeBuildToolingExecutor) runWaveDevelopmentServer(
	v *vormaruntime.Vorma,
) error {
	return toolingExecutor.dependencies.runWaveDevelopmentMode(v)
}

func (toolingExecutor runtimeBuildToolingExecutor) runWaveProductionBuild(
	v *vormaruntime.Vorma,
	options toolingBuildOptions,
) error {
	return toolingExecutor.runWithRuntimeWaveBuilder(
		v,
		func(builder runtimeWaveBuilder) error {
			return builder.Build(options)
		},
	)
}

func (toolingExecutor runtimeBuildToolingExecutor) runWithRuntimeWaveBuilder(
	v *vormaruntime.Vorma,
	runWithBuilder func(runtimeWaveBuilder) error,
) (operationErr error) {
	builder := toolingExecutor.dependencies.newWaveBuilder(v)
	return runWithClosableResource(
		builder,
		"close wave builder",
		runWithBuilder,
	)
}

func runProdHookPostProcessing(v *vormaruntime.Vorma) error {
	return defaultRuntimeBuildOperationExecutor.runProdHookPostProcessing(v)
}

func runProdHookPostProcessingWithRuntimeBuildDependencies(
	v *vormaruntime.Vorma,
	dependencies runtimeBuildDependencies,
) error {
	return newRuntimeBuildOperationExecutor(
		dependencies,
	).runProdHookPostProcessing(v)
}

func (runtimeBuildOperations runtimeBuildOperationExecutor) runProdHookPostProcessing(
	v *vormaruntime.Vorma,
) error {
	if err := runtimeBuildOperations.dependencies.runWaveViteProductionBuild(v); err != nil {
		return fmt.Errorf("vite production build failed: %w", err)
	}

	if err := runtimeBuildOperations.dependencies.runPostViteProductionBuild(v); err != nil {
		return fmt.Errorf("post Vite production build failed: %w", err)
	}

	return nil
}

func (runtimeBuildOperations runtimeBuildOperationExecutor) prepareDevBuildRuntime(
	v *vormaruntime.Vorma,
) {
	runtimeBuildOperations.dependencies.setWaveModeToDev()
	// Set isDev on this process's Vorma instance so callbacks
	// (like rebuildRoutesOnly) can run in the dev server process.
	commitRuntimeState(
		v,
		runtimeStateCommitInput{
			shouldCommitIsDev: true,
			isDev:             true,
		},
	)
}

func productionBuildOptions(noBinary bool) toolingBuildOptions {
	return toolingBuildOptions{
		CompileGo: !noBinary,
		IsDev:     false,
		IsRebuild: false,
	}
}

func newRuntimeBuildExecutor(v *vormaruntime.Vorma) runtimeBuildExecutor {
	return runtimeBuildExecutor{
		vorma:                  v,
		runtimeBuildOperations: defaultRuntimeBuildOperationExecutor,
	}
}

func newRuntimeBuildExecutorWithDependencies(
	v *vormaruntime.Vorma,
	dependencies runtimeBuildDependencies,
) runtimeBuildExecutor {
	return runtimeBuildExecutor{
		vorma:                  v,
		runtimeBuildOperations: newRuntimeBuildOperationExecutor(dependencies),
	}
}

func (buildExecutor runtimeBuildExecutor) run(
	isDev bool,
	noBinary bool,
) error {
	if isDev {
		return buildExecutor.runDevelopmentMode()
	}
	return buildExecutor.runProductionMode(noBinary)
}

func (buildExecutor runtimeBuildExecutor) runDevelopmentMode() error {
	buildExecutor.runtimeBuildOperations.prepareDevBuildRuntime(
		buildExecutor.vorma,
	)
	return buildExecutor.runtimeBuildOperations.dependencies.runWaveDevelopmentServer(
		buildExecutor.vorma,
	)
}

func (buildExecutor runtimeBuildExecutor) runProductionMode(
	noBinary bool,
) error {
	// Production Build
	//
	// The build flow is:
	// 1. wb.Build() sets up dist directory, then runs ProdBuildHook
	// 2. ProdBuildHook (e.g., "go run ./cmd/build --hook") runs in subprocess:
	//    a. buildInner() - parses routes, writes artifacts
	//    b. ViteProdBuild() - runs Vite
	//    c. postViteProdBuild() - processes Vite output
	// 3. wb.Build() compiles the Go binary
	//
	// All Vorma logic runs in the subprocess (via --hook) so state is shared.
	return buildExecutor.runtimeBuildOperations.dependencies.runWaveProductionBuild(
		buildExecutor.vorma,
		productionBuildOptions(noBinary),
	)
}
