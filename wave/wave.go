package wave

import (
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vormadev/vorma/wave/internal/constants"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/middleware"
	"github.com/vormadev/vorma/kit/response"
)

type RuntimeConfig struct {
	PublicPathPrefix      string
	IsUsingCriticalCSS    bool
	IsUsingNonCriticalCSS bool
}

// Do not instantiate directly. Use `wave.New()` instead.
type Wave struct {
	// Must be rooted at waveout/static.
	// If you are using embed.FS from an ancestor directory,
	// use fs.Sub to get a correctly rooted fs.FS.
	static_fs fs.FS
	logger    *slog.Logger
	caches
}

func (w *Wave) ensure_proper_instantiation() {
	if w.static_fs == nil {
		panic("[waveruntime]: Wave must be instantiated via New()")
	}
}

func (w *Wave) StaticRootFS() fs.FS {
	w.ensure_proper_instantiation()
	return w.static_fs
}

type Options struct {
	// Must be rooted at .waveout/static.
	// Ignored in dev, required in prod.
	//
	// If you are using embed.FS from an ancestor directory,
	// use fs.Sub to get a correctly rooted fs.FS.
	//
	// If you are using embed.FS, it's better to leave
	// this nil in dev and only provide it in prod via
	// build tags, to speed up your dev server compilation
	// time.
	DistStaticFS fs.FS
	Logger       *slog.Logger
}

func New(opts Options) *Wave {
	w := &Wave{
		static_fs: opts.DistStaticFS,
		logger:    opts.Logger,
	}
	if !IsDev() && w.static_fs == nil {
		panic("[waveruntime]: DistStaticFS must be provided in production")
	}
	if IsDev() {
		static_dir := envutil.GetStr(
			constants.ENV_KEY_DEV_RUNTIME_STATIC_DIR,
			"",
		)
		if static_dir == "" {
			panic(
				"[waveruntime]: env var " + constants.ENV_KEY_DEV_RUNTIME_STATIC_DIR + " must be set in dev",
			)
		}
		w.static_fs = os.DirFS(static_dir)
	}
	if w.logger == nil {
		w.logger = colorlog.New("wave")
	}
	w.init_caches()
	return w
}

func (w *Wave) Port() int {
	w.ensure_proper_instantiation()
	return Port()
}

func IsDev() bool {
	return envutil.GetBool(constants.ENV_KEY_DEV_RUNTIME_IS_DEV, false)
}

// Returns the PORT env var. Panics if not set or not an int.
func Port() int {
	p := envutil.GetInt("PORT", 0)
	if p <= 0 || p > 65535 {
		panic("[waveruntime]: PORT environment variable is not valid")
	}
	return p
}

func PortStr() string {
	return strconv.Itoa(Port())
}

/////// CACHES

type caches struct {
	public_fs                        *prod_cache[fs.FS]
	private_fs                       *prod_cache[fs.FS]
	runtime_cfg                      *prod_cache[RuntimeConfig]
	public_static_filemap_canonical  *prod_cache[map[string]string]
	public_static_filemap_exposed    *prod_cache[map[string]string]
	private_static_filemap_canonical *prod_cache[map[string]string]
	private_static_filemap_exposed   *prod_cache[map[string]string]
	critical_css                     *prod_cache[string]
	critical_css_style_el_details    *prod_cache[*critical_css_style_el_details]
	inverse_public_static_filemap    *prod_cache[map[string]string]
	public_filemap_data_url_els      *prod_cache[template.HTML]
}

func (w *Wave) init_caches() {
	w.public_fs = cache(w.__UNCACHED__public_fs)
	w.private_fs = cache(w.__UNCACHED__private_fs)
	w.runtime_cfg = cache(w.__UNCACHED__runtime_cfg)
	w.public_static_filemap_canonical = cache(
		w.__UNCACHED__public_static_filemap_canonical,
	)
	w.public_static_filemap_exposed = cache(
		w.__UNCACHED__public_static_filemap_exposed,
	)
	w.private_static_filemap_canonical = cache(
		w.__UNCACHED__private_static_filemap_canonical,
	)
	w.private_static_filemap_exposed = cache(
		w.__UNCACHED__private_static_filemap_exposed,
	)
	w.critical_css = cache(w.__UNCACHED__critical_css)
	w.critical_css_style_el_details = cache(
		w.__UNCACHED__critical_css_style_el_details,
	)
	w.inverse_public_static_filemap = cache(
		w.__UNCACHED__inverse_public_static_filemap,
	)
	w.public_filemap_data_url_els = cache(
		w.__UNCACHED__public_filemap_data_url_els,
	)
}

/////// PUBLIC FS

func (w *Wave) __UNCACHED__public_fs() (fs.FS, error) {
	return fs.Sub(w.static_fs, strip_seg1(constants.STATIC_ASSETS_PUBLIC_DIR))
}

func (w *Wave) PublicFS() (fs.FS, error) {
	w.ensure_proper_instantiation()
	return w.public_fs.get()
}

func (w *Wave) MustPublicFS() fs.FS {
	public_fs, err := w.PublicFS()
	if err != nil {
		panic("[waveruntime]: failed to get public FS: " + err.Error())
	}
	return public_fs
}

/////// PRIVATE FS

func (w *Wave) __UNCACHED__private_fs() (fs.FS, error) {
	return fs.Sub(w.static_fs, strip_seg1(constants.STATIC_ASSETS_PRIVATE_DIR))
}

func (w *Wave) PrivateFS() (fs.FS, error) {
	w.ensure_proper_instantiation()
	return w.private_fs.get()
}

func (w *Wave) MustPrivateFS() fs.FS {
	private_fs, err := w.PrivateFS()
	if err != nil {
		panic("[waveruntime]: failed to get private FS: " + err.Error())
	}
	return private_fs
}

/////// INTERNAL FS

func (w *Wave) __internal_fs() (fs.FS, error) {
	return fs.Sub(w.static_fs, strip_seg1(constants.STATIC_INTERNAL_DIR))
}

/////// RUNTIME CONFIG

func (w *Wave) __UNCACHED__runtime_cfg() (RuntimeConfig, error) {
	internal_fs, err := w.__internal_fs()
	if err != nil {
		return RuntimeConfig{}, err
	}
	runtime_cfg_bytes, err := fs.ReadFile(
		internal_fs,
		constants.RUNTIME_CFG_JSON_FILENAME,
	)
	if err != nil {
		return RuntimeConfig{}, err
	}
	runtime_cfg, err := jsonutil.Parse[RuntimeConfig](runtime_cfg_bytes)
	if err != nil {
		return RuntimeConfig{}, err
	}
	return runtime_cfg, nil
}

func (w *Wave) RuntimeConfig() (RuntimeConfig, error) {
	w.ensure_proper_instantiation()
	return w.runtime_cfg.get()
}

func (w *Wave) MustRuntimeConfig() RuntimeConfig {
	runtime_cfg, err := w.RuntimeConfig()
	if err != nil {
		panic("[waveruntime]: failed to get runtime config: " + err.Error())
	}
	return runtime_cfg
}

/////// PUBLIC PATH PREFIX

func (w *Wave) PublicPathPrefix() (string, error) {
	runtime_cfg, err := w.RuntimeConfig()
	if err != nil {
		return "", err
	}
	return runtime_cfg.PublicPathPrefix, nil
}

func (w *Wave) MustPublicPathPrefix() string {
	public_path_prefix, err := w.PublicPathPrefix()
	if err != nil {
		panic("[waveruntime]: failed to get public path prefix: " + err.Error())
	}
	return public_path_prefix
}

/////// PUBLIC STATIC FILEMAP

func (w *Wave) __UNCACHED__public_static_filemap_canonical() (map[string]string, error) {
	internal_fs, err := w.__internal_fs()
	if err != nil {
		return nil, err
	}
	public_filemap_bytes, err := fs.ReadFile(
		internal_fs,
		constants.PUBLIC_FILEMAP_JSON_FILENAME,
	)
	if err != nil {
		return nil, err
	}
	public_filemap, err := jsonutil.Parse[map[string]string](
		public_filemap_bytes,
	)
	if err != nil {
		return nil, err
	}
	return public_filemap, nil
}

func (w *Wave) __UNCACHED__public_static_filemap_exposed() (map[string]string, error) {
	public_filemap, err := w.public_static_filemap_canonical.get()
	if err != nil {
		return nil, err
	}
	return maps.Clone(public_filemap), nil
}

func (w *Wave) PublicStaticFilemap() (map[string]string, error) {
	w.ensure_proper_instantiation()
	return w.public_static_filemap_exposed.get()
}

func (w *Wave) MustPublicStaticFilemap() map[string]string {
	public_static_filemap, err := w.PublicStaticFilemap()
	if err != nil {
		panic(
			"[waveruntime]: failed to get public static filemap: " + err.Error(),
		)
	}
	return public_static_filemap
}

/////// PRIVATE STATIC FILEMAP

func (w *Wave) __UNCACHED__private_static_filemap_canonical() (map[string]string, error) {
	internal_fs, err := w.__internal_fs()
	if err != nil {
		return nil, err
	}
	private_filemap_bytes, err := fs.ReadFile(
		internal_fs,
		constants.PRIVATE_FILEMAP_JSON_FILENAME,
	)
	if err != nil {
		return nil, err
	}
	private_filemap, err := jsonutil.Parse[map[string]string](
		private_filemap_bytes,
	)
	if err != nil {
		return nil, err
	}
	return private_filemap, nil
}

func (w *Wave) __UNCACHED__private_static_filemap_exposed() (map[string]string, error) {
	private_filemap, err := w.private_static_filemap_canonical.get()
	if err != nil {
		return nil, err
	}
	return maps.Clone(private_filemap), nil
}

func (w *Wave) PrivateStaticFilemap() (map[string]string, error) {
	w.ensure_proper_instantiation()
	return w.private_static_filemap_exposed.get()
}

func (w *Wave) MustPrivateStaticFilemap() map[string]string {
	private_static_filemap, err := w.PrivateStaticFilemap()
	if err != nil {
		panic(
			"[waveruntime]: failed to get private static filemap: " + err.Error(),
		)
	}
	return private_static_filemap
}

/////// PUBLIC URL

func (w *Wave) PublicURL(src_path string) (string, error) {
	w.ensure_proper_instantiation()
	runtime_cfg, err := w.RuntimeConfig()
	if err != nil {
		return "", err
	}
	fm, err := w.public_static_filemap_canonical.get()
	if err != nil {
		return "", err
	}

	src_path = strings.TrimPrefix(strings.TrimSpace(src_path), "/")
	hashed, ok := fm[src_path]
	if !ok {
		return src_path, errors.New(
			"file not found in public static filemap: " + src_path,
		)
	}
	return path.Join(runtime_cfg.PublicPathPrefix, hashed), nil
}

func (w *Wave) MustPublicURL(src_path string) string {
	public_url, err := w.PublicURL(src_path)
	if err != nil {
		panic(
			"[waveruntime]: failed to get public URL for " + src_path + ": " + err.Error(),
		)
	}
	return public_url
}

/////// CRITICAL CSS

func (w *Wave) __UNCACHED__critical_css() (string, error) {
	internal_fs, err := w.__internal_fs()
	if err != nil {
		return "", err
	}
	critical_css_bytes, err := fs.ReadFile(
		internal_fs,
		constants.CRITICAL_CSS_FILENAME,
	)
	if err != nil {
		return "", err
	}
	return string(critical_css_bytes), nil
}

func (w *Wave) CriticalCSS() (string, error) {
	w.ensure_proper_instantiation()
	return w.critical_css.get()
}

func (w *Wave) MustCriticalCSS() string {
	critical_css, err := w.CriticalCSS()
	if err != nil {
		panic("[waveruntime]: failed to get critical CSS: " + err.Error())
	}
	return critical_css
}

type critical_css_style_el_details struct {
	style_el template.HTML
	csp_hash string
}

func (w *Wave) __UNCACHED__critical_css_style_el_details() (*critical_css_style_el_details, error) {
	critical_css, err := w.CriticalCSS()
	if err != nil {
		return nil, err
	}
	el := htmlutil.Element{
		Tag: "style",
		AttributesKnownSafe: map[string]string{
			"id": constants.CRITICAL_CSS_EL_ID,
		},
		DangerousInnerHTML: critical_css,
	}
	csp_hash, err := htmlutil.ComputeContentSha256(&el)
	if err != nil {
		return nil, err
	}
	style_el, err := htmlutil.RenderElement(&el)
	if err != nil {
		return nil, err
	}
	return &critical_css_style_el_details{
		style_el: style_el,
		csp_hash: csp_hash,
	}, nil
}

func (w *Wave) CriticalCSSStyleEl() (template.HTML, error) {
	w.ensure_proper_instantiation()
	details, err := w.critical_css_style_el_details.get()
	if err != nil {
		return "", err
	}
	return details.style_el, nil
}

func (w *Wave) MustCriticalCSSStyleEl() template.HTML {
	style_el, err := w.CriticalCSSStyleEl()
	if err != nil {
		panic(
			"[waveruntime]: failed to get critical CSS style element: " + err.Error(),
		)
	}
	return style_el
}

func (w *Wave) CriticalCSSStyleElCSPHash() (string, error) {
	w.ensure_proper_instantiation()
	details, err := w.critical_css_style_el_details.get()
	if err != nil {
		return "", err
	}
	return details.csp_hash, nil
}

func (w *Wave) MustCriticalCSSStyleElCSPHash() string {
	csp_hash, err := w.CriticalCSSStyleElCSPHash()
	if err != nil {
		panic(
			"[waveruntime]: failed to get critical CSS style element CSP hash: " + err.Error(),
		)
	}
	return csp_hash
}

/////// NON-CRITICAL CSS

func (w *Wave) NonCriticalCSSStyleSheetURL() (string, error) {
	return w.PublicURL(constants.NON_CRITICAL_CSS_FILENAME)
}

func (w *Wave) MustNonCriticalCSSStyleSheetURL() string {
	url, err := w.NonCriticalCSSStyleSheetURL()
	if err != nil {
		panic(
			"[waveruntime]: failed to get non-critical CSS stylesheet URL: " + err.Error(),
		)
	}
	return url
}

func (w *Wave) NonCriticalCSSLinkEl() (template.HTML, error) {
	url, err := w.NonCriticalCSSStyleSheetURL()
	if err != nil {
		return "", err
	}
	el := htmlutil.Element{
		Tag: "link",
		AttributesKnownSafe: map[string]string{
			"rel":  "stylesheet",
			"id":   constants.NON_CRITICAL_CSS_EL_ID,
			"href": url,
		},
	}
	return htmlutil.RenderElement(&el)
}

func (w *Wave) MustNonCriticalCSSLinkEl() template.HTML {
	link_el, err := w.NonCriticalCSSLinkEl()
	if err != nil {
		panic(
			"[waveruntime]: failed to get non-critical CSS link element: " + err.Error(),
		)
	}
	return link_el
}

/////// CRITICAL CSS + NON-CRITICAL CSS HELPER

func (w *Wave) CSSEls() (template.HTML, error) {
	var parts []string
	runtime_cfg, err := w.RuntimeConfig()
	if err != nil {
		return "", err
	}
	if runtime_cfg.IsUsingCriticalCSS {
		el, err := w.CriticalCSSStyleEl()
		if err != nil {
			return "", err
		}
		parts = append(parts, string(el))
	}
	if runtime_cfg.IsUsingNonCriticalCSS {
		el, err := w.NonCriticalCSSLinkEl()
		if err != nil {
			return "", err
		}
		parts = append(parts, string(el))
	}
	return template.HTML(strings.Join(parts, "\n")), nil
}

func (w *Wave) MustCSSEls() template.HTML {
	els, err := w.CSSEls()
	if err != nil {
		panic(
			"[waveruntime]: failed to get CSS elements: " + err.Error(),
		)
	}
	return els
}

/////// PUBLIC FILEMAP CLIENT INJECTION

func (w *Wave) __UNCACHED__public_filemap_data_url_els() (template.HTML, error) {
	fm, err := w.PublicStaticFilemap()
	if err != nil {
		// No public filemap available (e.g., public static not configured)
		return "", nil
	}
	if _, ok := fm[constants.PUBLIC_FILEMAP_FILENAME]; !ok {
		return "", nil
	}

	url, err := w.PublicURL(constants.PUBLIC_FILEMAP_FILENAME)
	if err != nil {
		return "", err
	}

	preload_el := htmlutil.Element{
		Tag: "link",
		AttributesKnownSafe: map[string]string{
			"rel":         "preload",
			"href":        url,
			"as":          "fetch",
			"crossorigin": "anonymous",
		},
	}
	meta_el := htmlutil.Element{
		Tag: "meta",
		AttributesKnownSafe: map[string]string{
			"id":       constants.PUBLIC_FILEMAP_META_EL_ID,
			"data-url": url,
		},
	}

	preload_html, err := htmlutil.RenderElement(&preload_el)
	if err != nil {
		return "", err
	}
	meta_html, err := htmlutil.RenderElement(&meta_el)
	if err != nil {
		return "", err
	}

	return preload_html + "\n" + meta_html, nil
}

// On client, do something like this:
//
//	const filemapURL = document.getElementById("wave-public-filemap-url").dataset.url;
//	const filemap = await (await fetch(filemapURL)).json();
//	function getPublicURL(srcPath) {
//		srcPath = srcPath.trim();
//		if (srcPath.startsWith("/")) {
//			srcPath = srcPath.slice(1);
//		}
//		return filemap[srcPath] || srcPath;
//	}
func (w *Wave) PublicFilemapDataURLEls() (template.HTML, error) {
	w.ensure_proper_instantiation()
	return w.public_filemap_data_url_els.get()
}

// On client, do something like this:
//
//	const filemapURL = document.getElementById("wave-public-filemap-url").dataset.url;
//	const filemap = await (await fetch(filemapURL)).json();
//	function getPublicURL(srcPath) {
//		srcPath = srcPath.trim();
//		if (srcPath.startsWith("/")) {
//			srcPath = srcPath.slice(1);
//		}
//		return filemap[srcPath] || srcPath;
//	}
func (w *Wave) MustPublicFilemapDataURLEls() template.HTML {
	els, err := w.PublicFilemapDataURLEls()
	if err != nil {
		panic(
			"[waveruntime]: failed to get filemap URL elements: " + err.Error(),
		)
	}
	return els
}

/////// STATIC ASSET SERVING

func (w *Wave) StaticFileServerHandler(
	add_immutable_cache_headers bool,
) (http.Handler, error) {
	public_fs, err := w.PublicFS()
	if err != nil {
		return nil, err
	}
	public_path_prefix, err := w.PublicPathPrefix()
	if err != nil {
		return nil, err
	}
	if add_immutable_cache_headers {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().
				Set("Cache-Control", "public, max-age=31536000, immutable")
			http.StripPrefix(public_path_prefix, http.FileServer(http.FS(public_fs))).
				ServeHTTP(w, r)
		}), nil
	}
	return http.StripPrefix(
		public_path_prefix,
		http.FileServer(http.FS(public_fs)),
	), nil
}

func (w *Wave) MustStaticFileServerHandler(
	add_immutable_cache_headers bool,
) http.Handler {
	handler, err := w.StaticFileServerHandler(add_immutable_cache_headers)
	if err != nil {
		panic(
			"[waveruntime]: failed to create static file server handler: " + err.Error(),
		)
	}
	return handler
}

func (w *Wave) StaticFileServerMiddleware(
	add_immutable_cache_headers bool,
) (func(http.Handler) http.Handler, error) {
	handler, err := w.StaticFileServerHandler(add_immutable_cache_headers)
	if err != nil {
		return nil, err
	}
	fn := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(_w http.ResponseWriter, r *http.Request) {
			ss, err := w.should_serve_as_public_asset(r.URL.Path)
			if err == nil && ss {
				handler.ServeHTTP(_w, r)
				return
			}
			next.ServeHTTP(_w, r)
		})
	}
	return fn, nil
}

func (w *Wave) MustStaticFileServerMiddleware(
	add_immutable_cache_headers bool,
) func(http.Handler) http.Handler {
	middleware, err := w.StaticFileServerMiddleware(add_immutable_cache_headers)
	if err != nil {
		panic(
			"[waveruntime]: failed to create static file server middleware: " + err.Error(),
		)
	}
	return middleware
}

func (w *Wave) should_serve_as_public_asset(_path string) (bool, error) {
	public_path_prefix, err := w.PublicPathPrefix()
	if err != nil {
		return false, err
	}
	if public_path_prefix == "" || public_path_prefix == "/" {
		return w.get_is_public_asset(_path)
	}
	return strings.HasPrefix(_path, public_path_prefix), nil
}

func (w *Wave) get_is_public_asset(_path string) (bool, error) {
	inverse, err := w.inverse_public_static_filemap.get()
	if err != nil {
		return false, err
	}
	_, is_public_asset := inverse[strings.TrimPrefix(_path, "/")]
	return is_public_asset, nil
}

func (w *Wave) __UNCACHED__inverse_public_static_filemap() (map[string]string, error) {
	fm, err := w.public_static_filemap_canonical.get()
	if err != nil {
		return nil, err
	}
	inverse := make(map[string]string, len(fm))
	for k, v := range fm {
		inverse[v] = k
	}
	return inverse, nil
}

/////// FAVICON REDIRECT

// FaviconRedirect returns middleware that redirects requests for
// /favicon.ico to the hashed public asset URL. Returns 404 if the
// favicon is not found in the public filemap.
func (w *Wave) FaviconRedirect() func(http.Handler) http.Handler {
	w.ensure_proper_instantiation()
	return middleware.ToHandlerMiddleware(
		"/favicon.ico",
		[]string{http.MethodGet, http.MethodHead},
		func(_w http.ResponseWriter, r *http.Request) {
			url, err := w.PublicURL("favicon.ico")
			if err != nil {
				res := response.New(_w)
				res.NotFound()
				return
			}
			http.Redirect(_w, r, url, http.StatusFound)
		},
	)
}

/////// DEV REFRESH

func (w *Wave) DevRefreshScriptEl() template.HTML {
	return GetDevRefreshScriptEl()
}

func (w *Wave) DevRefreshScriptElCSPHash() string {
	return GetDevRefreshScriptElCSPHash()
}

// GetDevRefreshScriptEl returns a <script> tag containing the browser sync
// client. Returns empty string in production.
func GetDevRefreshScriptEl() template.HTML {
	if !IsDev() {
		return ""
	}
	return template.HTML(
		fmt.Sprintf("<script>%s</script>", refresh_script_inner_html()),
	)
}

// GetDevRefreshScriptElCSPHash returns the base64-encoded SHA-256 hash of
// the refresh script content, suitable for Content-Security-Policy
// script-src directives. Returns empty string in production.
func GetDevRefreshScriptElCSPHash() string {
	if !IsDev() {
		return ""
	}
	return bytesutil.ToBase64(
		cryptoutil.Sha256Hash([]byte(refresh_script_inner_html())),
	)
}

//go:embed refresh_script.js
var refresh_script_tmpl string

func refresh_script_inner_html() string {
	if !IsDev() {
		return ""
	}
	p := envutil.GetStr(constants.ENV_KEY_DEV_RUNTIME_REFRESH_PORT, "")
	if p == "" {
		panic(fmt.Sprintf(
			"dev refresh script: environment variable %s is not set",
			constants.ENV_KEY_DEV_RUNTIME_REFRESH_PORT,
		))
	}
	t := envutil.GetStr(constants.ENV_KEY_DEV_RUNTIME_REFRESH_TOKEN, "")
	if t == "" {
		panic(fmt.Sprintf(
			"dev refresh script: environment variable %s is not set",
			constants.ENV_KEY_DEV_RUNTIME_REFRESH_TOKEN,
		))
	}
	s := strings.ReplaceAll(
		refresh_script_tmpl,
		"__REPLACE_ME_WITH_REFRESH_PORT__",
		p,
	)
	return strings.ReplaceAll(
		s,
		"__REPLACE_ME_WITH_REFRESH_TOKEN__",
		t,
	)
}

/////////////////////////////////////////////////////////////////////
/////// Internal utils
/////////////////////////////////////////////////////////////////////

// NOTE: We intentionally do not cache errors in case they are transient,
// which can theoretically happen if you are using the physical file system
// in prod (e.g., `os.DirFS`) rather than an `embed.FS`.

type prod_cache[T any] struct {
	val atomic.Pointer[cache_res[T]]
	mu  sync.Mutex
	fn  func() (T, error)
}

type cache_res[T any] struct {
	val T
	err error
}

func cache[T any](fn func() (T, error)) *prod_cache[T] {
	return &prod_cache[T]{fn: fn}
}

func (pc *prod_cache[T]) get() (T, error) {
	if IsDev() {
		return pc.fn()
	}
	if r := pc.val.Load(); r != nil {
		return r.val, r.err
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if r := pc.val.Load(); r != nil {
		return r.val, r.err
	}
	val, err := pc.fn()
	if err == nil {
		pc.val.Store(&cache_res[T]{val: val})
	}
	return val, err
}

func strip_seg1(p string) string {
	split := matcher.ParseSegments(p)
	if len(split) <= 1 {
		return ""
	}
	return path.Join(split[1:]...)
}
