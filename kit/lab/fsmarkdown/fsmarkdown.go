package fsmarkdown

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/vormadev/vorma/kit/lru"
	"github.com/vormadev/vorma/kit/matcher"
)

type FrontmatterParser = func(io.Reader, any) ([]byte, error)
type MarkdownParser = func([]byte, io.Writer) error

type Options struct {
	FS                fs.FS
	IsDev             bool
	FrontmatterParser FrontmatterParser
	MarkdownParser    MarkdownParser
}

type Page struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Date        string `yaml:"date"`
	Order       int    `yaml:"order"`
	HTML        string
	Raw         string
	URL         string
	IsFolder    bool
}

type NavItem struct {
	Title       string
	URL         string
	Description string
	Date        string
	IsFolder    bool
	IsActive    bool
}

type Result struct {
	Page      *Page
	Siblings  []NavItem
	Children  []NavItem
	ParentURL string
}

type Instance struct {
	Options
	page_cache   *lru.Cache[string, *Page]
	result_cache *lru.Cache[string, *Result]
	dir_cache    *lru.Cache[string, *dir_listing]
}

// New creates an Instance. Panics if FS, FrontmatterParser,
// or MarkdownParser are nil.
func New(opts Options) *Instance {
	if opts.FS == nil {
		panic("fsmarkdown: FS is required")
	}
	if opts.FrontmatterParser == nil {
		panic("fsmarkdown: FrontmatterParser is required")
	}
	if opts.MarkdownParser == nil {
		panic("fsmarkdown: MarkdownParser is required")
	}
	return &Instance{
		Options:      opts,
		page_cache:   lru.NewCache[string, *Page](1000),
		result_cache: lru.NewCache[string, *Result](1000),
		dir_cache:    lru.NewCache[string, *dir_listing](1000),
	}
}

// Lookup resolves a clean URL path (e.g. "/docs/intro") to a Result.
// Returns (result, false, nil) if the path does not exist.
func (inst *Instance) Lookup(_path string) (*Result, bool, error) {
	clean := clean_url_path(_path)

	if r, ok := inst.result_cache.Get(clean); ok && !inst.IsDev {
		return r, r.Page != nil, nil
	}

	page, found, err := inst.load_page(clean)
	if err != nil {
		return nil, false, err
	}
	if !found {
		r := &Result{}
		inst.result_cache.Set(clean, r, true)
		return r, false, nil
	}

	parent_dir := path.Dir(clean)

	sibling_listing, err := inst.load_dir(parent_dir)
	if err != nil {
		return nil, false, err
	}

	siblings := make_nav(sibling_listing.pages, clean)
	if parent_dir == "/" || parent_dir == "." {
		home := NavItem{Title: "Home", URL: "/", IsActive: clean == "/"}
		siblings = append([]NavItem{home}, siblings...)
	}

	var children []NavItem
	if page.IsFolder && clean != "/" {
		child_listing, err := inst.load_dir(clean)
		if err != nil {
			return nil, false, err
		}
		children = make_nav(child_listing.pages, clean)
	}

	var parent_url string
	if sibling_listing.has_index && clean != "/" && parent_dir != "/" {
		parent_url = parent_dir
	}

	r := &Result{
		Page:      page,
		Siblings:  siblings,
		Children:  children,
		ParentURL: parent_url,
	}
	inst.result_cache.Set(clean, r, false)
	return r, true, nil
}

// PlainMarkdown returns the raw markdown with a title heading prepended.
func (inst *Instance) PlainMarkdown(path string) (string, error) {
	r, found, err := inst.Lookup(path)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}

	return inst.build_plain_markdown(r.Page), nil
}

func (inst *Instance) build_plain_markdown(page *Page) string {
	var b strings.Builder
	if page.Title != "" {
		b.WriteString("# ")
		b.WriteString(page.Title)
		b.WriteString("\n")
	}
	b.WriteString(page.Raw)
	return b.String()
}

// PlainTextMiddleware serves plain markdown when the Accept header
// includes "text/plain" or "text/markdown" and the path matches
// one of the given patterns (kit/matcher semantics).
func (inst *Instance) PlainTextMiddleware(
	patterns ...string,
) func(http.Handler) http.Handler {
	m := matcher.New(nil)
	for _, p := range patterns {
		m.RegisterPattern(p)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := m.FindBestMatch(r.URL.Path); !ok {
				next.ServeHTTP(w, r)
				return
			}

			accept := strings.ToLower(r.Header.Get("Accept"))
			if !strings.Contains(accept, "text/plain") &&
				!strings.Contains(accept, "text/markdown") {
				next.ServeHTTP(w, r)
				return
			}

			result, found, err := inst.Lookup(r.URL.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if !found {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(inst.build_plain_markdown(result.Page)))
		})
	}
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

type dir_listing struct {
	pages     []*Page
	has_index bool
}

func (inst *Instance) load_page(clean_path string) (*Page, bool, error) {
	if p, ok := inst.page_cache.Get(clean_path); ok && !inst.IsDev {
		return p, p != nil, nil
	}

	is_folder, raw, err := inst.read_file(clean_path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			inst.page_cache.Set(clean_path, nil, true)
			return nil, false, nil
		}
		return nil, false, err
	}

	page, err := inst.parse(raw, clean_path, is_folder)
	if err != nil {
		return nil, false, err
	}

	inst.page_cache.Set(clean_path, page, false)
	return page, true, nil
}

func (inst *Instance) read_file(_clean_path string) (bool, []byte, error) {
	clean_path := normalize_path(_clean_path)

	data, err := fs.ReadFile(inst.FS, clean_path+".md")
	if err == nil {
		return false, data, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, nil, err
	}

	data, err = fs.ReadFile(
		inst.FS, path.Join(clean_path, "_index.md"),
	)
	if err != nil {
		return false, nil, err
	}
	return true, data, nil
}

func (inst *Instance) parse(
	data []byte, clean_path string, is_folder bool,
) (*Page, error) {
	var p Page
	rest, err := inst.FrontmatterParser(bytes.NewReader(data), &p)
	if err != nil {
		return nil, err
	}

	p.Raw = string(rest)

	var buf bytes.Buffer
	if err := inst.MarkdownParser(rest, &buf); err != nil {
		return nil, err
	}
	p.HTML = buf.String()
	p.URL = clean_url_path(clean_path)
	p.IsFolder = is_folder

	return &p, nil
}

func clean_url_path(p string) string {
	p = path.Clean(p)
	if p == "." {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

func normalize_path(p string) string {
	if p == "/" {
		p = "."
	}
	return strings.TrimSuffix(strings.TrimPrefix(p, "/"), "/")
}

func (inst *Instance) load_dir(dir_path string) (*dir_listing, error) {
	dir_path = normalize_path(dir_path)

	if l, ok := inst.dir_cache.Get(dir_path); ok && !inst.IsDev {
		return l, nil
	}

	entries, err := fs.ReadDir(inst.FS, dir_path)
	if err != nil {
		return nil, err
	}

	has_index := false
	var pages []*Page

	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".md")

		if entry.Type().IsRegular() &&
			!strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if name == "_index" {
			has_index = true
			continue
		}

		page, found, err := inst.load_page(path.Join("/", dir_path, name))
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		pages = append(pages, page)
	}

	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Order != 0 && pages[j].Order != 0 {
			return pages[i].Order < pages[j].Order
		}
		if pages[i].Order != 0 {
			return true
		}
		if pages[j].Order != 0 {
			return false
		}
		return pages[i].Date > pages[j].Date
	})

	listing := &dir_listing{pages: pages, has_index: has_index}
	inst.dir_cache.Set(dir_path, listing, false)
	return listing, nil
}

func make_nav(pages []*Page, active_path string) []NavItem {
	items := make([]NavItem, 0, len(pages))
	for _, p := range pages {
		title := p.Title
		if title == "" {
			title = path.Base(p.URL)
		}
		items = append(items, NavItem{
			Title:       title,
			URL:         p.URL,
			Description: p.Description,
			Date:        p.Date,
			IsFolder:    p.IsFolder,
			IsActive:    p.URL == active_path,
		})
	}
	return items
}
