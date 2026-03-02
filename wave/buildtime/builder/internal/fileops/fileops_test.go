package fileops_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/buildtime/builder/internal/fileops"
	"github.com/vormadev/vorma/wave/waveartifacts"
)

func TestHashFile_MatchesHashBytesForSameInputs(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "styles.css")
	content := []byte("body { color: red; }")
	if writeError := os.WriteFile(sourcePath, content, 0o644); writeError != nil {
		t.Fatalf("write source file: %v", writeError)
	}

	hashedFromFile, hashFileError := fileops.HashFile(sourcePath, "styles.css")
	if hashFileError != nil {
		t.Fatalf("HashFile returned error: %v", hashFileError)
	}
	hashedFromBytes := fileops.HashBytes(content, "styles.css")

	if hashedFromFile != hashedFromBytes {
		t.Fatalf(
			"expected HashFile and HashBytes to match for same content/name: file=%q bytes=%q",
			hashedFromFile,
			hashedFromBytes,
		)
	}
	if !strings.HasPrefix(hashedFromFile, waveartifacts.HashedOutputPrefix) {
		t.Fatalf(
			"expected hashed filename prefix %q, got %q",
			waveartifacts.HashedOutputPrefix,
			hashedFromFile,
		)
	}
}

func TestComputeFileContentHash_Deterministic(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "asset.txt")
	content := []byte("deterministic-content")
	if writeError := os.WriteFile(sourcePath, content, 0o644); writeError != nil {
		t.Fatalf("write source file: %v", writeError)
	}

	firstHash, firstHashError := fileops.ComputeFileContentHash(sourcePath)
	if firstHashError != nil {
		t.Fatalf("first ComputeFileContentHash error: %v", firstHashError)
	}
	secondHash, secondHashError := fileops.ComputeFileContentHash(sourcePath)
	if secondHashError != nil {
		t.Fatalf("second ComputeFileContentHash error: %v", secondHashError)
	}
	if firstHash != secondHash {
		t.Fatalf(
			"expected deterministic hash, first=%q second=%q",
			firstHash,
			secondHash,
		)
	}
}

func TestPublishHashedArtifactWithRef_ReplacesPreviousArtifact(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, waveartifacts.PublicDirname)
	refPath := filepath.Join(root, "public-filemap.ref")

	if mkdirError := os.MkdirAll(outputDir, 0o755); mkdirError != nil {
		t.Fatalf("mkdir output dir: %v", mkdirError)
	}

	previousArtifactName := "__hash_old_123.js"
	previousArtifactPath := filepath.Join(outputDir, previousArtifactName)
	if writePreviousError := os.WriteFile(
		previousArtifactPath,
		[]byte("old"),
		0o644,
	); writePreviousError != nil {
		t.Fatalf("write previous artifact: %v", writePreviousError)
	}
	if writeRefError := os.WriteFile(refPath, []byte(previousArtifactName), 0o644); writeRefError != nil {
		t.Fatalf("write ref file: %v", writeRefError)
	}

	nextArtifactName := "__hash_new_456.js"
	writtenName, publishError := fileops.PublishHashedArtifactWithRef(
		fileops.HashedArtifactPublishOptions{
			OutputDirectoryPath:   outputDir,
			RefFilePath:           refPath,
			DesiredHashedFileName: nextArtifactName,
			Content:               []byte("new"),
			GlobPattern:           "__hash_*.js",
		},
	)
	if publishError != nil {
		t.Fatalf(
			"PublishHashedArtifactWithRef returned error: %v",
			publishError,
		)
	}
	if writtenName != nextArtifactName {
		t.Fatalf(
			"written artifact name = %q, want %q",
			writtenName,
			nextArtifactName,
		)
	}
	if _, statPreviousError := os.Stat(previousArtifactPath); !os.IsNotExist(
		statPreviousError,
	) {
		t.Fatalf(
			"expected previous artifact to be removed, stat error: %v",
			statPreviousError,
		)
	}

	nextArtifactPath := filepath.Join(outputDir, nextArtifactName)
	nextBytes, readNextError := os.ReadFile(nextArtifactPath)
	if readNextError != nil {
		t.Fatalf("read next artifact: %v", readNextError)
	}
	if string(nextBytes) != "new" {
		t.Fatalf(
			"next artifact content = %q, want %q",
			string(nextBytes),
			"new",
		)
	}

	refBytes, readRefError := os.ReadFile(refPath)
	if readRefError != nil {
		t.Fatalf("read ref file: %v", readRefError)
	}
	if strings.TrimSpace(string(refBytes)) != nextArtifactName {
		t.Fatalf(
			"ref file content = %q, want %q",
			strings.TrimSpace(string(refBytes)),
			nextArtifactName,
		)
	}
}

func TestPublishHashedArtifactWithRef_NoOpWhenRefAndArtifactAlreadyMatch(
	t *testing.T,
) {
	root := t.TempDir()
	outputDir := filepath.Join(root, waveartifacts.PublicDirname)
	refPath := filepath.Join(root, "public-filemap.ref")
	if mkdirError := os.MkdirAll(outputDir, 0o755); mkdirError != nil {
		t.Fatalf("mkdir output dir: %v", mkdirError)
	}

	artifactName := "__hash_same_111.js"
	artifactPath := filepath.Join(outputDir, artifactName)
	if writeArtifactError := os.WriteFile(artifactPath, []byte("existing"), 0o644); writeArtifactError != nil {
		t.Fatalf("write artifact: %v", writeArtifactError)
	}
	if writeRefError := os.WriteFile(refPath, []byte(artifactName), 0o644); writeRefError != nil {
		t.Fatalf("write ref file: %v", writeRefError)
	}

	writtenName, publishError := fileops.PublishHashedArtifactWithRef(
		fileops.HashedArtifactPublishOptions{
			OutputDirectoryPath:   outputDir,
			RefFilePath:           refPath,
			DesiredHashedFileName: artifactName,
			Content: []byte(
				"new-content-that-should-not-overwrite",
			),
			GlobPattern: "__hash_*.js",
		},
	)
	if publishError != nil {
		t.Fatalf(
			"PublishHashedArtifactWithRef returned error: %v",
			publishError,
		)
	}
	if writtenName != artifactName {
		t.Fatalf(
			"written artifact name = %q, want %q",
			writtenName,
			artifactName,
		)
	}

	contentBytes, readError := os.ReadFile(artifactPath)
	if readError != nil {
		t.Fatalf("read artifact: %v", readError)
	}
	if string(contentBytes) != "existing" {
		t.Fatalf(
			"expected existing artifact content to remain unchanged, got %q",
			string(contentBytes),
		)
	}
}

func TestWriteFileAtomicBytesIfChanged_DetectsNoChange(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "artifact.txt")
	if writeError := os.WriteFile(targetPath, []byte("same"), 0o644); writeError != nil {
		t.Fatalf("write target file: %v", writeError)
	}

	changed, writeError := fileops.WriteFileAtomicBytesIfChanged(
		targetPath,
		[]byte("same"),
	)
	if writeError != nil {
		t.Fatalf("WriteFileAtomicBytesIfChanged returned error: %v", writeError)
	}
	if changed {
		t.Fatal("expected changed=false when bytes are identical")
	}

	changed, writeError = fileops.WriteFileAtomicBytesIfChanged(
		targetPath,
		[]byte("updated"),
	)
	if writeError != nil {
		t.Fatalf("WriteFileAtomicBytesIfChanged returned error: %v", writeError)
	}
	if !changed {
		t.Fatal("expected changed=true when bytes differ")
	}
}
