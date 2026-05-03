package vormarun

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/head"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/set"
)

func (instance *Instance) init() error {
	instance.init_once.Do(func() {
		if IsBuild() {
			return
		}

		instance.log = colorlog.New("vorma")

		if IsDev() {
			dist_dir := fsutil.SysNorm(instance.cfg.DistConfig.OutDir)
			sfs_to_use := os.DirFS(
				filepath.ToSlash(filepath.Join(dist_dir, ".vorma", "static")),
			)
			if _, err := fs.Stat(sfs_to_use, "."); err != nil {
				instance.init_err = fmt.Errorf("error accessing static FS at %s: %w", dist_dir, err)
				return
			}
			instance.static_fs = sfs_to_use
		} else {
			instance.static_fs = instance.cfg.DistConfig.StaticFS
		}

		manifest, err := read_manifest(instance.static_fs)
		if err != nil {
			instance.init_err = fmt.Errorf("error reading manifest: %w", err)
			return
		}
		instance.manifest_cache = manifest

		instance.client_build_id_cache, err = manifest.to_client_build_id()
		if err != nil {
			instance.init_err = fmt.Errorf("error hashing manifest into client build id: %w", err)
			return
		}

		root_html_template := instance.cfg.HTMLConfig.Template
		if strings.TrimSpace(root_html_template) == "" {
			root_html_template = default_root_html_template
		}
		tmpl, err := template.New("root").Parse(root_html_template)
		if err != nil {
			instance.init_err = fmt.Errorf("error parsing RootHTMLTemplate: %w", err)
			return
		}
		instance.root_template = tmpl

		instance.head_renderer = head.NewRenderer("vorma")
		if instance.cfg.HTMLConfig.HeadDedupeKeys != nil {
			h := head.NewBuilder()
			instance.cfg.HTMLConfig.HeadDedupeKeys(h)
			instance.head_renderer.InitDedupeRules(h)
		}

		fpf, err := instance.make_final_public_filepaths()
		if err != nil {
			instance.init_err = fmt.Errorf("error making final public filepaths: %w", err)
			return
		}
		instance.final_public_filepaths_cache = fpf
	})

	return instance.init_err
}

func New(config *Config) (*Instance, error) {
	return &Instance{cfg: config}, nil
}

type Router struct {
	instance *Instance
	*mux.Router
	loaders_mux      *mux.NestedRouter
	api_mux          *mux.Router
	api_routes       APIRoutes
	views            Views
	finalize_once    sync.Once
	api_http_methods *set.Set[string]
	api_allow_header string
}

func (router *Router) Instance() *Instance { return router.instance }
func (router *Router) View(view AnyView) {
	router.views = append(router.views, view)
}
func (router *Router) APIRoute(api_route AnyAPIRoute) {
	router.api_routes = append(router.api_routes, api_route)
}

func (instance *Instance) Router() (*Router, error) {
	if err := instance.init(); err != nil {
		return nil, fmt.Errorf("error initializing instance: %w", err)
	}

	instance.router_once.Do(func() {
		router := &Router{
			instance: instance,
			Router: mux.NewRouter(mux.Options{
				DynamicParamPrefix:     Dynamic_Param_Prefix,
				SplatSegmentIdentifier: Splat_Segment_Identifier,
			}),
		}
		instance.router = router

		if IsBuild() {
			return
		}

		if IsDev() {
			router.AddHTTPHandlerFunc(
				"GET",
				"/.vorma/healthz",
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("ok"))
				},
			)
		}

		router.loaders_mux = mux.NewNestedRouter(mux.NestedOptions{
			DynamicParamPrefix:             Dynamic_Param_Prefix,
			SplatSegmentIdentifier:         Splat_Segment_Identifier,
			ExplicitIndexSegmentIdentifier: Explicit_Index_Segment_Identifier,
			ParseInput:                     parse_loader_input,
		})

		router.api_mux = mux.NewRouter(mux.Options{
			MountRoot:              instance.manifest_cache.APIMountRoot,
			DynamicParamPrefix:     Dynamic_Param_Prefix,
			SplatSegmentIdentifier: Splat_Segment_Identifier,
			ParseInput:             parse_api_input,
		})
	})

	return instance.router, nil
}

func (router *Router) Views() []AnyView         { return router.views }
func (router *Router) APIRoutes() []AnyAPIRoute { return router.api_routes }
func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.finalize_once.Do(func() {
		if IsBuild() {
			return
		}
		instance := router.instance

		for _, view := range router.views {
			view.register_to_mux(router.loaders_mux)
		}
		_ = router.AddHTTPHandler("GET", "/*", router.loaders_handler())

		router.api_http_methods = set.New[string]()
		for _, api_route := range router.api_routes {
			router.api_http_methods.Add(api_route.GetMethod())
			api_route.register_to_mux(router.api_mux)
		}
		allow_val_slice := router.api_http_methods.Slice()
		slices.Sort(allow_val_slice)
		router.api_allow_header = strings.Join(allow_val_slice, ", ")
		api_mount_root := instance.manifest_cache.APIMountRoot
		for m := range router.api_http_methods.Range() {
			_ = router.AddHTTPHandler(
				m,
				api_mount_root+"*",
				router.api_handler(),
			)
		}
	})

	router.Router.ServeHTTP(w, r)
}

const default_root_html_template = `<!doctype html>
<html lang="en">
<head>
{{.VormaHead}}
</head>
<body>
{{.VormaBody}}
</body>
</html>
`
