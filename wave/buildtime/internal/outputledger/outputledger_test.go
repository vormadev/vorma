package outputledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/wave/waveartifacts"
)

func TestLedger_RecordAndLoadRecordedRelativePaths(t *testing.T) {
	staticRootPath := filepath.Join(t.TempDir(), "dist", "static")
	ledgerFilePath := filepath.Join(
		staticRootPath,
		waveartifacts.InternalDirname,
		"ledger.txt",
	)
	if err := os.MkdirAll(filepath.Dir(ledgerFilePath), 0o755); err != nil {
		t.Fatalf("mkdir ledger directory: %v", err)
	}

	ledger := New(staticRootPath, ledgerFilePath)
	if err := ledger.Reset(); err != nil {
		t.Fatalf("reset ledger: %v", err)
	}

	keepAssetPath := filepath.Join(
		staticRootPath,
		waveartifacts.AssetsDirname,
		waveartifacts.PublicDirname,
		"a.js",
	)
	keepInternalPath := filepath.Join(
		staticRootPath,
		waveartifacts.InternalDirname,
		"schema.json",
	)
	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	if err := ledger.RecordPaths([]string{
		keepAssetPath,
		keepInternalPath,
		keepAssetPath,
		outsidePath,
	}); err != nil {
		t.Fatalf("record paths: %v", err)
	}

	recorded, err := ledger.LoadRecordedRelativePaths()
	if err != nil {
		t.Fatalf("load recorded paths: %v", err)
	}
	if _, ok := recorded[filepath.ToSlash(filepath.Join(
		waveartifacts.AssetsDirname,
		waveartifacts.PublicDirname,
		"a.js",
	))]; !ok {
		t.Fatalf("expected recorded assets/public/a.js, got %#v", recorded)
	}
	if _, ok := recorded[filepath.ToSlash(filepath.Join(
		waveartifacts.InternalDirname,
		"schema.json",
	))]; !ok {
		t.Fatalf("expected recorded internal/schema.json, got %#v", recorded)
	}
	if _, ok := recorded["outside.txt"]; ok {
		t.Fatalf("did not expect outside path in recorded set: %#v", recorded)
	}
}

func TestLedger_SweepUnrecordedFiles(t *testing.T) {
	staticRootPath := filepath.Join(t.TempDir(), "dist", "static")
	publicRootPath := filepath.Join(
		staticRootPath,
		waveartifacts.AssetsDirname,
		waveartifacts.PublicDirname,
	)
	internalRootPath := filepath.Join(
		staticRootPath,
		waveartifacts.InternalDirname,
	)
	if err := os.MkdirAll(publicRootPath, 0o755); err != nil {
		t.Fatalf("mkdir public root: %v", err)
	}
	if err := os.MkdirAll(internalRootPath, 0o755); err != nil {
		t.Fatalf("mkdir internal root: %v", err)
	}

	keepPublicPath := filepath.Join(publicRootPath, "keep.js")
	stalePublicPath := filepath.Join(publicRootPath, "stale.js")
	keepInternalPath := filepath.Join(internalRootPath, "keep.txt")
	staleInternalPath := filepath.Join(internalRootPath, "stale.txt")
	for _, filePath := range []string{
		keepPublicPath,
		stalePublicPath,
		keepInternalPath,
		staleInternalPath,
	} {
		if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
			t.Fatalf("write fixture file %q: %v", filePath, err)
		}
	}

	ledgerFilePath := filepath.Join(internalRootPath, "ledger.txt")
	ledger := New(staticRootPath, ledgerFilePath)
	if err := ledger.Reset(); err != nil {
		t.Fatalf("reset ledger: %v", err)
	}
	if err := ledger.RecordPaths([]string{keepPublicPath, keepInternalPath}); err != nil {
		t.Fatalf("record paths: %v", err)
	}

	if err := ledger.SweepUnrecordedFiles([]string{
		filepath.Join(staticRootPath, waveartifacts.AssetsDirname),
		internalRootPath,
	}); err != nil {
		t.Fatalf("sweep unrecorded files: %v", err)
	}

	assertPathExists(t, keepPublicPath)
	assertPathExists(t, keepInternalPath)
	assertPathExists(t, ledgerFilePath)
	assertPathMissing(t, stalePublicPath)
	assertPathMissing(t, staleInternalPath)
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path to exist %q: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected path to be removed %q, stat err=%v", path, err)
	}
}
