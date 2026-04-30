package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseVersionNormalizesNpmAndGoVersions(t *testing.T) {
	version := release_version("v1.2.3-pre.4")

	if got := version.npm_version(); got != "1.2.3-pre.4" {
		t.Fatalf("npm_version() = %q, want %q", got, "1.2.3-pre.4")
	}
	if got := version.go_tag(); got != "v1.2.3-pre.4" {
		t.Fatalf("go_tag() = %q, want %q", got, "v1.2.3-pre.4")
	}
	if got := version.npm_tag(); got != "pre" {
		t.Fatalf("npm_tag() = %q, want %q", got, "pre")
	}
}

func TestReleaseVersionUsesLatestForStableVersions(t *testing.T) {
	version := release_version("1.2.3")

	if got := version.npm_version(); got != "1.2.3" {
		t.Fatalf("npm_version() = %q, want %q", got, "1.2.3")
	}
	if got := version.go_tag(); got != "v1.2.3" {
		t.Fatalf("go_tag() = %q, want %q", got, "v1.2.3")
	}
	if got := version.npm_tag(); got != "latest" {
		t.Fatalf("npm_tag() = %q, want %q", got, "latest")
	}
}

func TestReadNpmReleasePackagesRejectsMismatchedVersions(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})

	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(npm_package_json_path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(create_package_json_path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		npm_package_json_path,
		[]byte("{\n  \"version\": \"1.2.3\"\n}\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		create_package_json_path,
		[]byte("{\n  \"version\": \"1.2.4\"\n}\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	_, _, err = (release_app{}).read_npm_release_packages()
	if err == nil {
		t.Fatal("expected mismatched package versions to fail")
	}
	if !strings.Contains(err.Error(), "package versions differ") {
		t.Fatalf("error = %v", err)
	}
}
