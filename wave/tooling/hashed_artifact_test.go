package tooling

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/wave/tooling/builder/static"
)

func TestPublishHashedArtifactWithRef_RefMissingCleansStaleArtifacts(
	t *testing.T,
) {
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
	publishedFileName, publishError := static.PublishHashedArtifactWithRef(
		static.HashedArtifactPublishOptions{
			OutputDirectoryPath:   outputDirectoryPath,
			RefFilePath:           refFilePath,
			DesiredHashedFileName: desiredHashedFileName,
			Content:               desiredContent,
			GlobPattern:           "styles-*.css",
		},
	)
	if publishError != nil {
		t.Fatalf(
			"static.PublishHashedArtifactWithRef returned error: %v",
			publishError,
		)
	}
	if publishedFileName != desiredHashedFileName {
		t.Fatalf(
			"expected published file name %q, got %q",
			desiredHashedFileName,
			publishedFileName,
		)
	}

	if _, statError := os.Stat(staleArtifactPath); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected stale artifact to be deleted, stat error: %v",
			statError,
		)
	}

	desiredArtifactPath := filepath.Join(
		outputDirectoryPath,
		desiredHashedFileName,
	)
	desiredArtifactBytes, readArtifactError := os.ReadFile(desiredArtifactPath)
	if readArtifactError != nil {
		t.Fatalf("failed reading desired artifact: %v", readArtifactError)
	}
	if string(desiredArtifactBytes) != string(desiredContent) {
		t.Fatalf(
			"expected desired artifact Content %q, got %q",
			string(desiredContent),
			string(desiredArtifactBytes),
		)
	}

	refBytes, readRefError := os.ReadFile(refFilePath)
	if readRefError != nil {
		t.Fatalf("failed reading ref file: %v", readRefError)
	}
	if string(refBytes) != desiredHashedFileName {
		t.Fatalf(
			"expected ref file to contain %q, got %q",
			desiredHashedFileName,
			string(refBytes),
		)
	}
}

func TestPublishHashedArtifactWithRef_RefUpdateRemovesPreviousArtifact(
	t *testing.T,
) {
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
	_, publishError := static.PublishHashedArtifactWithRef(
		static.HashedArtifactPublishOptions{
			OutputDirectoryPath:   outputDirectoryPath,
			RefFilePath:           refFilePath,
			DesiredHashedFileName: desiredHashedFileName,
			Content:               []byte("next"),
			GlobPattern:           "styles-*.css",
		},
	)
	if publishError != nil {
		t.Fatalf(
			"static.PublishHashedArtifactWithRef returned error: %v",
			publishError,
		)
	}

	if _, statError := os.Stat(previousArtifactPath); !os.IsNotExist(
		statError,
	) {
		t.Fatalf(
			"expected previous artifact to be deleted, stat error: %v",
			statError,
		)
	}

	nextArtifactPath := filepath.Join(
		outputDirectoryPath,
		desiredHashedFileName,
	)
	nextArtifactBytes, readArtifactError := os.ReadFile(nextArtifactPath)
	if readArtifactError != nil {
		t.Fatalf("failed reading next artifact: %v", readArtifactError)
	}
	if string(nextArtifactBytes) != "next" {
		t.Fatalf(
			"expected next artifact Content %q, got %q",
			"next",
			string(nextArtifactBytes),
		)
	}

	refBytes, readRefError := os.ReadFile(refFilePath)
	if readRefError != nil {
		t.Fatalf("failed reading ref file: %v", readRefError)
	}
	if string(refBytes) != desiredHashedFileName {
		t.Fatalf(
			"expected ref file to contain %q, got %q",
			desiredHashedFileName,
			string(refBytes),
		)
	}
}

func TestPublishHashedArtifactWithRef_EmptyRefFileCleansStaleArtifacts(
	t *testing.T,
) {
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
	if err := os.WriteFile(refFilePath, []byte(" \n "), 0o644); err != nil {
		t.Fatalf("failed writing empty ref file: %v", err)
	}

	desiredHashedFileName := "styles-new.css"
	_, publishError := static.PublishHashedArtifactWithRef(
		static.HashedArtifactPublishOptions{
			OutputDirectoryPath:   outputDirectoryPath,
			RefFilePath:           refFilePath,
			DesiredHashedFileName: desiredHashedFileName,
			Content:               []byte("new"),
			GlobPattern:           "styles-*.css",
		},
	)
	if publishError != nil {
		t.Fatalf(
			"static.PublishHashedArtifactWithRef returned error: %v",
			publishError,
		)
	}

	if _, statError := os.Stat(staleArtifactPath); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected stale artifact to be deleted when ref is empty, stat error: %v",
			statError,
		)
	}
}
