package release_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/vormadev/vorma"
)

func TestReleaseDistributionConformance(t *testing.T) {
	root := repoRoot(t)
	rootPkg := mustReadJSONMap(t, filepath.Join(root, "package.json"))
	createPkg := mustReadJSONMap(t, filepath.Join(root, "internal", "framework", "_typescript", "create", "package.json"))
	goMod := mustReadFileAsString(t, filepath.Join(root, "go.mod"))
	makefile := mustReadFileAsString(t, filepath.Join(root, "Makefile"))
	buildTSScript := mustReadFileAsString(t, filepath.Join(root, "internal", "scripts", "buildts", "main.go"))
	npmBumperScript := mustReadFileAsString(t, filepath.Join(root, "internal", "scripts", "npm_bumper", "main.go"))
	goBumperScript := mustReadFileAsString(t, filepath.Join(root, "lab", "bumper", "bumper.go"))
	createMainTS := mustReadFileAsString(t, filepath.Join(root, "internal", "framework", "_typescript", "create", "main.ts"))
	vormaGo := mustReadFileAsString(t, filepath.Join(root, "vorma.go"))
	readme := mustReadFileAsString(t, filepath.Join(root, "README.md"))
	releaseSpec := mustReadFileAsString(t, filepath.Join(root, "specs", "VORMA_RELEASE_DISTRIBUTION_SPEC.md"))
	versionSpec := mustReadFileAsString(t, filepath.Join(root, "specs", "VORMA_VERSIONING_COMPATIBILITY_SPEC.md"))

	t.Run("RDC-VERS-001_REL-VERS-001_REL-VERS-004_REL-VERS-006_versions_semver_aligned_and_runtime_floors_hold", func(t *testing.T) {
		rootVersion := mustStringField(t, rootPkg, "version")
		createVersion := mustStringField(t, createPkg, "version")

		semverPattern := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
		if !semverPattern.MatchString(rootVersion) {
			t.Fatalf("expected root package version to be semver-like, got %q", rootVersion)
		}
		if !semverPattern.MatchString(createVersion) {
			t.Fatalf("expected create package version to be semver-like, got %q", createVersion)
		}
		if rootVersion != createVersion {
			t.Fatalf("expected root/create package versions to match, got root=%q create=%q", rootVersion, createVersion)
		}

		goVersionMatch := regexp.MustCompile(`(?m)^go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)$`).FindStringSubmatch(goMod)
		if len(goVersionMatch) != 2 {
			t.Fatalf("expected go.mod to declare go toolchain version, got %q", goMod)
		}
		if !versionAtLeast(t, goVersionMatch[1], "1.24.0") {
			t.Fatalf("expected go toolchain floor >= 1.24.0, got %q", goVersionMatch[1])
		}

		engines := mustMapField(t, createPkg, "engines")
		if got := mustStringField(t, engines, "node"); got != ">=22.11.0" {
			t.Fatalf("expected create package node engine floor %q, got %q", ">=22.11.0", got)
		}
	})

	t.Run("RDC-VERS-002_REL-VERS-002_REL-VERS-003_release_channel_publish_targets_are_split_pre_vs_latest", func(t *testing.T) {
		preExpected := []string{
			"tspublishpre: tsprepforpub",
			"@npm publish --access public --tag pre",
			"@cd internal/framework/_typescript/create && npm publish --access public --tag pre",
		}
		for _, fragment := range preExpected {
			if !strings.Contains(makefile, fragment) {
				t.Fatalf("expected pre-release publish flow to include %q", fragment)
			}
		}

		nonPreExpected := []string{
			"tspublishnonpre: tsprepforpub",
			"@npm publish --access public",
			"@cd internal/framework/_typescript/create && npm publish --access public",
		}
		for _, fragment := range nonPreExpected {
			if !strings.Contains(makefile, fragment) {
				t.Fatalf("expected final-release publish flow to include %q", fragment)
			}
		}
		if strings.Contains(makefile, "tspublishnonpre: tsprepforpub\n\t@npm publish --access public --tag pre") {
			t.Fatalf("expected final-release publish flow to not force npm pre tag")
		}
	})

	t.Run("RDC-VERS-003_REL-VERS-005_go_release_tag_path_uses_v_prefixed_semver", func(t *testing.T) {
		required := []string{
			`t.Plain("v")`,
			`bumpedVersion := "v" + trimmedVersion`,
			`t.Cmd("git", "tag", bumpedVersion)`,
		}
		for _, fragment := range required {
			if !strings.Contains(goBumperScript, fragment) {
				t.Fatalf("expected go release tag path to include %q", fragment)
			}
		}
	})

	t.Run("RDC-GO-001_REL-GO-001_module_path_stability", func(t *testing.T) {
		moduleMatch := regexp.MustCompile(`(?m)^module\s+(.+)$`).FindStringSubmatch(goMod)
		if len(moduleMatch) != 2 {
			t.Fatalf("expected go.mod module declaration")
		}
		if got := strings.TrimSpace(moduleMatch[1]); got != "github.com/vormadev/vorma" {
			t.Fatalf("expected module path %q, got %q", "github.com/vormadev/vorma", got)
		}
	})

	t.Run("RDC-GO-002_REL-GO-005_REL-ART-005_embedded_npm_version_matches_manifest_versions", func(t *testing.T) {
		rootVersion := mustStringField(t, rootPkg, "version")
		createVersion := mustStringField(t, createPkg, "version")
		embeddedVersion := vorma.Internal__GetCurrentNPMVersion()

		if embeddedVersion != rootVersion {
			t.Fatalf("expected embedded npm version to match root package version, embedded=%q root=%q", embeddedVersion, rootVersion)
		}
		if createVersion != rootVersion {
			t.Fatalf("expected create package version to match root package version, create=%q root=%q", createVersion, rootVersion)
		}
	})

	t.Run("RDC-GO-003_REL-GO-002_public_go_entry_surface_is_present", func(t *testing.T) {
		requiredConstructors := []string{"NewVormaApp", "NewLoader", "NewAction"}
		requiredVars := []string{
			"MustGetPort",
			"GetIsDev",
			"SetModeToDev",
			"IsJSONRequest",
			"VormaBuildIDHeaderKey",
			"EnableThirdPartyRouter",
		}
		requiredTypes := []string{
			"Vorma",
			"HeadEls",
			"AdHocType",
			"VormaAppConfig",
			"LoadersRouter",
			"LoaderReqData",
			"ActionsRouter",
			"ActionReqData",
			"None",
			"Action",
			"Loader",
			"LoaderFunc",
			"ActionFunc",
			"LoadersRouterOptions",
			"ActionsRouterOptions",
			"FormData",
			"LoaderError",
		}

		for _, symbol := range append(append(requiredConstructors, requiredVars...), requiredTypes...) {
			pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbol) + `\b`)
			if !pattern.MatchString(vormaGo) {
				t.Fatalf("expected top-level Go API surface to include symbol %q", symbol)
			}
		}
	})

	t.Run("RDC-GO-004_REL-GO-003_REL-GO-004_tag_publication_and_go_proxy_propagation_steps_are_present", func(t *testing.T) {
		required := []string{
			`t.Cmd("git", "tag", bumpedVersion)`,
			`t.Cmd("git", "push", "origin", "refs/tags/"+bumpedVersion)`,
			`t.Cmd("go", "list", "-m", "all")`,
			`"GOPROXY=proxy.golang.org"`,
		}
		for _, fragment := range required {
			if !strings.Contains(goBumperScript, fragment) {
				t.Fatalf("expected go release flow to include %q", fragment)
			}
		}
	})

	t.Run("RDC-NPM-001_REL-NPM-001_REL-COMPAT-004_package_identity_and_ui_variant_exports", func(t *testing.T) {
		if got := mustStringField(t, rootPkg, "name"); got != "vorma" {
			t.Fatalf("expected root package name %q, got %q", "vorma", got)
		}

		exports := mustMapField(t, rootPkg, "exports")
		for _, exportKey := range []string{"./react", "./preact", "./solid"} {
			if _, ok := exports[exportKey]; !ok {
				t.Fatalf("expected ui variant export %q to be present", exportKey)
			}
		}
	})

	t.Run("RDC-NPM-002_REL-NPM-002_REL-NPM-003_REL-ART-002_required_exports_have_import_and_types_entries", func(t *testing.T) {
		exports := mustMapField(t, rootPkg, "exports")
		requiredExports := []string{
			"./client",
			"./react",
			"./solid",
			"./preact",
			"./vite",
			"./kit/converters",
			"./kit/cookies",
			"./kit/csrf",
			"./kit/debounce",
			"./kit/fmt",
			"./kit/json",
			"./kit/listeners",
			"./kit/matcher/register",
			"./kit/matcher/find-best",
			"./kit/matcher/find-nested",
			"./kit/theme",
			"./kit/url",
		}

		for _, exportKey := range requiredExports {
			raw, ok := exports[exportKey]
			if !ok {
				t.Fatalf("expected required export key %q to be present", exportKey)
			}
			entry := mustAnyMap(t, raw, exportKey)
			importPath := mustStringField(t, entry, "import")
			typesPath := mustStringField(t, entry, "types")

			if !strings.HasSuffix(importPath, ".js") {
				t.Fatalf("expected export %q import path to end in .js, got %q", exportKey, importPath)
			}
			if !strings.HasSuffix(typesPath, ".d.ts") {
				t.Fatalf("expected export %q types path to end in .d.ts, got %q", exportKey, typesPath)
			}
		}
	})

	t.Run("RDC-NPM-003_REL-NPM-004_REL-NPM-005_REL-NPM-007_REL-ART-001_REL-ART-003_publish_files_and_export_targets_are_npm_dist_scoped", func(t *testing.T) {
		files := mustStringSliceField(t, rootPkg, "files")
		requireContains(t, files, "npm_dist/")
		requireContains(t, files, "LICENSE")
		requireContains(t, files, "README.md")
		requireContains(t, files, "!**/*.test.*")
		requireContains(t, files, "!**/*.bench.*")

		exports := mustMapField(t, rootPkg, "exports")
		for exportKey, raw := range exports {
			entry := mustAnyMap(t, raw, exportKey)
			importPath := mustStringField(t, entry, "import")
			typesPath := mustStringField(t, entry, "types")

			if !strings.HasPrefix(importPath, "./npm_dist/") {
				t.Fatalf("expected export %q import target to be under ./npm_dist/, got %q", exportKey, importPath)
			}
			if !strings.HasPrefix(typesPath, "./npm_dist/") {
				t.Fatalf("expected export %q types target to be under ./npm_dist/, got %q", exportKey, typesPath)
			}
		}
	})

	t.Run("RDC-NPM-004_REL-NPM-006_side_effects_metadata_stability", func(t *testing.T) {
		if got := mustBoolField(t, rootPkg, "sideEffects"); got {
			t.Fatalf("expected sideEffects=false, got true")
		}
	})

	t.Run("RDC-CREATE-001_REL-CREATE-001_REL-CREATE-002_REL-CREATE-003_REL-CREATE-004_REL-ART-004_create_package_distribution_contract", func(t *testing.T) {
		if got := mustStringField(t, createPkg, "name"); got != "create-vorma" {
			t.Fatalf("expected create package name %q, got %q", "create-vorma", got)
		}
		if got := mustStringField(t, createPkg, "version"); got != mustStringField(t, rootPkg, "version") {
			t.Fatalf("expected create/root package versions to match, got create=%q root=%q", got, mustStringField(t, rootPkg, "version"))
		}

		bin := mustMapField(t, createPkg, "bin")
		if got := mustStringField(t, bin, "create-vorma"); got != "./dist/main.js" {
			t.Fatalf("expected create-vorma bin to target %q, got %q", "./dist/main.js", got)
		}

		files := mustStringSliceField(t, createPkg, "files")
		if len(files) != 1 || files[0] != "dist" {
			t.Fatalf("expected create package files allowlist to be exactly [dist], got %#v", files)
		}

		engines := mustMapField(t, createPkg, "engines")
		if got := mustStringField(t, engines, "node"); got != ">=22.11.0" {
			t.Fatalf("expected create package node engine floor %q, got %q", ">=22.11.0", got)
		}
	})

	t.Run("RDC-CREATE-002_REL-CREATE-005_scaffold_runtime_checks_local_go_and_node_prerequisites", func(t *testing.T) {
		required := []string{
			`execSync("go version", { encoding: "utf8" }).trim()`,
			`if (major < 1 || (major === 1 && minor < 24))`,
			"Go version 1.24 or higher is required.",
			"const nodeVersion = process.version",
			"if (nodeMajor < 22)",
			"Node.js version 22.11 or higher is required",
		}
		for _, fragment := range required {
			if !strings.Contains(createMainTS, fragment) {
				t.Fatalf("expected create runtime prerequisite check to include %q", fragment)
			}
		}
	})

	t.Run("RDC-PIPE-001_REL-PIPE-001_REL-PIPE-002_release_prep_and_dist_build_targets_are_present", func(t *testing.T) {
		requiredFragments := []string{
			"tsprepforpub: tsreset tstest tslint tscheck",
			"npmbuild:",
			"@go run ./internal/scripts/buildts",
		}
		for _, fragment := range requiredFragments {
			if !strings.Contains(makefile, fragment) {
				t.Fatalf("expected Makefile to include %q", fragment)
			}
		}
	})

	t.Run("RDC-PIPE-002_REL-PIPE-003_REL-PIPE-004_dist_rebuild_cleans_previous_output_and_fails_on_warnings", func(t *testing.T) {
		requiredFragments := []string{
			`var targetDir = "./npm_dist"`,
			"os.RemoveAll(targetDir)",
			"os.MkdirAll(targetDir, 0755)",
			"if len(result.Warnings) > 0",
			`log.Fatalf("%s: esbuild had warnings", label)`,
		}
		for _, fragment := range requiredFragments {
			if !strings.Contains(buildTSScript, fragment) {
				t.Fatalf("expected buildts implementation to include %q", fragment)
			}
		}
	})

	t.Run("RDC-PIPE-003_REL-PIPE-005_REL-PIPE-006_release_sequence_building_blocks_and_instruction_alignment_are_present", func(t *testing.T) {
		requireOrderedSubstrings(
			t,
			npmBumperScript,
			[]string{
				`t.Cmd("make", "tsprepforpub")`,
				`t.Cmd("make", "npmbuild")`,
			},
		)

		requiredMakeTargets := []string{
			"tsprepforpub:",
			"npmbuild:",
			"tspublishpre:",
			"tspublishnonpre:",
		}
		for _, fragment := range requiredMakeTargets {
			if !strings.Contains(makefile, fragment) {
				t.Fatalf("expected release tooling alignment to include %q", fragment)
			}
		}

		requiredGoReleaseSteps := []string{
			`have you pushed your code?`,
			`t.Cmd("git", "tag", bumpedVersion)`,
			`t.Cmd("git", "push", "origin", "refs/tags/"+bumpedVersion)`,
		}
		for _, fragment := range requiredGoReleaseSteps {
			if !strings.Contains(goBumperScript, fragment) {
				t.Fatalf("expected go release phase coverage to include %q", fragment)
			}
		}
	})

	t.Run("RDC-COMPAT-001_REL-COMPAT-001_REL-COMPAT-002_REL-COMPAT-003_compatibility_and_deprecation_policy_artifacts_are_captured", func(t *testing.T) {
		releasePolicyRequired := []string{
			"### REL-COMPAT-001",
			"### REL-COMPAT-002",
			"### REL-COMPAT-003",
		}
		for _, fragment := range releasePolicyRequired {
			if !strings.Contains(releaseSpec, fragment) {
				t.Fatalf("expected release compatibility policy section to include %q", fragment)
			}
		}

		versionPolicyRequired := []string{
			"### VER-PRE1-002",
			"release notes/changelog",
			"### VER-DEPR-002",
			"### VER-DEPR-003",
		}
		for _, fragment := range versionPolicyRequired {
			if !strings.Contains(versionSpec, fragment) {
				t.Fatalf("expected compatibility/deprecation policy artifacts to include %q", fragment)
			}
		}

		if !strings.Contains(readme, "Sub-1.0 releases may contain breaking changes.") {
			t.Fatalf("expected README to disclose pre-1.0 breaking-change possibility")
		}
	})
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root detection failed from %s: %v", wd, err)
	}
	return root
}

func mustReadJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json file %s: %v", path, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("unmarshal json file %s: %v body=%q", path, err, string(b))
	}
	return payload
}

func mustReadFileAsString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(b)
}

func mustMapField(t *testing.T, m map[string]any, field string) map[string]any {
	t.Helper()
	raw, ok := m[field]
	if !ok {
		t.Fatalf("expected field %q to be present", field)
	}
	return mustAnyMap(t, raw, field)
}

func mustAnyMap(t *testing.T, raw any, field string) map[string]any {
	t.Helper()
	typed, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be map[string]any, got %#v", field, raw)
	}
	return typed
}

func mustStringField(t *testing.T, m map[string]any, field string) string {
	t.Helper()
	raw, ok := m[field]
	if !ok {
		t.Fatalf("expected field %q to be present", field)
	}
	typed, ok := raw.(string)
	if !ok {
		t.Fatalf("expected field %q to be string, got %#v", field, raw)
	}
	return typed
}

func mustBoolField(t *testing.T, m map[string]any, field string) bool {
	t.Helper()
	raw, ok := m[field]
	if !ok {
		t.Fatalf("expected field %q to be present", field)
	}
	typed, ok := raw.(bool)
	if !ok {
		t.Fatalf("expected field %q to be bool, got %#v", field, raw)
	}
	return typed
}

func mustStringSliceField(t *testing.T, m map[string]any, field string) []string {
	t.Helper()
	raw, ok := m[field]
	if !ok {
		t.Fatalf("expected field %q to be present", field)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected field %q to be []any, got %#v", field, raw)
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("expected all %q entries to be strings, got %#v", field, item)
		}
		out = append(out, s)
	}
	return out
}

func requireContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected list to contain %q, got %#v", want, values)
}

func requireOrderedSubstrings(t *testing.T, haystack string, needles []string) {
	t.Helper()
	searchFrom := 0
	for _, needle := range needles {
		relative := strings.Index(haystack[searchFrom:], needle)
		if relative < 0 {
			t.Fatalf("expected ordered content to include %q after index %d", needle, searchFrom)
		}
		searchFrom += relative + len(needle)
	}
}

func versionAtLeast(t *testing.T, got string, floor string) bool {
	t.Helper()
	gotParts := parseVersionTuple(t, got)
	floorParts := parseVersionTuple(t, floor)
	for i := 0; i < len(gotParts) && i < len(floorParts); i++ {
		if gotParts[i] > floorParts[i] {
			return true
		}
		if gotParts[i] < floorParts[i] {
			return false
		}
	}
	return true
}

func parseVersionTuple(t *testing.T, v string) []int {
	t.Helper()
	segments := strings.Split(v, ".")
	if len(segments) < 2 {
		t.Fatalf("expected version %q to have at least major.minor", v)
	}
	out := make([]int, len(segments))
	for i, segment := range segments {
		n, err := strconv.Atoi(segment)
		if err != nil {
			t.Fatalf("expected numeric version segment in %q, got %q", v, segment)
		}
		out[i] = n
	}
	return out
}
