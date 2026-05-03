package cssbundle

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundleResolvesURLs(t *testing.T) {
	dir := t.TempDir()
	entry_path := filepath.Join(dir, "critical.css")
	err := os.WriteFile(
		entry_path,
		[]byte(`.logo{background:url("/logo.svg?v=1#mark")}`),
		0o644,
	)
	if err != nil {
		t.Fatalf("error writing CSS entry: %v", err)
	}

	result, err := Bundle(BundleArgs{
		EntryPath: entry_path,
		ResolveURL: func(
			_ string,
			parsed *url.URL,
		) (string, bool, error) {
			if parsed.Path != "/logo.svg" {
				return "", false, nil
			}
			return "/static/vorma_out_logo_HASH.svg?v=1#mark", true, nil
		},
	})
	if err != nil {
		t.Fatalf("error bundling CSS: %v", err)
	}

	expected := `url(/static/vorma_out_logo_HASH.svg?v=1#mark)`
	if !strings.Contains(result.CSS, expected) {
		t.Fatalf("expected CSS to contain %q, got %q", expected, result.CSS)
	}
}

func TestBundleReturnsResolverErrors(t *testing.T) {
	dir := t.TempDir()
	entry_path := filepath.Join(dir, "critical.css")
	err_resolver_failed := errors.New("resolver failed")
	err := os.WriteFile(entry_path, []byte(`.logo{background:url("./logo.svg")}`), 0o644)
	if err != nil {
		t.Fatalf("error writing CSS entry: %v", err)
	}

	_, err = Bundle(BundleArgs{
		EntryPath: entry_path,
		ResolveURL: func(
			_ string,
			_ *url.URL,
		) (string, bool, error) {
			return "", false, err_resolver_failed
		},
	})
	if err == nil {
		t.Fatalf("expected resolver error")
	}
	if !strings.Contains(err.Error(), err_resolver_failed.Error()) {
		t.Fatalf("expected resolver error, got %v", err)
	}
}
