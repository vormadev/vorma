package vormabuild

import (
	"os"
	"strings"
	"testing"
)

func TestCleanStaticPublicOutDir_IgnoresMissingDirectory(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}

	if err := cleanStaticPublicOutDir(app); err != nil {
		t.Fatalf("cleanStaticPublicOutDir should ignore missing directory, got: %v", err)
	}
}

func TestCleanStaticPublicOutDir_ReturnsErrorWhenPathIsNotDirectory(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	mustWriteFile(t, fixture.publicDir, []byte("not a directory"))

	err := cleanStaticPublicOutDir(app)
	if err == nil {
		t.Fatal("expected cleanStaticPublicOutDir to return error")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("error = %q, expected not-a-directory context", err)
	}
}
