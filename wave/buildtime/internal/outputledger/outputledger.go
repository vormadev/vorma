// Package outputledger provides a Wave-owned on-disk build-output ledger and
// final reconciliation sweep helpers.
//
// Wave and framework build stages append the files they own for the current
// build into one ledger. A final sweep can then delete stale files under
// dist/static assets/internal that were not recorded for this build.
package outputledger

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ledgerFileWriteMode      fs.FileMode = 0o644
	ledgerDirectoryWriteMode fs.FileMode = 0o755
)

// Ledger stores and reconciles build outputs under one dist/static root.
type Ledger struct {
	staticRootPath string
	ledgerFilePath string
}

// New returns a build-output ledger bound to one dist/static root.
func New(staticRootPath string, ledgerFilePath string) *Ledger {
	return &Ledger{
		staticRootPath: filepath.Clean(staticRootPath),
		ledgerFilePath: filepath.Clean(ledgerFilePath),
	}
}

// Reset clears ledger content for a new build.
func (ledger *Ledger) Reset() error {
	if validationError := ledger.validate(); validationError != nil {
		return validationError
	}
	if mkdirError := os.MkdirAll(
		filepath.Dir(ledger.ledgerFilePath),
		ledgerDirectoryWriteMode,
	); mkdirError != nil {
		return fmt.Errorf("create ledger directory: %w", mkdirError)
	}
	if writeError := os.WriteFile(
		ledger.ledgerFilePath,
		nil,
		ledgerFileWriteMode,
	); writeError != nil {
		return fmt.Errorf("reset build output ledger: %w", writeError)
	}
	return nil
}

// RecordPaths appends absolute output file paths to the ledger.
func (ledger *Ledger) RecordPaths(outputPaths []string) error {
	if validationError := ledger.validate(); validationError != nil {
		return validationError
	}

	relativePaths := make([]string, 0, len(outputPaths))
	for _, outputPath := range outputPaths {
		relativePath, shouldRecord, resolveError := ledger.relativePathForStatic(
			outputPath,
		)
		if resolveError != nil {
			return resolveError
		}
		if !shouldRecord {
			continue
		}
		relativePaths = append(relativePaths, relativePath)
	}
	if len(relativePaths) == 0 {
		return nil
	}

	sort.Strings(relativePaths)
	uniqueRelativePaths := make([]string, 0, len(relativePaths))
	for _, relativePath := range relativePaths {
		if len(uniqueRelativePaths) > 0 &&
			uniqueRelativePaths[len(uniqueRelativePaths)-1] == relativePath {
			continue
		}
		uniqueRelativePaths = append(uniqueRelativePaths, relativePath)
	}

	if mkdirError := os.MkdirAll(
		filepath.Dir(ledger.ledgerFilePath),
		ledgerDirectoryWriteMode,
	); mkdirError != nil {
		return fmt.Errorf("create ledger directory: %w", mkdirError)
	}
	ledgerFile, openError := os.OpenFile(
		ledger.ledgerFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		ledgerFileWriteMode,
	)
	if openError != nil {
		return fmt.Errorf("open build output ledger for append: %w", openError)
	}
	defer ledgerFile.Close()

	payloadBuilder := &strings.Builder{}
	for _, relativePath := range uniqueRelativePaths {
		payloadBuilder.WriteString(relativePath)
		payloadBuilder.WriteByte('\n')
	}
	if _, writeError := ledgerFile.WriteString(payloadBuilder.String()); writeError != nil {
		return fmt.Errorf("append build output ledger: %w", writeError)
	}
	return nil
}

// LoadRecordedRelativePaths reads ledger content as a normalized relative-path
// set rooted at dist/static.
func (ledger *Ledger) LoadRecordedRelativePaths() (map[string]struct{}, error) {
	if validationError := ledger.validate(); validationError != nil {
		return nil, validationError
	}

	ledgerFile, openError := os.Open(ledger.ledgerFilePath)
	if openError != nil {
		if errors.Is(openError, os.ErrNotExist) {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf("open build output ledger: %w", openError)
	}
	defer ledgerFile.Close()

	recordedRelativePaths := make(map[string]struct{})
	scanner := bufio.NewScanner(ledgerFile)
	for scanner.Scan() {
		recordedPath := normalizeRecordedRelativePath(scanner.Text())
		if recordedPath == "" || !isSweepTargetRelativePath(recordedPath) {
			continue
		}
		recordedRelativePaths[recordedPath] = struct{}{}
	}
	if scanError := scanner.Err(); scanError != nil {
		return nil, fmt.Errorf("scan build output ledger: %w", scanError)
	}
	return recordedRelativePaths, nil
}

// SweepUnrecordedFiles deletes files under the supplied directories that were
// not recorded in the current ledger.
func (ledger *Ledger) SweepUnrecordedFiles(
	directoriesToSweep []string,
) error {
	recordedRelativePaths, loadError := ledger.LoadRecordedRelativePaths()
	if loadError != nil {
		return loadError
	}

	ledgerRelativePath, shouldKeepLedger, resolveLedgerRelativeError := ledger.relativePathForStatic(
		ledger.ledgerFilePath,
	)
	if resolveLedgerRelativeError != nil {
		return resolveLedgerRelativeError
	}
	if shouldKeepLedger {
		recordedRelativePaths[ledgerRelativePath] = struct{}{}
	}

	for _, directoryToSweep := range directoriesToSweep {
		trimmedDirectoryToSweep := strings.TrimSpace(directoryToSweep)
		if trimmedDirectoryToSweep == "" {
			continue
		}
		if sweepError := ledger.sweepOneDirectory(
			trimmedDirectoryToSweep,
			recordedRelativePaths,
		); sweepError != nil {
			return sweepError
		}
		if cleanupError := removeEmptyDirectoriesUnderRoot(
			trimmedDirectoryToSweep,
		); cleanupError != nil {
			return cleanupError
		}
	}
	return nil
}

func (ledger *Ledger) validate() error {
	if ledger == nil {
		return errors.New("build output ledger is nil")
	}
	if strings.TrimSpace(ledger.staticRootPath) == "" {
		return errors.New("build output ledger static root path is empty")
	}
	if strings.TrimSpace(ledger.ledgerFilePath) == "" {
		return errors.New("build output ledger file path is empty")
	}
	return nil
}

func (ledger *Ledger) sweepOneDirectory(
	directoryToSweep string,
	recordedRelativePaths map[string]struct{},
) error {
	_, statError := os.Stat(directoryToSweep)
	if statError != nil {
		if errors.Is(statError, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat sweep directory %q: %w", directoryToSweep, statError)
	}

	return filepath.WalkDir(
		directoryToSweep,
		func(path string, directoryEntry fs.DirEntry, walkError error) error {
			if walkError != nil {
				if errors.Is(walkError, os.ErrNotExist) {
					return nil
				}
				return walkError
			}
			if directoryEntry.IsDir() {
				return nil
			}

			relativePath, shouldCheckPath, resolveError := ledger.relativePathForStatic(
				path,
			)
			if resolveError != nil {
				return resolveError
			}
			if !shouldCheckPath {
				return nil
			}
			if _, keep := recordedRelativePaths[relativePath]; keep {
				return nil
			}

			if removeError := os.Remove(path); removeError != nil &&
				!errors.Is(removeError, os.ErrNotExist) {
				return fmt.Errorf(
					"remove stale build output %q: %w",
					path,
					removeError,
				)
			}
			return nil
		},
	)
}

func removeEmptyDirectoriesUnderRoot(rootPath string) error {
	_, rootStatError := os.Stat(rootPath)
	if rootStatError != nil {
		if errors.Is(rootStatError, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf(
			"stat root directory %q for empty-dir cleanup: %w",
			rootPath,
			rootStatError,
		)
	}

	var directories []string
	if walkError := filepath.WalkDir(
		rootPath,
		func(path string, directoryEntry fs.DirEntry, walkError error) error {
			if walkError != nil {
				if errors.Is(walkError, os.ErrNotExist) {
					return nil
				}
				return walkError
			}
			if directoryEntry.IsDir() {
				directories = append(directories, path)
			}
			return nil
		},
	); walkError != nil {
		return walkError
	}

	rootPathClean := filepath.Clean(rootPath)
	sort.Slice(directories, func(i int, j int) bool {
		return len(directories[i]) > len(directories[j])
	})
	for _, directoryPath := range directories {
		if filepath.Clean(directoryPath) == rootPathClean {
			continue
		}
		directoryEntries, readError := os.ReadDir(directoryPath)
		if readError != nil {
			if errors.Is(readError, os.ErrNotExist) {
				continue
			}
			return readError
		}
		if len(directoryEntries) > 0 {
			continue
		}
		if removeError := os.Remove(directoryPath); removeError != nil &&
			!errors.Is(removeError, os.ErrNotExist) {
			return removeError
		}
	}
	return nil
}

func (ledger *Ledger) relativePathForStatic(
	absolutePathOrRelativePath string,
) (string, bool, error) {
	absoluteStaticRootPath, staticRootError := filepath.Abs(ledger.staticRootPath)
	if staticRootError != nil {
		return "", false, fmt.Errorf(
			"resolve absolute static root path %q: %w",
			ledger.staticRootPath,
			staticRootError,
		)
	}

	absolutePath, absolutePathError := filepath.Abs(absolutePathOrRelativePath)
	if absolutePathError != nil {
		return "", false, fmt.Errorf(
			"resolve absolute output path %q: %w",
			absolutePathOrRelativePath,
			absolutePathError,
		)
	}

	relativePath, relativePathError := filepath.Rel(
		absoluteStaticRootPath,
		absolutePath,
	)
	if relativePathError != nil {
		return "", false, fmt.Errorf(
			"resolve build-output relative path for %q: %w",
			absolutePathOrRelativePath,
			relativePathError,
		)
	}

	normalizedRelativePath := normalizeRecordedRelativePath(relativePath)
	if normalizedRelativePath == "" ||
		normalizedRelativePath == "." ||
		normalizedRelativePath == ".." ||
		strings.HasPrefix(normalizedRelativePath, "../") {
		return "", false, nil
	}
	if !isSweepTargetRelativePath(normalizedRelativePath) {
		return "", false, nil
	}
	return normalizedRelativePath, true, nil
}

func normalizeRecordedRelativePath(relativePath string) string {
	trimmedRelativePath := strings.TrimSpace(relativePath)
	if trimmedRelativePath == "" {
		return ""
	}

	normalizedRelativePath := filepath.ToSlash(filepath.Clean(trimmedRelativePath))
	if normalizedRelativePath == "." {
		return ""
	}
	return normalizedRelativePath
}

func isSweepTargetRelativePath(relativePath string) bool {
	return strings.HasPrefix(relativePath, "assets/") ||
		strings.HasPrefix(relativePath, "internal/")
}
