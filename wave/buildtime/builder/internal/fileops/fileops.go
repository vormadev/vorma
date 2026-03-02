// Package fileops provides deterministic file hashing and atomic write helpers
// for Wave build artifacts.
//
// Builder phases share these primitives to keep artifact change detection and
// write semantics consistent across outputs.
package fileops

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/artifactio"
	"github.com/vormadev/vorma/wave/waveartifacts"
)

// HashFile computes a content-addressed filename for one source file.
func HashFile(
	filePath string,
	originalName string,
) (string, error) {
	file, openError := os.Open(filePath)
	if openError != nil {
		return "", openError
	}
	defer file.Close()

	hasher := sha256.New()
	_, _ = hasher.Write([]byte(originalName))
	if _, copyError := io.Copy(hasher, file); copyError != nil {
		return "", copyError
	}
	return formatHashedArtifactName(hasher.Sum(nil), originalName), nil
}

// HashBytes computes a content-addressed filename for in-memory bytes.
func HashBytes(content []byte, originalName string) string {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(originalName))
	_, _ = hasher.Write(content)
	return formatHashedArtifactName(hasher.Sum(nil), originalName)
}

func formatHashedArtifactName(
	hashSum []byte,
	originalName string,
) string {
	hashPrefix := fmt.Sprintf("%x", hashSum)[:12]
	extension := filepath.Ext(originalName)
	baseName := strings.TrimSuffix(originalName, extension)
	return waveartifacts.ApplyWaveFileOutputPrefix(
		fmt.Sprintf(
			"%s_%s%s",
			baseName,
			hashPrefix,
			extension,
		),
	)
}

// HashedArtifactPublishOptions controls publish + ref-file update for hashed assets.
type HashedArtifactPublishOptions struct {
	Log                   *slog.Logger
	OutputDirectoryPath   string
	RefFilePath           string
	DesiredHashedFileName string
	Content               []byte
	GlobPattern           string
}

// PublishHashedArtifactWithRef writes hashed artifact, updates ref file, and cleans stale artifacts.
func PublishHashedArtifactWithRef(
	options HashedArtifactPublishOptions,
) (string, error) {
	if mkdirError := os.MkdirAll(options.OutputDirectoryPath, 0o755); mkdirError != nil {
		return "", fmt.Errorf("mkdir output directory: %w", mkdirError)
	}

	desiredArtifactPath := filepath.Join(
		options.OutputDirectoryPath,
		options.DesiredHashedFileName,
	)
	previousArtifactFileName, hasRefFile, readRefError := readHashedArtifactRefFileName(
		options.RefFilePath,
	)
	if readRefError != nil {
		return "", readRefError
	}

	hasValidRef := hasRefFile && previousArtifactFileName != ""
	if hasValidRef &&
		previousArtifactFileName == options.DesiredHashedFileName {
		if _, statError := os.Stat(desiredArtifactPath); statError == nil {
			return options.DesiredHashedFileName, nil
		} else if !os.IsNotExist(statError) {
			return "", statError
		}
	}

	if hasValidRef &&
		previousArtifactFileName != options.DesiredHashedFileName {
		removeHashedArtifactIfPresent(
			options.Log,
			filepath.Join(
				options.OutputDirectoryPath,
				previousArtifactFileName,
			),
		)
	}
	if !hasValidRef {
		cleanupOldHashedArtifactsWhenRefFileMissing(
			options.Log,
			options.OutputDirectoryPath,
			options.GlobPattern,
			options.DesiredHashedFileName,
		)
	}

	if writeArtifactError := artifactio.WriteFileAtomically(
		desiredArtifactPath,
		options.Content,
		0o644,
	); writeArtifactError != nil {
		return "", writeArtifactError
	}
	if writeRefError := artifactio.WriteFileAtomically(
		options.RefFilePath,
		[]byte(options.DesiredHashedFileName),
		0o644,
	); writeRefError != nil {
		return "", fmt.Errorf("write hashed artifact ref: %w", writeRefError)
	}

	return options.DesiredHashedFileName, nil
}

func readHashedArtifactRefFileName(
	refFilePath string,
) (string, bool, error) {
	refBytes, readError := os.ReadFile(refFilePath)
	if readError != nil {
		if os.IsNotExist(readError) {
			return "", false, nil
		}
		return "", false, readError
	}
	return strings.TrimSpace(string(refBytes)), true, nil
}

func removeHashedArtifactIfPresent(
	log *slog.Logger,
	artifactPath string,
) {
	if removeError := os.Remove(artifactPath); removeError != nil &&
		!os.IsNotExist(removeError) {
		if log != nil {
			log.Warn(
				"failed to remove old hashed artifact",
				"file",
				artifactPath,
				"error",
				removeError,
			)
		}
	}
}

func cleanupOldHashedArtifactsWhenRefFileMissing(
	log *slog.Logger,
	outputDirectoryPath string,
	globPattern string,
	desiredHashedFileName string,
) {
	if strings.TrimSpace(globPattern) == "" {
		return
	}

	staleArtifactPaths, globError := filepath.Glob(
		filepath.Join(outputDirectoryPath, globPattern),
	)
	if globError != nil {
		if log != nil {
			log.Warn(
				"failed to glob old hashed artifacts",
				"error",
				globError,
			)
		}
		return
	}

	for _, staleArtifactPath := range staleArtifactPaths {
		if filepath.Base(staleArtifactPath) == desiredHashedFileName {
			continue
		}
		if removeError := os.Remove(staleArtifactPath); removeError != nil &&
			!os.IsNotExist(removeError) {
			if log != nil {
				log.Warn(
					"failed to remove old hashed artifact",
					"file",
					staleArtifactPath,
					"error",
					removeError,
				)
			}
		}
	}
}

func writeFileAtomicBytes(path string, data []byte) error {
	directoryPath := filepath.Dir(path)
	if mkdirError := os.MkdirAll(directoryPath, 0o755); mkdirError != nil {
		return mkdirError
	}
	return artifactio.WriteFileAtomically(path, data, 0o644)
}

// WriteFileAtomicBytes writes bytes atomically to path.
func WriteFileAtomicBytes(path string, data []byte) error {
	return writeFileAtomicBytes(path, data)
}

// WriteFileAtomicBytesIfChanged writes bytes atomically only when content differs.
func WriteFileAtomicBytesIfChanged(path string, data []byte) (bool, error) {
	existingData, readError := os.ReadFile(path)
	if readError == nil {
		if bytes.Equal(existingData, data) {
			return false, nil
		}
	} else if !os.IsNotExist(readError) {
		return false, readError
	}

	if writeError := writeFileAtomicBytes(path, data); writeError != nil {
		return false, writeError
	}
	return true, nil
}

// ComputeFileContentHash computes SHA-256 content hash for a file.
func ComputeFileContentHash(path string) (string, error) {
	file, openError := os.Open(path)
	if openError != nil {
		return "", openError
	}
	defer file.Close()

	hasher := sha256.New()
	if _, copyError := io.Copy(hasher, file); copyError != nil {
		return "", copyError
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
