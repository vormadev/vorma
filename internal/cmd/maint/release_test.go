package main

import "testing"

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
