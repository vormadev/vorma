package wave

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

type waveTestFixture struct {
	root string
	cfg  *ParsedConfig
}

func newWaveTestFixture(t *testing.T) *waveTestFixture {
	t.Helper()

	root := t.TempDir()
	cfg := &ParsedConfig{
		Core: &CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: StaticAssetDirs{
				Public:  filepath.Join(root, "public-src"),
				Private: filepath.Join(root, "private-src"),
			},
			CSSEntryFiles: CSSEntryFiles{
				Critical:    "./src/critical.css",
				NonCritical: "./src/non_critical.css",
			},
			PublicPathPrefix: "/assets/",
		},
		Vite: &ViteConfig{DefaultPort: 5173},
		Watch: &WatchConfig{
			WatchRoot:           "./watch/root",
			HealthcheckEndpoint: "/healthz",
		},
	}
	cfg.Dist = DistLayout{Root: cfg.Core.DistDir}

	mustEnsureDir(t, cfg.Dist.StaticPublic())
	mustEnsureDir(t, cfg.Dist.StaticPrivate())
	mustEnsureDir(t, cfg.Dist.Internal())

	mustWriteFile(t, filepath.Join(cfg.Dist.StaticPublic(), "logo.txt"), "logo")
	mustWriteFile(t, filepath.Join(cfg.Dist.StaticPublic(), "favicon.ico"), "ico")
	mustWriteFile(t, filepath.Join(cfg.Dist.StaticPublic(), "vorma_out", "logo.hash.txt"), "hashed-logo")
	mustWriteFile(t, filepath.Join(cfg.Dist.StaticPublic(), "vorma_out", "favicon.hash.ico"), "hashed-ico")
	mustWriteFile(t, filepath.Join(cfg.Dist.StaticPublic(), "vorma_out", "vorma_internal_normal_hash.css"), "body{color:blue;}")
	mustWriteFile(t, filepath.Join(cfg.Dist.StaticPublic(), "vorma_out", "vorma_internal_public_filemap_hash.js"), "export const wavePublicFileMap = {}")
	mustWriteFile(t, filepath.Join(cfg.Dist.StaticPrivate(), "template.html"), "private")

	mustWriteFile(t, cfg.Dist.CriticalCSS(), "body{color:red;}")
	mustWriteFile(t, cfg.Dist.NormalCSSRef(), "vorma_out/vorma_internal_normal_hash.css")
	mustWriteFile(t, cfg.Dist.PublicFileMapRef(), "vorma_out/vorma_internal_public_filemap_hash.js")

	mustWriteGob(t, cfg.Dist.PublicFileMapGob(), FileMap{
		"logo.txt": {
			DistName:    "vorma_out/logo.hash.txt",
			ContentHash: "logo-hash",
		},
		"favicon.ico": {
			DistName:    "vorma_out/favicon.hash.ico",
			ContentHash: "favicon-hash",
		},
	})

	return &waveTestFixture{root: root, cfg: cfg}
}

func (f *waveTestFixture) configJSON(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(f.cfg)
	if err != nil {
		t.Fatalf("failed to marshal config JSON: %v", err)
	}
	return data
}

func mustEnsureDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}
}

func mustWriteFile(t *testing.T, filePath string, content string) {
	t.Helper()
	mustEnsureDir(t, filepath.Dir(filePath))
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", filePath, err)
	}
}

func mustWriteGob(t *testing.T, filePath string, data any) {
	t.Helper()
	mustEnsureDir(t, filepath.Dir(filePath))

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		t.Fatalf("failed to encode gob for %s: %v", filePath, err)
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("failed to write gob file %s: %v", filePath, err)
	}
}

func mustEncodeGobBytes(t *testing.T, data any) []byte {
	t.Helper()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		t.Fatalf("failed to encode gob bytes: %v", err)
	}

	return buf.Bytes()
}

func setWaveDevModeForTest(t *testing.T, isDev bool) {
	t.Helper()
	if isDev {
		t.Setenv(envMode, envModeDev)
		return
	}
	t.Setenv(envMode, "production")
}

func resetPortCacheForTest() {
	defaultPortResolver = NewPortResolver()
}

func mustReadFileFromFS(t *testing.T, filesystem fs.FS, filePath string) string {
	t.Helper()
	data, err := fs.ReadFile(filesystem, filePath)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", filePath, err)
	}
	return string(data)
}
