package vormarun

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/vormadev/vorma/kit/middleware"
	"github.com/vormadev/vorma/kit/response"
)

func (router *Router) MustUsePublicFileServerMiddleware() {
	if err := router.UsePublicFileServerMiddleware(); err != nil {
		panic(fmt.Sprintf("Failed to add public file server middleware: %v", err))
	}
}

func (router *Router) UsePublicFileServerMiddleware() error {
	if IsBuild() {
		return nil
	}
	inst := router.instance
	handler, err := inst.public_file_server_handler()
	if err != nil {
		return fmt.Errorf("error creating static file server handler: %w", err)
	}
	router.UseMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, err := inst.is_public_asset(r.URL.Path)
			if err == nil && ok {
				handler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	return nil
}

func (inst *Instance) public_file_server_handler() (http.Handler, error) {
	public_fs, err := inst.PublicFS()
	if err != nil {
		return nil, fmt.Errorf("error getting public FS: %w", err)
	}
	public_static_base_path, err := inst.public_static_base_path()
	if err != nil {
		return nil, fmt.Errorf("error getting public static base path: %w", err)
	}
	file_server := http.FileServer(http.FS(public_fs))
	file_server = http.StripPrefix(public_static_base_path, file_server)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		file_server.ServeHTTP(w, r)
	}), nil
}

// FaviconRedirect returns middleware that redirects requests for
// /favicon.ico to the hashed public asset URL. Returns 404 if the
// favicon is not found in the public filemap.
func (inst *Instance) FaviconRedirect() func(http.Handler) http.Handler {
	if err := inst.init(); IsBuild() || err != nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return middleware.ToHandlerMiddleware(
		"/favicon.ico",
		[]string{http.MethodGet, http.MethodHead},
		func(w http.ResponseWriter, r *http.Request) {
			url, err := inst.PublicURL("favicon.ico")
			if err != nil {
				res := response.New(w)
				res.NotFound()
				return
			}
			http.Redirect(w, r, url, http.StatusFound)
		},
	)
}

func (inst *Instance) is_public_asset(path string) (bool, error) {
	public_static_base_path, err := inst.public_static_base_path()
	if err != nil {
		return false, fmt.Errorf("error getting public static base path: %w", err)
	}
	if public_static_base_path == "" || public_static_base_path == "/" {
		ipf, err := inst.final_public_filepaths()
		if err != nil {
			return false, fmt.Errorf("error getting inverse public filemap: %w", err)
		}
		return ipf.Has(path), nil
	}
	return strings.HasPrefix(path, public_static_base_path), nil
}
