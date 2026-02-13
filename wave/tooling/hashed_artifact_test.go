package tooling

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishHashedArtifactWithRef_RefMissingCleansStaleArtifacts(t *testing.T) {
	root := t.TempDir()
	outputDirectoryPath := filepath.Join(root, "assets")
	refFilePath := filepath.Join(root, "assets.ref")

	if err := os.MkdirAll(outputDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating output directory: %v", err)
	}

	staleArtifactPath := filepath.Join(outputDirectoryPath, "styles-old.css")
	if err := os.WriteFile(staleArtifactPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("failed writing stale artifact: %v", err)
	}

	desiredHashedFileName := "styles-new.css"
	desiredContent := []byte("new")
	publishedFileName, publishError := publishHashedArtifactWithRef(hashedArtifactPublishOptions{
		outputDirectoryPath:   outputDirectoryPath,
		refFilePath:           refFilePath,
		desiredHashedFileName: desiredHashedFileName,
		content:               desiredContent,
		globPattern:           "styles-*.css",
	})
	if publishError != nil {
		t.Fatalf("publishHashedArtifactWithRef returned error: %v", publishError)
	}
	if publishedFileName != desiredHashedFileName {
		t.Fatalf("expected published file name %q, got %q", desiredHashedFileName, publishedFileName)
	}

	if _, statError := os.Stat(staleArtifactPath); !os.IsNotExist(statError) {
		t.Fatalf("expected stale artifact to be deleted, stat error: %v", statError)
	}

	desiredArtifactPath := filepath.Join(outputDirectoryPath, desiredHashedFileName)
	desiredArtifactBytes, readArtifactError := os.ReadFile(desiredArtifactPath)
	if readArtifactError != nil {
		t.Fatalf("failed reading desired artifact: %v", readArtifactError)
	}
	if string(desiredArtifactBytes) != string(desiredContent) {
		t.Fatalf("expected desired artifact content %q, got %q", string(desiredContent), string(desiredArtifactBytes))
	}

	refBytes, readRefError := os.ReadFile(refFilePath)
	if readRefError != nil {
		t.Fatalf("failed reading ref file: %v", readRefError)
	}
	if string(refBytes) != desiredHashedFileName {
		t.Fatalf("expected ref file to contain %q, got %q", desiredHashedFileName, string(refBytes))
	}
}

func TestPublishHashedArtifactWithRef_RefUpdateRemovesPreviousArtifact(t *testing.T) {
	root := t.TempDir()
	outputDirectoryPath := filepath.Join(root, "assets")
	refFilePath := filepath.Join(root, "assets.ref")

	if err := os.MkdirAll(outputDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating output directory: %v", err)
	}

	previousFileName := "styles-prev.css"
	previousArtifactPath := filepath.Join(outputDirectoryPath, previousFileName)
	if err := os.WriteFile(previousArtifactPath, []byte("prev"), 0o644); err != nil {
		t.Fatalf("failed writing previous artifact: %v", err)
	}
	if err := os.WriteFile(refFilePath, []byte(previousFileName), 0o644); err != nil {
		t.Fatalf("failed writing initial ref file: %v", err)
	}

	desiredHashedFileName := "styles-next.css"
	_, publishError := publishHashedArtifactWithRef(hashedArtifactPublishOptions{
		outputDirectoryPath:   outputDirectoryPath,
		refFilePath:           refFilePath,
		desiredHashedFileName: desiredHashedFileName,
		content:               []byte("next"),
		globPattern:           "styles-*.css",
	})
	if publishError != nil {
		t.Fatalf("publishHashedArtifactWithRef returned error: %v", publishError)
	}

	if _, statError := os.Stat(previousArtifactPath); !os.IsNotExist(statError) {
		t.Fatalf("expected previous artifact to be deleted, stat error: %v", statError)
	}

	nextArtifactPath := filepath.Join(outputDirectoryPath, desiredHashedFileName)
	nextArtifactBytes, readArtifactError := os.ReadFile(nextArtifactPath)
	if readArtifactError != nil {
		t.Fatalf("failed reading next artifact: %v", readArtifactError)
	}
	if string(nextArtifactBytes) != "next" {
		t.Fatalf("expected next artifact content %q, got %q", "next", string(nextArtifactBytes))
	}

	refBytes, readRefError := os.ReadFile(refFilePath)
	if readRefError != nil {
		t.Fatalf("failed reading ref file: %v", readRefError)
	}
	if string(refBytes) != desiredHashedFileName {
		t.Fatalf("expected ref file to contain %q, got %q", desiredHashedFileName, string(refBytes))
	}
}
