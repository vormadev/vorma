package tooling

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type hashedArtifactPublishOptions struct {
	log                   *slog.Logger
	outputDirectoryPath   string
	refFilePath           string
	desiredHashedFileName string
	content               []byte
	globPattern           string
}

func publishHashedArtifactWithRef(
	opts hashedArtifactPublishOptions,
) (string, error) {
	if err := os.MkdirAll(opts.outputDirectoryPath, 0o755); err != nil {
		return "", fmt.Errorf("mkdir output directory: %w", err)
	}

	desiredOutputPath := filepath.Join(
		opts.outputDirectoryPath,
		opts.desiredHashedFileName,
	)
	previousHashedFileName, hasPreviousRefFile, readRefError := readHashedArtifactRefFileName(
		opts.refFilePath,
	)
	if readRefError != nil {
		return "", readRefError
	}
	hasValidPreviousRefFile := hasPreviousRefFile &&
		previousHashedFileName != ""

	if hasValidPreviousRefFile &&
		previousHashedFileName == opts.desiredHashedFileName {
		if _, statError := os.Stat(desiredOutputPath); statError == nil {
			return opts.desiredHashedFileName, nil
		} else if !os.IsNotExist(statError) {
			return "", statError
		}
	}

	if hasValidPreviousRefFile &&
		previousHashedFileName != opts.desiredHashedFileName {
		removeHashedArtifactIfPresent(
			opts.log,
			filepath.Join(opts.outputDirectoryPath, previousHashedFileName),
		)
	}
	if !hasValidPreviousRefFile {
		cleanupOldHashedArtifactsWhenRefFileMissing(
			opts.log,
			opts.outputDirectoryPath,
			opts.globPattern,
			opts.desiredHashedFileName,
		)
	}

	if _, writeError := writeFileAtomicBytesIfChanged(desiredOutputPath, opts.content); writeError != nil {
		return "", writeError
	}

	if _, writeError := writeFileAtomicBytesIfChanged(opts.refFilePath, []byte(opts.desiredHashedFileName)); writeError != nil {
		return "", fmt.Errorf("write hashed artifact ref: %w", writeError)
	}

	return opts.desiredHashedFileName, nil
}

func readHashedArtifactRefFileName(
	refFilePath string,
) (string, bool, error) {
	existingRefData, readError := os.ReadFile(refFilePath)
	if readError != nil {
		if os.IsNotExist(readError) {
			return "", false, nil
		}
		return "", false, readError
	}

	return strings.TrimSpace(string(existingRefData)), true, nil
}

func removeHashedArtifactIfPresent(
	logger *slog.Logger,
	artifactPath string,
) {
	if removeError := os.Remove(artifactPath); removeError != nil &&
		!os.IsNotExist(removeError) {
		if logger != nil {
			logger.Warn(
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
	logger *slog.Logger,
	outputDirectoryPath string,
	globPattern string,
	desiredHashedFileName string,
) {
	oldFiles, globError := filepath.Glob(
		filepath.Join(outputDirectoryPath, globPattern),
	)
	if globError != nil {
		if logger != nil {
			logger.Warn(
				"failed to glob old hashed artifacts",
				"error",
				globError,
			)
		}
		return
	}

	for _, oldFilePath := range oldFiles {
		if filepath.Base(oldFilePath) == desiredHashedFileName {
			continue
		}
		if removeError := os.Remove(oldFilePath); removeError != nil {
			if logger != nil {
				logger.Warn(
					"failed to remove old hashed artifact",
					"file",
					oldFilePath,
					"error",
					removeError,
				)
			}
		}
	}
}
