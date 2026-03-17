package docsync

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vormadev/vorma/wave"
)

const generatedMarker = "<!-- GENERATED_BY: internal/site/backend/cmd/sync_docs -->"
const mirrorURLPrefix = "/reference/readmes/"
const mirrorRootRel = "internal/site/frontend/assets/reference/readmes"
const publicAssetsRootRel = "internal/site/frontend/assets"
const legacyMirrorRootRel = "internal/site/frontend/assets/nohash/reference"
const docsReferenceURLRoot = "/docs/~"

var htmlSrcURLRe = regexp.MustCompile(`(?i)\bsrc\s*=\s*"([^"]+)"`)
var markdownLinkURLRe = regexp.MustCompile(`(!?\[[^\]]*\]\()([^)]+)(\))`)
var htmlAttrDoubleURLRe = regexp.MustCompile(`(?i)\b(src|href)\s*=\s*"([^"]+)"`)
var htmlAttrSingleURLRe = regexp.MustCompile(`(?i)\b(src|href)\s*=\s*'([^']+)'`)

// Result describes sync outcomes.
type Result struct {
	Sources       int
	Written       int
	Deleted       int
	AssetsWritten int
	AssetsDeleted int
	URLsRewritten int
}

type readmeDocMeta struct {
	Source string
}

type readmeDocSpec struct {
	SourcePath string
	ReadmeRel  string
	Dir        string
	TargetPath string
	ParentDir  string
	ChildName  string
}

type assetPlan struct {
	repoRoot               string
	mirrorRoot             string
	expected               map[string]string // dest -> source
	readmeDirs             map[string]struct{}
	dirsWithReadmeChildren map[string]struct{}
}

func newAssetPlan(
	repoRoot string,
	readmeDirs, dirsWithReadmeChildren map[string]struct{},
) *assetPlan {
	return &assetPlan{
		repoRoot:               repoRoot,
		mirrorRoot:             filepath.Join(repoRoot, mirrorRootRel),
		expected:               make(map[string]string),
		readmeDirs:             readmeDirs,
		dirsWithReadmeChildren: dirsWithReadmeChildren,
	}
}

func (p *assetPlan) registerAsset(
	resolvedRepoPath, sourceAbsPath string,
) (string, error) {
	resolvedRepoPath = strings.TrimPrefix(
		filepath.ToSlash(resolvedRepoPath),
		"./",
	)
	resolvedRepoPath = strings.TrimPrefix(resolvedRepoPath, "/")
	if resolvedRepoPath == "" || resolvedRepoPath == "." {
		return "", fmt.Errorf(
			"invalid mirror path for asset: %q",
			sourceAbsPath,
		)
	}

	destPath := filepath.Join(
		p.mirrorRoot,
		filepath.FromSlash(resolvedRepoPath),
	)
	p.expected[destPath] = sourceAbsPath
	return mirrorURLPrefix + resolvedRepoPath, nil
}

// SyncAllReadmes syncs every README.md in the repository into docs/~.
func SyncAllReadmes() (Result, error) {
	wd, err := os.Getwd()
	if err != nil {
		return Result{}, fmt.Errorf("get cwd: %w", err)
	}

	repoRoot, err := findRepoRoot(wd)
	if err != nil {
		return Result{}, err
	}
	modulePath, err := readModulePath(repoRoot)
	if err != nil {
		return Result{}, err
	}

	sources, err := collectReadmes(repoRoot)
	if err != nil {
		return Result{}, err
	}

	readmeDirs, err := collectReadmeDirs(sources, repoRoot)
	if err != nil {
		return Result{}, err
	}
	dirsWithReadmeChildren := collectDirsWithReadmeChildren(readmeDirs)
	sectionDirs := collectSectionDirs(readmeDirs)

	docsRoot := filepath.Join(
		repoRoot,
		"internal",
		"site",
		"backend",
		"assets",
		"markdown",
		"docs",
	)
	docsOutRoot := filepath.Join(docsRoot, "~")
	plan := newAssetPlan(repoRoot, readmeDirs, dirsWithReadmeChildren)
	readmeSpecs, err := buildReadmeSpecs(
		sources,
		repoRoot,
		docsOutRoot,
		dirsWithReadmeChildren,
	)
	if err != nil {
		return Result{}, err
	}
	siblingOrders := buildSiblingOrderMap(readmeSpecs, sectionDirs)

	expectedDocs := make(map[string]struct{}, len(sources))
	readmeMeta := make(map[string]readmeDocMeta, len(sources))
	res := Result{Sources: len(sources)}

	for _, spec := range readmeSpecs {
		expectedDocs[spec.TargetPath] = struct{}{}
		order, ok := lookupSiblingOrder(
			siblingOrders,
			spec.ParentDir,
			spec.ChildName,
		)
		if !ok {
			return res, fmt.Errorf(
				"missing sibling order for %s/%s",
				spec.ParentDir,
				spec.ChildName,
			)
		}

		content, meta, err := renderReadmeOverviewFile(
			spec.SourcePath,
			repoRoot,
			modulePath,
			plan,
			order,
		)
		if err != nil {
			return res, err
		}
		readmeMeta[spec.Dir] = meta

		changed, err := writeIfChanged(spec.TargetPath, content)
		if err != nil {
			return res, err
		}
		if changed {
			res.Written++
		}
	}

	indexesWritten, err := writeSectionIndexes(
		docsOutRoot,
		sectionDirs,
		readmeMeta,
		expectedDocs,
		siblingOrders,
	)
	if err != nil {
		return res, err
	}
	res.Written += indexesWritten

	deleted, err := deleteStaleGeneratedDocs(docsRoot, expectedDocs)
	if err != nil {
		return res, err
	}
	res.Deleted = deleted

	if err := pruneEmptyDirs(docsRoot); err != nil {
		return res, err
	}

	assetsWritten, assetsDeleted, err := syncMirrorAssets(plan)
	if err != nil {
		return res, err
	}
	res.AssetsWritten = assetsWritten
	res.AssetsDeleted = assetsDeleted

	if err := removeLegacyMirrorRoot(repoRoot); err != nil {
		return res, err
	}

	return res, nil
}

// SyncAndResolvePublicURLs runs README sync and then rewrites generated doc URLs
// using the current Wave public file map.
func SyncAndResolvePublicURLs() (Result, error) {
	res, err := SyncAllReadmes()
	if err != nil {
		return res, err
	}

	rewritten, err := ResolveGeneratedDocsPublicURLs()
	if err != nil {
		return res, err
	}
	res.URLsRewritten = rewritten
	return res, nil
}

// ResolveGeneratedDocsPublicURLs rewrites generated docs URLs using the current
// public file map.
func ResolveGeneratedDocsPublicURLs() (int, error) {
	assetMap, publicPathPrefix, err := loadPublicAssetMap()
	if err != nil {
		return 0, err
	}
	return rewriteGeneratedDocsWithAssetMap(assetMap, publicPathPrefix)
}

func loadPublicAssetMap() (map[string]string, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("get cwd: %w", err)
	}

	repoRoot, err := findRepoRoot(wd)
	if err != nil {
		return nil, "", err
	}

	staticRootRel, err := filepath.Rel(
		wd,
		repoRoot,
		"internal/site/backend/.waveout/static",
	)
	if err != nil {
		return nil, "", fmt.Errorf("resolve Wave static root: %w", err)
	}

	if wave.IsDev() && os.Getenv("WAVE_DEV_STATIC_DIR") == "" {
		// sync_docs also runs from Wave lifecycle hook processes. Those
		// processes already carry WAVE_IS_DEV=true, but they do not get the
		// runtime static-dir env var because they are not the supervised app.
		if err := os.Setenv("WAVE_DEV_STATIC_DIR", staticRootRel); err != nil {
			return nil, "", fmt.Errorf("set WAVE_DEV_STATIC_DIR: %w", err)
		}
	}

	waveRuntime := wave.New(wave.Options{
		DistStaticFS: os.DirFS(staticRootRel),
	})
	assetMap, err := waveRuntime.PublicStaticFilemap()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("load Wave public static filemap: %w", err)
	}

	publicPathPrefix, err := waveRuntime.PublicPathPrefix()
	if err != nil {
		return nil, "", fmt.Errorf("load Wave public path prefix: %w", err)
	}

	return assetMap, publicPathPrefix, nil
}

func rewriteGeneratedDocsWithAssetMap(
	assetMap map[string]string,
	publicPathPrefix string,
) (int, error) {
	if len(assetMap) == 0 {
		return 0, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("get cwd: %w", err)
	}

	repoRoot, err := findRepoRoot(wd)
	if err != nil {
		return 0, err
	}

	docsOutRoot := filepath.Join(
		repoRoot,
		"internal",
		"site",
		"backend",
		"assets",
		"markdown",
		"docs",
	)
	info, err := os.Stat(docsOutRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat docs output root %s: %w", docsOutRoot, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf(
			"docs output root is not a directory: %s",
			docsOutRoot,
		)
	}

	rewrittenFiles := 0
	err = filepath.WalkDir(
		docsOutRoot,
		func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || filepath.Ext(p) != ".md" {
				return nil
			}

			raw, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("read generated doc %s: %w", p, err)
			}
			if !bytes.Contains(raw, []byte(generatedMarker)) {
				return nil
			}

			updated := rewriteContentWithPublicAssetMap(
				string(raw),
				assetMap,
				publicPathPrefix,
			)

			changed, err := writeIfChanged(p, []byte(updated))
			if err != nil {
				return err
			}
			if changed {
				rewrittenFiles++
			}
			return nil
		},
	)
	if err != nil {
		return rewrittenFiles, fmt.Errorf(
			"walk generated docs for URL rewrite: %w",
			err,
		)
	}

	return rewrittenFiles, nil
}

func findRepoRoot(start string) (string, error) {
	cur, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("abs start path: %w", err)
	}

	for {
		if isRepoRoot(cur) {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}

	return "", fmt.Errorf("failed to locate repo root from %s", start)
}

func isRepoRoot(dir string) bool {
	if !isDir(filepath.Join(dir, "kit")) {
		return false
	}
	if !isDir(filepath.Join(dir, "internal", "site", "backend")) {
		return false
	}
	return true
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func collectReadmes(repoRoot string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(
		repoRoot,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() && shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}

			if d.IsDir() {
				return nil
			}

			if d.Name() == "README.md" {
				paths = append(paths, path)
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("walk repo for README files: %w", err)
	}

	sort.Strings(paths)
	return paths, nil
}

func collectReadmeDirs(
	sources []string,
	repoRoot string,
) (map[string]struct{}, error) {
	readmeDirs := make(map[string]struct{}, len(sources))
	for _, src := range sources {
		rel, err := filepath.Rel(repoRoot, src)
		if err != nil {
			return nil, fmt.Errorf("rel path for %s: %w", src, err)
		}
		relDir := normalizeDocDir(filepath.ToSlash(filepath.Dir(rel)))
		readmeDirs[relDir] = struct{}{}
	}
	return readmeDirs, nil
}

func collectDirsWithReadmeChildren(
	readmeDirs map[string]struct{},
) map[string]struct{} {
	withChildren := make(map[string]struct{}, len(readmeDirs))
	for dir := range readmeDirs {
		d := normalizeDocDir(dir)
		for {
			parent := normalizeDocDir(filepath.ToSlash(filepath.Dir(d)))
			if parent == d {
				break
			}
			withChildren[parent] = struct{}{}
			if parent == "." {
				break
			}
			d = parent
		}
	}
	return withChildren
}

func collectSectionDirs(readmeDirs map[string]struct{}) map[string]struct{} {
	sectionDirs := map[string]struct{}{".": {}}
	for dir := range readmeDirs {
		d := normalizeDocDir(dir)
		for {
			parent := normalizeDocDir(filepath.ToSlash(filepath.Dir(d)))
			sectionDirs[parent] = struct{}{}
			if parent == "." {
				break
			}
			d = parent
		}
	}
	return sectionDirs
}

func buildReadmeSpecs(
	sources []string,
	repoRoot, outRoot string,
	dirsWithReadmeChildren map[string]struct{},
) ([]readmeDocSpec, error) {
	specs := make([]readmeDocSpec, 0, len(sources))
	for _, src := range sources {
		rel, err := filepath.Rel(repoRoot, src)
		if err != nil {
			return nil, fmt.Errorf("rel path for %s: %w", src, err)
		}
		rel = filepath.ToSlash(rel)
		dir := normalizeDocDir(filepath.ToSlash(filepath.Dir(rel)))
		hasReadmeChildren := dirHasReadmeChildren(dir, dirsWithReadmeChildren)

		parentDir, childName := readmeSitemapPlacement(dir, hasReadmeChildren)
		specs = append(specs, readmeDocSpec{
			SourcePath: src,
			ReadmeRel:  rel,
			Dir:        dir,
			TargetPath: overviewTargetPath(rel, outRoot, hasReadmeChildren),
			ParentDir:  parentDir,
			ChildName:  childName,
		})
	}
	return specs, nil
}

func buildSiblingOrderMap(
	readmeSpecs []readmeDocSpec,
	sectionDirs map[string]struct{},
) map[string]int {
	childrenByParent := make(map[string]map[string]struct{})
	addChild := func(parent, child string) {
		parent = normalizeDocDir(parent)
		child = strings.TrimSpace(child)
		if child == "" {
			return
		}
		if _, ok := childrenByParent[parent]; !ok {
			childrenByParent[parent] = make(map[string]struct{})
		}
		childrenByParent[parent][child] = struct{}{}
	}

	for _, spec := range readmeSpecs {
		addChild(spec.ParentDir, spec.ChildName)
	}

	for sectionDir := range sectionDirs {
		sectionDir = normalizeDocDir(sectionDir)
		if sectionDir == "." {
			continue
		}
		parent := normalizeDocDir(filepath.ToSlash(filepath.Dir(sectionDir)))
		addChild(parent, path.Base(sectionDir))
	}

	orders := make(map[string]int)
	for parent, childrenSet := range childrenByParent {
		children := make([]string, 0, len(childrenSet))
		for child := range childrenSet {
			children = append(children, child)
		}
		sort.Slice(children, func(i, j int) bool {
			if strings.EqualFold(children[i], "readme") &&
				!strings.EqualFold(children[j], "readme") {
				return true
			}
			if strings.EqualFold(children[j], "readme") &&
				!strings.EqualFold(children[i], "readme") {
				return false
			}
			a := strings.ToLower(children[i])
			b := strings.ToLower(children[j])
			if a == b {
				return children[i] < children[j]
			}
			return a < b
		})
		for idx, child := range children {
			orders[siblingOrderKey(parent, child)] = idx + 1
		}
	}

	return orders
}

func siblingOrderKey(parent, child string) string {
	return normalizeDocDir(
		parent,
	) + "\x00" + strings.ToLower(
		strings.TrimSpace(child),
	)
}

func lookupSiblingOrder(
	orders map[string]int,
	parent, child string,
) (int, bool) {
	order, ok := orders[siblingOrderKey(parent, child)]
	return order, ok
}

func readmeSitemapPlacement(
	dir string,
	hasReadmeChildren bool,
) (parentDir string, childName string) {
	dir = normalizeDocDir(dir)
	if dir == "." {
		return ".", "readme"
	}
	if hasReadmeChildren {
		return dir, "readme"
	}
	parent := normalizeDocDir(filepath.ToSlash(filepath.Dir(dir)))
	return parent, path.Base(dir)
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules":
		return true
	default:
		return false
	}
}

func overviewTargetPath(
	readmeRel, outRoot string,
	hasReadmeChildren bool,
) string {
	dir := normalizeDocDir(filepath.ToSlash(filepath.Dir(readmeRel)))
	if dir == "." || hasReadmeChildren {
		if dir == "." {
			return filepath.Join(outRoot, "readme.md")
		}
		return filepath.Join(outRoot, dir, "readme.md")
	}
	return filepath.Join(outRoot, dir+".md")
}

func normalizeDocDir(dir string) string {
	dir = strings.TrimSpace(filepath.ToSlash(dir))
	if dir == "" || dir == "." || dir == "/" {
		return "."
	}
	clean := strings.TrimPrefix(path.Clean("/"+dir), "/")
	if clean == "" || clean == "." {
		return "."
	}
	return clean
}

func dirHasReadmeChildren(
	dir string,
	dirsWithReadmeChildren map[string]struct{},
) bool {
	_, ok := dirsWithReadmeChildren[normalizeDocDir(dir)]
	return ok
}

func renderReadmeOverviewFile(
	srcPath, repoRoot, modulePath string,
	plan *assetPlan,
	order int,
) ([]byte, readmeDocMeta, error) {
	rel, err := filepath.Rel(repoRoot, srcPath)
	if err != nil {
		return nil, readmeDocMeta{}, fmt.Errorf(
			"rel path for %s: %w",
			srcPath,
			err,
		)
	}
	rel = filepath.ToSlash(rel)

	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, readmeDocMeta{}, fmt.Errorf(
			"read source README %s: %w",
			srcPath,
			err,
		)
	}

	raw = normalizeNewlines(raw)
	md, err := rewriteContent(string(raw), rel, plan)
	if err != nil {
		return nil, readmeDocMeta{}, err
	}

	title := readmeTitle(rel, modulePath)
	description := readmeDescriptionPath(rel)
	meta := readmeDocMeta{
		Source: rel,
	}

	var out strings.Builder
	out.WriteString("---\n")
	out.WriteString("title: ")
	out.WriteString(yamlQuoted(title))
	out.WriteString("\n")
	out.WriteString("description: ")
	out.WriteString(yamlQuoted(description))
	out.WriteString("\n")
	out.WriteString("source: ")
	out.WriteString(yamlQuoted(rel))
	out.WriteString("\n")
	if order > 0 {
		out.WriteString("order: ")
		out.WriteString(fmt.Sprintf("%d", order))
		out.WriteString("\n")
	}
	out.WriteString("---\n\n")
	out.WriteString(generatedMarker)
	out.WriteString("\n\n")
	out.WriteString(md)
	if !strings.HasSuffix(md, "\n") {
		out.WriteString("\n")
	}

	return []byte(out.String()), meta, nil
}

func writeSectionIndexes(
	outRoot string,
	sectionDirs map[string]struct{},
	readmeMeta map[string]readmeDocMeta,
	expected map[string]struct{},
	siblingOrders map[string]int,
) (int, error) {
	written := 0

	orderedSectionDirs := make([]string, 0, len(sectionDirs))
	for dir := range sectionDirs {
		orderedSectionDirs = append(orderedSectionDirs, dir)
	}
	sort.Slice(orderedSectionDirs, func(i, j int) bool {
		a := normalizeDocDir(orderedSectionDirs[i])
		b := normalizeDocDir(orderedSectionDirs[j])
		if a == b {
			return false
		}
		return a < b
	})

	for _, dir := range orderedSectionDirs {
		target := sectionIndexTargetPath(outRoot, dir)
		expected[target] = struct{}{}

		meta, hasReadme := readmeMeta[dir]
		order, hasOrder := sectionOrderForDir(dir, siblingOrders)
		content := renderSectionIndex(dir, meta, hasReadme, order, hasOrder)
		changed, err := writeIfChanged(target, content)
		if err != nil {
			return written, err
		}
		if changed {
			written++
		}
	}

	return written, nil
}

func sectionIndexTargetPath(outRoot, dir string) string {
	if dir == "." {
		return filepath.Join(outRoot, "_index.md")
	}
	return filepath.Join(outRoot, dir, "_index.md")
}

func renderSectionIndex(
	dir string,
	readmeMeta readmeDocMeta,
	hasReadme bool,
	order int,
	hasOrder bool,
) []byte {
	title := sectionTitleForDir(dir)
	description := sectionDescriptionForDir(dir)
	var out strings.Builder
	out.WriteString("---\n")
	out.WriteString("title: ")
	out.WriteString(yamlQuoted(title))
	out.WriteString("\n")
	out.WriteString("description: ")
	out.WriteString(yamlQuoted(description))
	out.WriteString("\n")
	if hasOrder {
		out.WriteString("order: ")
		out.WriteString(fmt.Sprintf("%d", order))
		out.WriteString("\n")
	}
	if hasReadme && readmeMeta.Source != "" {
		out.WriteString("source: ")
		out.WriteString(yamlQuoted(readmeMeta.Source))
		out.WriteString("\n")
	}
	out.WriteString("---\n\n")
	out.WriteString(generatedMarker)
	out.WriteString("\n\n")
	out.WriteString("# ")
	out.WriteString(title)
	out.WriteString("\n\n")
	out.WriteString(description)
	out.WriteString("\n")

	return []byte(out.String())
}

func sectionOrderForDir(dir string, siblingOrders map[string]int) (int, bool) {
	if dir == "." || dir == "" || dir == "/" {
		return 1, true
	}
	parent := normalizeDocDir(filepath.ToSlash(filepath.Dir(dir)))
	child := path.Base(normalizeDocDir(dir))
	return lookupSiblingOrder(siblingOrders, parent, child)
}

func sectionTitleForDir(dir string) string {
	if dir == "." || dir == "" || dir == "/" {
		return "Reference"
	}
	return path.Base(dir)
}

func sectionDescriptionForDir(dir string) string {
	if normalizeDocDir(dir) == "." {
		return "Browse README references across the repository."
	}
	return refDirDescription(dir)
}

func normalizeNewlines(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	b = bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
	return b
}

func rewriteContentWithPublicAssetMap(
	md string,
	assetMap map[string]string,
	publicPathPrefix string,
) string {
	hasTrailingNewline := strings.HasSuffix(md, "\n")
	md = strings.TrimSuffix(md, "\n")
	if md == "" {
		if hasTrailingNewline {
			return "\n"
		}
		return ""
	}

	lines := strings.Split(md, "\n")
	outLines := make([]string, len(lines))
	inFence := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			outLines[i] = line
			continue
		}
		if inFence {
			outLines[i] = line
			continue
		}

		rewritten := rewriteHTMLAttrsWithPublicAssetMap(
			line,
			assetMap,
			publicPathPrefix,
		)
		rewritten = rewriteMarkdownWithPublicAssetMap(
			rewritten,
			assetMap,
			publicPathPrefix,
		)
		outLines[i] = rewritten
	}

	out := strings.Join(outLines, "\n")
	if hasTrailingNewline {
		out += "\n"
	}
	return out
}

func rewriteHTMLAttrsWithPublicAssetMap(
	line string,
	assetMap map[string]string,
	publicPathPrefix string,
) string {
	out := htmlAttrDoubleURLRe.ReplaceAllStringFunc(
		line,
		func(match string) string {
			parts := htmlAttrDoubleURLRe.FindStringSubmatch(match)
			if len(parts) != 3 {
				return match
			}
			rewritten, ok := rewritePublicURL(
				parts[2],
				assetMap,
				publicPathPrefix,
			)
			if !ok {
				return match
			}
			return strings.Replace(match, parts[2], rewritten, 1)
		},
	)

	out = htmlAttrSingleURLRe.ReplaceAllStringFunc(
		out,
		func(match string) string {
			parts := htmlAttrSingleURLRe.FindStringSubmatch(match)
			if len(parts) != 3 {
				return match
			}
			rewritten, ok := rewritePublicURL(
				parts[2],
				assetMap,
				publicPathPrefix,
			)
			if !ok {
				return match
			}
			return strings.Replace(match, parts[2], rewritten, 1)
		},
	)

	return out
}

func rewriteMarkdownWithPublicAssetMap(
	line string,
	assetMap map[string]string,
	publicPathPrefix string,
) string {
	return markdownLinkURLRe.ReplaceAllStringFunc(
		line,
		func(match string) string {
			parts := markdownLinkURLRe.FindStringSubmatch(match)
			if len(parts) != 4 {
				return match
			}
			target := strings.TrimSpace(parts[2])
			if strings.Contains(target, " ") {
				return match
			}
			rewritten, ok := rewritePublicURL(
				target,
				assetMap,
				publicPathPrefix,
			)
			if !ok {
				return match
			}
			return parts[1] + rewritten + parts[3]
		},
	)
}

func rewritePublicURL(
	target string,
	assetMap map[string]string,
	publicPathPrefix string,
) (string, bool) {
	pathPart, suffix := splitPathAndSuffix(strings.TrimSpace(target))
	if pathPart == "" || !strings.HasPrefix(pathPart, "/") ||
		strings.HasPrefix(pathPart, "//") {
		return "", false
	}

	lower := strings.ToLower(pathPart)
	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "data:") {
		return "", false
	}

	key := strings.TrimPrefix(path.Clean(pathPart), "/")
	if key == "" || key == "." {
		return "", false
	}

	distName, ok := assetMap[key]
	if !ok {
		return "", false
	}

	resolved := path.Join(publicPathPrefix, distName)
	if !strings.HasPrefix(resolved, "/") {
		resolved = "/" + resolved
	}
	return resolved + suffix, true
}

func rewriteContent(md, readmeRel string, plan *assetPlan) (string, error) {
	hasTrailingNewline := strings.HasSuffix(md, "\n")
	md = strings.TrimSuffix(md, "\n")
	if md == "" {
		if hasTrailingNewline {
			return "\n", nil
		}
		return "", nil
	}

	lines := strings.Split(md, "\n")
	outLines := make([]string, len(lines))
	inFence := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			outLines[i] = line
			continue
		}
		if inFence {
			outLines[i] = line
			continue
		}

		rewritten, err := rewriteHTMLSrc(line, readmeRel, plan)
		if err != nil {
			return "", err
		}
		rewritten, err = rewriteMarkdownLinks(rewritten, readmeRel, plan)
		if err != nil {
			return "", err
		}
		outLines[i] = rewritten
	}

	out := strings.Join(outLines, "\n")
	if hasTrailingNewline {
		out += "\n"
	}
	return out, nil
}

func rewriteHTMLSrc(line, readmeRel string, plan *assetPlan) (string, error) {
	var rewriteErr error
	out := htmlSrcURLRe.ReplaceAllStringFunc(line, func(match string) string {
		if rewriteErr != nil {
			return match
		}
		parts := htmlSrcURLRe.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		original := strings.TrimSpace(parts[1])
		rewritten, ok, err := rewriteURLTarget(original, readmeRel, plan, true)
		if err != nil {
			rewriteErr = err
			return match
		}
		if !ok {
			return match
		}
		return strings.Replace(match, `"`+original+`"`, `"`+rewritten+`"`, 1)
	})
	if rewriteErr != nil {
		return "", rewriteErr
	}
	return out, nil
}

func rewriteMarkdownLinks(
	line, readmeRel string,
	plan *assetPlan,
) (string, error) {
	var rewriteErr error
	out := markdownLinkURLRe.ReplaceAllStringFunc(
		line,
		func(match string) string {
			if rewriteErr != nil {
				return match
			}
			parts := markdownLinkURLRe.FindStringSubmatch(match)
			if len(parts) != 4 {
				return match
			}
			target := strings.TrimSpace(parts[2])
			if strings.Contains(target, " ") {
				return match
			}

			isImage := strings.HasPrefix(parts[1], "![")
			rewritten, ok, err := rewriteURLTarget(
				target,
				readmeRel,
				plan,
				isImage,
			)
			if err != nil {
				rewriteErr = err
				return match
			}
			if !ok {
				return match
			}
			return parts[1] + rewritten + parts[3]
		},
	)
	if rewriteErr != nil {
		return "", rewriteErr
	}
	return out, nil
}

func rewriteURLTarget(
	target, readmeRel string,
	plan *assetPlan,
	assetContext bool,
) (string, bool, error) {
	pathPart, suffix := splitPathAndSuffix(strings.TrimSpace(target))
	if pathPart == "" {
		return "", false, nil
	}

	lower := strings.ToLower(pathPart)
	if filepath.IsAbs(pathPart) {
		relPath, ok := absPathToRepoRel(pathPart, plan.repoRoot)
		if !ok {
			if assetContext {
				return "", false, fmt.Errorf(
					"asset path outside repo is not allowed: %s (from %s)",
					target,
					readmeRel,
				)
			}
			return "", false, nil
		}
		return rewriteResolvedRepoPath(
			relPath,
			suffix,
			readmeRel,
			plan,
			assetContext,
		)
	}

	if strings.HasPrefix(pathPart, "#") || strings.HasPrefix(pathPart, "/") {
		return "", false, nil
	}

	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") {
		if assetContext {
			return "", false, fmt.Errorf(
				"third-party asset URLs are not allowed: %s (from %s)",
				target,
				readmeRel,
			)
		}
		return "", false, nil
	}

	if strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "data:") {
		return "", false, nil
	}

	if strings.Contains(pathPart, "://") {
		if assetContext {
			return "", false, fmt.Errorf(
				"third-party asset URLs are not allowed: %s (from %s)",
				target,
				readmeRel,
			)
		}
		return "", false, nil
	}

	baseDir := filepath.ToSlash(filepath.Dir(readmeRel))
	if baseDir == "." {
		baseDir = ""
	}
	resolved := path.Clean(path.Join("/", baseDir, pathPart))
	resolved = strings.TrimPrefix(resolved, "/")
	if resolved == "" || resolved == "." {
		return "", false, nil
	}

	return rewriteResolvedRepoPath(
		resolved,
		suffix,
		readmeRel,
		plan,
		assetContext,
	)
}

func splitPathAndSuffix(target string) (pathPart, suffix string) {
	if idx := strings.IndexAny(target, "?#"); idx >= 0 {
		return target[:idx], target[idx:]
	}
	return target, ""
}

func absPathToRepoRel(absPath, repoRoot string) (string, bool) {
	absClean := filepath.Clean(absPath)
	rootClean := filepath.Clean(repoRoot)
	rel, err := filepath.Rel(rootClean, absClean)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

func rewriteResolvedRepoPath(
	resolved, suffix, readmeRel string,
	plan *assetPlan,
	assetContext bool,
) (string, bool, error) {
	resolved = filepath.ToSlash(path.Clean("/" + resolved))
	resolved = strings.TrimPrefix(resolved, "/")
	if resolved == "" || resolved == "." {
		return "", false, nil
	}

	if strings.EqualFold(path.Base(resolved), "README.md") {
		return docsReadmeURLForDir(path.Dir(resolved), plan) + suffix, true, nil
	}

	if _, ok := plan.readmeDirs[normalizeDocDir(resolved)]; ok {
		return docsURLForDir(resolved) + suffix, true, nil
	}

	sourceAbs := filepath.Join(plan.repoRoot, filepath.FromSlash(resolved))
	info, err := os.Stat(sourceAbs)
	if err != nil {
		if os.IsNotExist(err) {
			if assetContext {
				return "", false, fmt.Errorf(
					"asset file not found: %s (from %s)",
					resolved,
					readmeRel,
				)
			}
			return "", false, nil
		}
		return "", false, fmt.Errorf("stat path %s: %w", sourceAbs, err)
	}

	if info.IsDir() {
		if assetContext {
			return "", false, fmt.Errorf(
				"asset path points to a directory: %s (from %s)",
				resolved,
				readmeRel,
			)
		}
		return "", false, nil
	}

	if publicKey, ok := toPublicAssetKey(resolved); ok {
		return "/" + publicKey + suffix, true, nil
	}

	mirrorURL, err := plan.registerAsset(resolved, sourceAbs)
	if err != nil {
		return "", false, err
	}
	return mirrorURL + suffix, true, nil
}

func toPublicAssetKey(resolved string) (string, bool) {
	resolved = filepath.ToSlash(resolved)
	if !strings.HasPrefix(resolved, publicAssetsRootRel+"/") {
		return "", false
	}
	key := strings.TrimPrefix(resolved, publicAssetsRootRel+"/")
	key = strings.TrimPrefix(key, "/")
	if key == "" || key == "." {
		return "", false
	}
	return key, true
}

func docsURLForDir(dir string) string {
	cleanDir := normalizeDocDir(dir)
	if cleanDir == "." || cleanDir == "/" {
		return docsReferenceURLRoot
	}
	return docsReferenceURLRoot + "/" + strings.TrimPrefix(cleanDir, "./")
}

func docsReadmeURLForDir(dir string, plan *assetPlan) string {
	cleanDir := normalizeDocDir(dir)
	if !dirHasReadmeChildren(cleanDir, plan.dirsWithReadmeChildren) {
		return docsURLForDir(cleanDir)
	}
	return docsURLForDir(cleanDir) + "/readme"
}

func syncMirrorAssets(plan *assetPlan) (written, deleted int, err error) {
	keys := make([]string, 0, len(plan.expected))
	for dest := range plan.expected {
		keys = append(keys, dest)
	}
	sort.Strings(keys)

	for _, dest := range keys {
		src := plan.expected[dest]
		content, err := os.ReadFile(src)
		if err != nil {
			return written, deleted, fmt.Errorf(
				"read asset source %s: %w",
				src,
				err,
			)
		}
		changed, err := writeIfChanged(dest, content)
		if err != nil {
			return written, deleted, err
		}
		if changed {
			written++
		}
	}

	info, err := os.Stat(plan.mirrorRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return written, deleted, nil
		}
		return written, deleted, fmt.Errorf(
			"stat mirror root %s: %w",
			plan.mirrorRoot,
			err,
		)
	}
	if !info.IsDir() {
		return written, deleted, fmt.Errorf(
			"mirror root is not a directory: %s",
			plan.mirrorRoot,
		)
	}

	err = filepath.WalkDir(
		plan.mirrorRoot,
		func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if _, ok := plan.expected[p]; ok {
				return nil
			}
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("remove stale mirrored asset %s: %w", p, err)
			}
			deleted++
			return nil
		},
	)
	if err != nil {
		return written, deleted, err
	}

	if err := pruneEmptyDirs(plan.mirrorRoot); err != nil {
		return written, deleted, err
	}

	return written, deleted, nil
}

func removeLegacyMirrorRoot(repoRoot string) error {
	legacy := filepath.Join(repoRoot, legacyMirrorRootRel)
	if err := os.RemoveAll(legacy); err != nil {
		return fmt.Errorf(
			"remove legacy mirrored assets under %s: %w",
			legacy,
			err,
		)
	}
	return nil
}

func readModulePath(repoRoot string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
		if modulePath == "" {
			break
		}
		return modulePath, nil
	}
	return "", fmt.Errorf("module path not found in go.mod")
}

func readmeTitle(readmeRel, modulePath string) string {
	if readmeRel == "README.md" {
		return path.Base(modulePath)
	}
	dir := normalizeDocDir(filepath.ToSlash(filepath.Dir(readmeRel)))
	if dir == "." {
		return path.Base(modulePath)
	}
	return path.Base(dir)
}

func readmeDescriptionPath(readmeRel string) string {
	dir := normalizeDocDir(filepath.ToSlash(filepath.Dir(readmeRel)))
	return refReadmeDescription(dir)
}

func refDirDescription(dir string) string {
	dir = normalizeDocDir(dir)
	if dir == "." {
		return "~/"
	}
	return "~/" + dir + "/"
}

func refReadmeDescription(dir string) string {
	dir = normalizeDocDir(dir)
	if dir == "." {
		return "~"
	}
	return "~/" + dir
}

func yamlQuoted(s string) string {
	s = strings.ReplaceAll(s, `\\`, `\\\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	return `"` + s + `"`
}

func writeIfChanged(path string, content []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read existing output %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("mkdir output dir for %s: %w", path, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0644); err != nil {
		return false, fmt.Errorf("write temp output %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return false, fmt.Errorf("rename temp output to %s: %w", path, err)
	}
	return true, nil
}

func deleteStaleGeneratedDocs(
	outRoot string,
	expected map[string]struct{},
) (int, error) {
	info, err := os.Stat(outRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat output root %s: %w", outRoot, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("output root is not a directory: %s", outRoot)
	}

	deleted := 0
	err = filepath.WalkDir(
		outRoot,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".md" {
				return nil
			}
			if _, ok := expected[path]; ok {
				return nil
			}

			b, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read stale candidate %s: %w", path, err)
			}
			if !bytes.Contains(b, []byte(generatedMarker)) {
				return nil
			}

			if err := os.Remove(path); err != nil {
				return fmt.Errorf(
					"remove stale generated file %s: %w",
					path,
					err,
				)
			}
			deleted++
			return nil
		},
	)
	if err != nil {
		return deleted, fmt.Errorf("walk output root %s: %w", outRoot, err)
	}

	return deleted, nil
}

func pruneEmptyDirs(root string) error {
	var dirs []string
	err := filepath.WalkDir(
		root,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				dirs = append(dirs, path)
			}
			return nil
		},
	)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("walk dirs for pruning: %w", err)
	}

	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})

	for _, dir := range dirs {
		if dir == root {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read dir %s for pruning: %w", dir, err)
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove empty dir %s: %w", dir, err)
			}
		}
	}

	return nil
}
