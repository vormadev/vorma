package fsmarkdown

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func new_test_instance(t *testing.T, file_system fs.FS) *Instance {
	t.Helper()
	return New(Options{
		FS:    file_system,
		IsDev: true,
		FrontmatterParser: func(r io.Reader, dest any) ([]byte, error) {
			raw, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			parts := strings.SplitN(string(raw), "---", 3)
			if len(parts) == 3 && strings.TrimSpace(parts[0]) == "" {
				fake_yaml_into(parts[1], dest)
				return []byte(strings.TrimLeft(parts[2], "\n")), nil
			}
			return raw, nil
		},
		MarkdownParser: func(src []byte, w io.Writer) error {
			_, err := w.Write(src)
			return err
		},
	})
}

// fake_yaml_into parses flat "key: value" lines into a struct via
// reflection, matching on yaml tags. Good enough for tests.
func fake_yaml_into(block string, dest any) {
	v := reflect.ValueOf(dest).Elem()
	t := v.Type()

	tag_map := map[string]int{}
	for i := 0; i < t.NumField(); i++ {
		if tag := t.Field(i).Tag.Get("yaml"); tag != "" {
			tag_map[tag] = i
		}
	}

	for line := range strings.SplitSeq(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		idx, ok := tag_map[key]
		if !ok {
			continue
		}
		field := v.Field(idx)
		switch field.Kind() {
		case reflect.String:
			field.SetString(val)
		case reflect.Int:
			if n, err := strconv.Atoi(val); err == nil {
				field.SetInt(int64(n))
			}
		}
	}
}

func new_simple_instance(t *testing.T, file_system fs.FS) *Instance {
	t.Helper()
	return New(Options{
		FS:    file_system,
		IsDev: true,
		FrontmatterParser: func(r io.Reader, _ any) ([]byte, error) {
			return io.ReadAll(r)
		},
		MarkdownParser: func(src []byte, w io.Writer) error {
			_, err := w.Write(src)
			return err
		},
	})
}

func must_lookup(t *testing.T, inst *Instance, path string) *Result {
	t.Helper()
	r, found, err := inst.Lookup(path)
	if err != nil {
		t.Fatalf("Lookup(%q) error: %v", path, err)
	}
	if !found {
		t.Fatalf("Lookup(%q) not found", path)
	}
	return r
}

// ---------------------------------------------------------------------------
// New() panics
// ---------------------------------------------------------------------------

func TestNew_PanicsWithoutFS(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when FS is nil")
		}
	}()
	New(Options{
		FrontmatterParser: func(r io.Reader, _ any) ([]byte, error) { return io.ReadAll(r) },
		MarkdownParser:    func(_ []byte, _ io.Writer) error { return nil },
	})
}

func TestNew_PanicsWithoutFrontmatterParser(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when FrontmatterParser is nil")
		}
	}()
	New(Options{
		FS:             fstest.MapFS{},
		MarkdownParser: func(_ []byte, _ io.Writer) error { return nil },
	})
}

func TestNew_PanicsWithoutMarkdownParser(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when MarkdownParser is nil")
		}
	}()
	New(Options{
		FS:                fstest.MapFS{},
		FrontmatterParser: func(r io.Reader, _ any) ([]byte, error) { return io.ReadAll(r) },
	})
}

// ---------------------------------------------------------------------------
// Lookup: basic page resolution
// ---------------------------------------------------------------------------

func TestLookup_RootIndex(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home content")},
	})

	r := must_lookup(t, inst, "/")

	if r.Page.HTML != "home content" {
		t.Fatalf("HTML = %q, want %q", r.Page.HTML, "home content")
	}
	if !r.Page.IsFolder {
		t.Fatal("root index should be IsFolder")
	}
}

func TestLookup_RegularPage(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"about.md":  {Data: []byte("about page")},
	})

	r := must_lookup(t, inst, "/about")

	if r.Page.HTML != "about page" {
		t.Fatalf("HTML = %q, want %q", r.Page.HTML, "about page")
	}
	if r.Page.IsFolder {
		t.Fatal("regular page should not be IsFolder")
	}
	if r.Page.URL != "/about" {
		t.Fatalf("URL = %q, want %q", r.Page.URL, "/about")
	}
}

func TestLookup_FolderIndex(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md":               {Data: []byte("home")},
		"docs/_index.md":          {Data: []byte("docs index")},
		"docs/getting-started.md": {Data: []byte("getting started")},
	})

	r := must_lookup(t, inst, "/docs")

	if !r.Page.IsFolder {
		t.Fatal("folder index should be IsFolder")
	}
	if r.Page.HTML != "docs index" {
		t.Fatalf("HTML = %q, want %q", r.Page.HTML, "docs index")
	}
}

func TestLookup_NotFound(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
	})

	r, found, err := inst.Lookup("/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found=false for missing path")
	}
	if r.Page != nil {
		t.Fatal("expected nil Page for missing path")
	}
}

func TestLookup_DotWellKnownReturnsNotFound(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
	})

	_, found, err := inst.Lookup(
		"/.well-known/appspecific/com.chrome.devtools.json",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found=false for .well-known path")
	}
}

func TestLookup_NestedPage(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md":              {Data: []byte("home")},
		"docs/_index.md":         {Data: []byte("docs")},
		"docs/guides/_index.md":  {Data: []byte("guides")},
		"docs/guides/install.md": {Data: []byte("install")},
	})

	r := must_lookup(t, inst, "/docs/guides/install")

	if r.Page.HTML != "install" {
		t.Fatalf("HTML = %q, want %q", r.Page.HTML, "install")
	}
}

// ---------------------------------------------------------------------------
// Lookup: siblings
// ---------------------------------------------------------------------------

func TestLookup_SiblingsIncludesHomeAtRoot(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"about.md":  {Data: []byte("about")},
		"blog.md":   {Data: []byte("blog")},
	})

	r := must_lookup(t, inst, "/about")

	if len(r.Siblings) == 0 {
		t.Fatal("expected siblings")
	}
	if r.Siblings[0].Title != "Home" || r.Siblings[0].URL != "/" {
		t.Fatalf("first sibling should be Home, got %+v", r.Siblings[0])
	}
}

func TestLookup_SiblingsMarkActive(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"about.md":  {Data: []byte("about")},
		"blog.md":   {Data: []byte("blog")},
	})

	r := must_lookup(t, inst, "/about")

	active_count := 0
	for _, s := range r.Siblings {
		if s.IsActive {
			active_count++
			if s.URL != "/about" {
				t.Fatalf("wrong item marked active: %+v", s)
			}
		}
	}
	if active_count != 1 {
		t.Fatalf("expected exactly 1 active sibling, got %d", active_count)
	}
}

func TestLookup_SiblingsExcludesIndex(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"about.md":  {Data: []byte("about")},
	})

	r := must_lookup(t, inst, "/about")

	for _, s := range r.Siblings {
		if strings.Contains(s.URL, "_index") {
			t.Fatalf("siblings should not contain _index: %+v", s)
		}
	}
}

func TestLookup_SiblingsIgnoresNonMarkdownFiles(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"about.md":  {Data: []byte("about")},
		"image.png": {Data: []byte("not markdown")},
		"style.css": {Data: []byte("not markdown")},
	})

	r := must_lookup(t, inst, "/about")

	for _, s := range r.Siblings {
		if s.URL == "/" {
			continue
		}
		if s.URL != "/about" {
			t.Fatalf("unexpected sibling: %+v", s)
		}
	}
}

// ---------------------------------------------------------------------------
// Lookup: children
// ---------------------------------------------------------------------------

func TestLookup_ChildrenPopulatedForFolder(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md":      {Data: []byte("home")},
		"docs/_index.md": {Data: []byte("docs")},
		"docs/intro.md":  {Data: []byte("intro")},
		"docs/setup.md":  {Data: []byte("setup")},
	})

	r := must_lookup(t, inst, "/docs")

	if len(r.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(r.Children))
	}
}

func TestLookup_ChildrenEmptyForRegularPage(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"about.md":  {Data: []byte("about")},
	})

	r := must_lookup(t, inst, "/about")

	if len(r.Children) != 0 {
		t.Fatalf(
			"expected no children for regular page, got %d",
			len(r.Children),
		)
	}
}

func TestLookup_ChildrenEmptyForRoot(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"about.md":  {Data: []byte("about")},
	})

	r := must_lookup(t, inst, "/")

	if len(r.Children) != 0 {
		t.Fatalf(
			"expected no children for root (root uses siblings), got %d",
			len(r.Children),
		)
	}
}

func TestLookup_ChildrenMarksFolders(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md":             {Data: []byte("home")},
		"docs/_index.md":        {Data: []byte("docs")},
		"docs/intro.md":         {Data: []byte("intro")},
		"docs/guides/_index.md": {Data: []byte("guides")},
	})

	r := must_lookup(t, inst, "/docs")

	folder_count := 0
	for _, c := range r.Children {
		if c.IsFolder {
			folder_count++
		}
	}
	if folder_count != 1 {
		t.Fatalf("expected 1 folder child, got %d", folder_count)
	}
}

// ---------------------------------------------------------------------------
// Lookup: ParentURL
// ---------------------------------------------------------------------------

func TestLookup_ParentURL_SetForNestedPage(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md":      {Data: []byte("home")},
		"docs/_index.md": {Data: []byte("docs")},
		"docs/intro.md":  {Data: []byte("intro")},
	})

	r := must_lookup(t, inst, "/docs/intro")

	if r.ParentURL != "/docs" {
		t.Fatalf("ParentURL = %q, want %q", r.ParentURL, "/docs")
	}
}

func TestLookup_ParentURL_EmptyAtRoot(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
	})

	r := must_lookup(t, inst, "/")

	if r.ParentURL != "" {
		t.Fatalf("ParentURL = %q, want empty at root", r.ParentURL)
	}
}

func TestLookup_ParentURL_EmptyForTopLevelPage(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"about.md":  {Data: []byte("about")},
	})

	r := must_lookup(t, inst, "/about")

	if r.ParentURL != "" {
		t.Fatalf("ParentURL = %q, want empty for top-level page", r.ParentURL)
	}
}

// ---------------------------------------------------------------------------
// Frontmatter
// ---------------------------------------------------------------------------

func TestLookup_ParsesFrontmatter(t *testing.T) {
	inst := new_test_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"post.md": {Data: []byte(
			"---\ntitle: My Post\ndescription: A great post\ndate: 2025-01-15\norder: 3\n---\nBody here",
		)},
	})

	r := must_lookup(t, inst, "/post")

	if r.Page.Title != "My Post" {
		t.Fatalf("Title = %q, want %q", r.Page.Title, "My Post")
	}
	if r.Page.Description != "A great post" {
		t.Fatalf(
			"Description = %q, want %q",
			r.Page.Description,
			"A great post",
		)
	}
	if r.Page.Date != "2025-01-15" {
		t.Fatalf("Date = %q, want %q", r.Page.Date, "2025-01-15")
	}
	if r.Page.Order != 3 {
		t.Fatalf("Order = %d, want %d", r.Page.Order, 3)
	}
	if r.Page.HTML != "Body here" {
		t.Fatalf("HTML = %q, want %q", r.Page.HTML, "Body here")
	}
}

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------

func TestLookup_SortsByOrderThenDate(t *testing.T) {
	inst := new_test_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"a.md": {
			Data: []byte("---\ntitle: A\ndate: 2025-01-01\n---\n"),
		},
		"b.md": {
			Data: []byte("---\ntitle: B\ndate: 2025-03-01\n---\n"),
		},
		"c.md": {Data: []byte("---\ntitle: C\norder: 2\n---\n")},
		"d.md": {Data: []byte("---\ntitle: D\norder: 1\n---\n")},
	})

	r := must_lookup(t, inst, "/a")

	// Skip "Home" at index 0.
	titles := []string{}
	for _, s := range r.Siblings {
		if s.URL == "/" {
			continue
		}
		titles = append(titles, s.Title)
	}

	// Ordered items first (D=1, C=2), then by date descending (B=March, A=Jan).
	expected := []string{"D", "C", "B", "A"}
	if len(titles) != len(expected) {
		t.Fatalf("got %v, want %v", titles, expected)
	}
	for i, title := range titles {
		if title != expected[i] {
			t.Fatalf(
				"position %d: got %q, want %q (full: %v)",
				i,
				title,
				expected[i],
				titles,
			)
		}
	}
}

// ---------------------------------------------------------------------------
// NavItem title fallback
// ---------------------------------------------------------------------------

func TestLookup_NavItemFallsBackToFilename(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md":   {Data: []byte("home")},
		"untitled.md": {Data: []byte("no frontmatter")},
	})

	r := must_lookup(t, inst, "/untitled")

	found := false
	for _, s := range r.Siblings {
		if s.URL == "/untitled" {
			found = true
			if s.Title != "untitled" {
				t.Fatalf(
					"expected fallback title %q, got %q",
					"untitled",
					s.Title,
				)
			}
		}
	}
	if !found {
		t.Fatal("untitled page not found in siblings")
	}
}

// ---------------------------------------------------------------------------
// PlainMarkdown
// ---------------------------------------------------------------------------

func TestPlainMarkdown_IncludesTitleHeading(t *testing.T) {
	inst := new_test_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"post.md":   {Data: []byte("---\ntitle: Hello\n---\nWorld")},
	})

	md, err := inst.PlainMarkdown("/post")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(md, "# Hello\n") {
		t.Fatalf("expected title heading, got %q", md)
	}
	if !strings.Contains(md, "World") {
		t.Fatalf("expected body content, got %q", md)
	}
}

func TestPlainMarkdown_EmptyForNotFound(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
	})

	md, err := inst.PlainMarkdown("/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if md != "" {
		t.Fatalf("expected empty string for missing page, got %q", md)
	}
}

func TestPlainMarkdown_OmitsTitleWhenEmpty(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"bare.md":   {Data: []byte("just content")},
	})

	md, err := inst.PlainMarkdown("/bare")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasPrefix(md, "#") {
		t.Fatalf("expected no title heading, got %q", md)
	}
}

// ---------------------------------------------------------------------------
// PlainTextMiddleware
// ---------------------------------------------------------------------------

func TestPlainTextMiddleware_ServesPlainText(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("hello from markdown")},
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	handler := inst.PlainTextMiddleware("/*")(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", ct, "text/plain; charset=utf-8")
	}
	if !strings.Contains(rec.Body.String(), "hello from markdown") {
		t.Fatalf("body = %q, expected markdown content", rec.Body.String())
	}
}

func TestPlainTextMiddleware_CaseInsensitiveAcceptHeader(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("content")},
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	handler := inst.PlainTextMiddleware("/*")(next)

	for _, accept := range []string{"TEXT/PLAIN", "Text/Plain", "text/MARKDOWN", "TEXT/markdown"} {
		t.Run(accept, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf(
					"Accept=%q: status = %d, want %d",
					accept,
					rec.Code,
					http.StatusOK,
				)
			}
		})
	}
}

func TestPlainTextMiddleware_PassesThroughForHTMLAccept(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("content")},
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	handler := inst.PlainTextMiddleware("/*")(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf(
			"status = %d, want %d (should pass through)",
			rec.Code,
			http.StatusTeapot,
		)
	}
}

func TestPlainTextMiddleware_PassesThroughForNonMatchingPattern(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("content")},
		"docs.md":   {Data: []byte("docs")},
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	handler := inst.PlainTextMiddleware("/docs")(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf(
			"status = %d, want %d (should pass through)",
			rec.Code,
			http.StatusTeapot,
		)
	}
}

func TestPlainTextMiddleware_MarkdownAcceptHeader(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("content")},
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	handler := inst.PlainTextMiddleware("/*")(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/markdown")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPlainTextMiddleware_Returns404ForMissingMarkdownPage(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("content")},
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	handler := inst.PlainTextMiddleware("/*")(next)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if strings.Contains(rec.Body.String(), "content") {
		t.Fatalf("body = %q, expected not-found response", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Frontmatter in nav items
// ---------------------------------------------------------------------------

func TestLookup_NavItemsCarryFrontmatterFields(t *testing.T) {
	inst := new_test_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"post.md": {Data: []byte(
			"---\ntitle: My Post\ndescription: A summary\ndate: 2025-06-01\n---\nbody",
		)},
	})

	r := must_lookup(t, inst, "/post")

	var nav *NavItem
	for i := range r.Siblings {
		if r.Siblings[i].URL == "/post" {
			nav = &r.Siblings[i]
			break
		}
	}
	if nav == nil {
		t.Fatal("post not found in siblings")
	}
	if nav.Description != "A summary" {
		t.Fatalf("Description = %q, want %q", nav.Description, "A summary")
	}
	if nav.Date != "2025-06-01" {
		t.Fatalf("Date = %q, want %q", nav.Date, "2025-06-01")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestLookup_PathCleaning(t *testing.T) {
	inst := new_simple_instance(t, fstest.MapFS{
		"_index.md": {Data: []byte("home")},
		"about.md":  {Data: []byte("about")},
	})

	for _, dirty_path := range []string{"/about/", "/about/.", "//about"} {
		t.Run(fmt.Sprintf("path=%s", dirty_path), func(t *testing.T) {
			r, found, err := inst.Lookup(dirty_path)
			if err != nil {
				t.Fatalf("Lookup(%q) error: %v", dirty_path, err)
			}
			if !found {
				t.Fatalf("Lookup(%q) not found", dirty_path)
			}
			if r.Page.URL != "/about" {
				t.Fatalf("URL = %q, want %q", r.Page.URL, "/about")
			}
		})
	}
}
