package wave

import (
	"net/http"

	"github.com/vormadev/vorma/kit/middleware"
)

func (w *Wave) GetServeStaticHandler(immutable bool) (http.Handler, error) {
	publicFS, err := w.GetPublicFS()
	if err != nil {
		return nil, err
	}

	fileServer := http.StripPrefix(
		w.cfg.PublicPathPrefix(),
		http.FileServer(http.FS(publicFS)),
	)

	if !immutable {
		return fileServer, nil
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(rw, req)
	}), nil
}

func (w *Wave) MustGetServeStaticHandler(immutable bool) http.Handler {
	h, err := w.GetServeStaticHandler(immutable)
	if err != nil {
		panic(err)
	}
	return h
}

func (w *Wave) ServeStatic(immutable bool) func(http.Handler) http.Handler {
	handler, err := w.GetServeStaticHandler(immutable)
	if err != nil {
		w.log.Error("failed to create static handler", "error", err)
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if w.IsPublicAsset(req.URL.Path) {
				handler.ServeHTTP(rw, req)
				return
			}
			next.ServeHTTP(rw, req)
		})
	}
}

func (w *Wave) FaviconRedirect() middleware.Middleware {
	return middleware.ToHandlerMiddleware(
		"/favicon.ico",
		[]string{http.MethodGet, http.MethodHead},
		func(rw http.ResponseWriter, req *http.Request) {
			url := w.GetPublicURL("favicon.ico")
			fallback := w.cfg.PublicPathPrefix() + "favicon.ico"
			if url == fallback {
				rw.WriteHeader(http.StatusNotFound)
				return
			}
			http.Redirect(rw, req, url, http.StatusFound)
		},
	)
}
