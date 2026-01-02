// buyer beware
package fsmarkdown

import (
	"bytes"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/lru"
	"github.com/vormadev/vorma/kit/typed"
	"golang.org/x/sync/errgroup"
)

type FrontmatterParser = func(io.Reader, any) ([]byte, error)
type MarkdownParser = func([]byte) []byte

// Do not initialize manually. Always create with New().
type Instance struct {
	Options
	pageDetailsCache *lru.Cache[string, *DetailedPage]
	sitemapCache     typed.SyncMap[generateSitemapInput, *generateSitemapInnerData]
	basePageCache    *lru.Cache[string, *Page]
}

type Options struct {
	FS                fs.FS
	FrontmatterParser FrontmatterParser
	MarkdownParser    MarkdownParser
	IsDev             bool
}

func New(opts Options) *Instance {
	if opts.FS == nil {
		log.Fatal("FS is required")
	}
	if opts.FrontmatterParser == nil {
		log.Fatal("FrontmatterParser is required")
	}
	if opts.MarkdownParser == nil {
		log.Fatal("MarkdownParser is required")
	}
	return &Instance{
		Options:          opts,
		pageDetailsCache: lru.NewCache[string, *DetailedPage](1_000),
		sitemapCache:     typed.SyncMap[generateSitemapInput, *generateSitemapInnerData]{},
		basePageCache:    lru.NewCache[string, *Page](1_000),
	}
}

type Page struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Date        string `yaml:"date"`
	Order       int    `yaml:"order"`
	Content     template.HTML
	RawContent  string
	URL         string
	IsFolder    bool
}

type DetailedPage struct {
	*Page
	Sitemap      Sitemap
	IndexSitemap Sitemap
	BackItem     string
}

type SitemapItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	Date        string `json:"date,omitempty"`
	IsFolder    bool   `json:"isFolder,omitempty"`
	IsActive    bool   `json:"isActive,omitempty"`
}

type Sitemap []SitemapItem

func (inst *Instance) GetPageDetails(r *http.Request) (detailedPage *DetailedPage, err error) {
	cleanPath := filepath.Clean(r.URL.Path)

	if p, ok := inst.pageDetailsCache.Get(cleanPath); ok && !inst.IsDev {
		return p, nil
	}

	pageBase, found, err := inst.getPageBase(cleanPath)
	if err != nil {
		log.Println("Error getting pageBase in getPageDetails: ", err)
		return nil, err
	}

	var eg errgroup.Group
	var indexSitemap, sitemap Sitemap
	var backItem string

	if pageBase.IsFolder && cleanPath != "/" {
		eg.Go(func() error {
			sm, err := inst.generateSitemap(generateSitemapInput{CleanPath: cleanPath, IsIndex: true})
			if err != nil {
				log.Println("Error generating sitemap in getPageDetails: ", err)
				return err
			}
			indexSitemap = sm.Sitemap
			return nil
		})
	}

	eg.Go(func() error {
		sm, err := inst.generateSitemap(generateSitemapInput{CleanPath: cleanPath, IsIndex: false})
		if err != nil {
			log.Println("Error generating sitemap in getPageDetails: ", err)
			return err
		}
		sitemap = sm.Sitemap
		if sm.BackItem != "/" {
			backItem = sm.BackItem
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		log.Println("Error waiting for errgroup in getPageDetails: ", err)
		return nil, err
	}

	p := &DetailedPage{
		Page:         pageBase,
		Sitemap:      sitemap,
		IndexSitemap: indexSitemap,
		BackItem:     backItem,
	}

	inst.pageDetailsCache.Set(cleanPath, p, !found)

	return p, nil
}

func (inst *Instance) GetPlainMarkdown(r *http.Request) (string, error) {
	p, err := inst.GetPageDetails(r)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	if p.Title != "" {
		result.WriteString("# ")
		result.WriteString(p.Title)
		result.WriteString("\n")
	}
	result.WriteString(p.RawContent)

	return result.String(), nil
}

type generateSitemapInput struct {
	CleanPath string
	IsIndex   bool
}

type generateSitemapOutput struct {
	Sitemap  Sitemap
	BackItem string
}

type generateSitemapInnerData struct {
	Pages    []*Page
	BackItem string
	DirToUse string
}

func (inst *Instance) generateSitemap(input generateSitemapInput) (*generateSitemapOutput, error) {
	var innerData *generateSitemapInnerData

	if x, ok := inst.sitemapCache.Load(input); ok && !inst.IsDev {
		innerData = x
	} else {
		dirToUse := filepath.Dir(input.CleanPath)
		if input.IsIndex {
			dirToUse = "/" + input.CleanPath
		}

		directChildren, err := fs.ReadDir(inst.FS, filepath.Join("markdown", dirToUse))
		if err != nil {
			log.Println("Error reading dir in generateSitemap: ", err)
			return nil, err
		}

		pages, hasIndex, err := inst.processDirectChildren(directChildren, dirToUse)
		if err != nil {
			log.Println("Error processing direct children in generateSitemap: ", err)
			return nil, err
		}

		// Sort pages by date
		sort.Slice(pages, func(i, j int) bool {
			// If both have order, use that
			if pages[i].Order != 0 && pages[j].Order != 0 {
				return pages[i].Order < pages[j].Order
			}
			// If only one has order, it comes first
			if pages[i].Order != 0 {
				return true
			}
			if pages[j].Order != 0 {
				return false
			}
			// Otherwise, fall back to date (newest first)
			return pages[i].Date > pages[j].Date
		})

		var backItem string
		if !input.IsIndex && hasIndex && input.CleanPath != "/" {
			backItem = filepath.Dir(input.CleanPath)
		}

		innerData = &generateSitemapInnerData{
			Pages:    pages,
			BackItem: backItem,
			DirToUse: dirToUse,
		}

		inst.sitemapCache.Store(input, innerData)
	}

	sitemap := Sitemap{}
	if innerData.DirToUse == "/" {
		item := SitemapItem{Title: "Home", URL: "/", IsActive: input.CleanPath == "/"}
		sitemap = append(sitemap, item)
	}
	for _, p := range innerData.Pages {
		item := SitemapItem{
			Title:       p.Title,
			URL:         p.URL,
			Description: p.Description,
			Date:        p.Date,
			IsFolder:    p.IsFolder,
			IsActive:    p.URL == input.CleanPath,
		}
		sitemap = append(sitemap, item)
	}

	output := &generateSitemapOutput{
		Sitemap:  sitemap,
		BackItem: innerData.BackItem,
	}

	return output, nil
}

func (inst *Instance) processDirectChildren(directChildren []fs.DirEntry, dirToUse string) ([]*Page, bool, error) {
	type result struct {
		index int
		page  *Page
	}

	hasIndex := false
	results := make([]result, 0, len(directChildren))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(directChildren))

	for i, file := range directChildren {
		wg.Add(1)
		go func(i int, file fs.DirEntry) {
			defer wg.Done()

			name := strings.TrimSuffix(file.Name(), ".md")
			if file.Type().IsRegular() && !strings.HasSuffix(file.Name(), ".md") {
				return
			}
			if name == "_index" {
				mu.Lock()
				hasIndex = true
				mu.Unlock()
				return
			}

			pageBase, found, err := inst.getPageBase(filepath.Join(dirToUse, name))
			if err != nil {
				errChan <- err
				return
			}
			if !found {
				return
			}
			if pageBase.Title == "" {
				pageBase.Title = name
			}

			mu.Lock()
			results = append(results, result{index: i, page: pageBase})
			mu.Unlock()
		}(i, file)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return nil, false, err
		}
	}

	// Sort results to preserve original order
	sort.Slice(results, func(i, j int) bool {
		return results[i].index < results[j].index
	})

	// Extract pages in order
	pages := make([]*Page, 0, len(results))
	for _, r := range results {
		pages = append(pages, r.page)
	}

	return pages, hasIndex, nil
}

var notFoundPage = &Page{
	Title:   "Error",
	Content: "# 404\n\nNothing found.",
}

func (inst *Instance) getPageBase(cleanPath string) (p *Page, found bool, err error) {
	var ok bool
	if p, ok = inst.basePageCache.Get(cleanPath); ok && !inst.IsDev {
		return p, true, nil
	}

	isFolder, fileBytes, err := inst.readPageFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Page not found: ", cleanPath, err)
			return notFoundPage, false, nil
		}
		return nil, false, err
	}

	p, err = inst.parseMarkdown(fileBytes, cleanPath, isFolder)
	if err != nil {
		return nil, false, err
	}

	found = p != notFoundPage
	inst.basePageCache.Set(cleanPath, p, !found)
	return p, found, nil
}

func (inst *Instance) readPageFile(cleanPath string) (bool, []byte, error) {
	fileBytes, err := fs.ReadFile(inst.FS, "markdown"+cleanPath+".md")
	if err == nil {
		return false, fileBytes, nil
	}

	if !os.IsNotExist(err) {
		return false, nil, err
	}

	fileBytes, err = fs.ReadFile(inst.FS, "markdown"+filepath.Join(cleanPath, "_index.md"))
	if err != nil {
		return false, nil, err
	}

	return true, fileBytes, nil
}

func (inst *Instance) parseMarkdown(fileBytes []byte, cleanPath string, isFolder bool) (*Page, error) {
	var p Page
	rest, err := inst.FrontmatterParser(bytes.NewReader(fileBytes), &p)
	if err != nil {
		return nil, err
	}

	p.RawContent = string(rest)
	p.Content = template.HTML(inst.MarkdownParser(rest))
	p.URL = cleanPath
	p.IsFolder = isFolder

	return &p, nil
}
