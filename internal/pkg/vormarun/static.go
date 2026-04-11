package vormarun

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/vormadev/vorma/kit/middleware"
	"github.com/vormadev/vorma/kit/response"
)

func (v *Vorma) PublicFileServerHandler() (http.Handler, error) {
	public_fs, err := v.PublicFS()
	if err != nil {
		return nil, fmt.Errorf("error getting public FS: %w", err)
	}
	public_static_base_path, err := v.public_static_base_path()
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

func (v *Vorma) PublicFileServerMiddleware() (func(http.Handler) http.Handler, error) {
	handler, err := v.PublicFileServerHandler()
	if err != nil {
		return nil, fmt.Errorf("error creating static file server handler: %w", err)
	}
	fn := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(_w http.ResponseWriter, r *http.Request) {
			ok, err := v.is_public_asset(r.URL.Path)
			if err == nil && ok {
				handler.ServeHTTP(_w, r)
				return
			}
			next.ServeHTTP(_w, r)
		})
	}
	return fn, nil
}

// FaviconRedirect returns middleware that redirects requests for
// /favicon.ico to the hashed public asset URL. Returns 404 if the
// favicon is not found in the public filemap.
func (v *Vorma) FaviconRedirect() func(http.Handler) http.Handler {
	return middleware.ToHandlerMiddleware(
		"/favicon.ico",
		[]string{http.MethodGet, http.MethodHead},
		func(w http.ResponseWriter, r *http.Request) {
			url, err := v.PublicURL("favicon.ico")
			if err != nil {
				res := response.New(w)
				res.NotFound()
				return
			}
			http.Redirect(w, r, url, http.StatusFound)
		},
	)
}

func (v *Vorma) is_public_asset(path string) (bool, error) {
	public_static_base_path, err := v.public_static_base_path()
	if err != nil {
		return false, fmt.Errorf("error getting public static base path: %w", err)
	}
	if public_static_base_path == "" || public_static_base_path == "/" {
		ipf, err := v.final_public_filepaths()
		if err != nil {
			return false, fmt.Errorf("error getting inverse public filemap: %w", err)
		}
		return ipf.Has(path), nil
	}
	return strings.HasPrefix(path, public_static_base_path), nil
}
