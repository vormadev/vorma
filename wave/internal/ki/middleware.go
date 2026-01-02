package ki

import (
	"net/http"

	"github.com/vormadev/vorma/kit/middleware"
	"github.com/vormadev/vorma/kit/response"
)

func (c *Config) FaviconRedirect() middleware.Middleware {
	methods := []string{http.MethodGet, http.MethodHead}

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		faviconDotIcoURL := c.GetPublicURL("favicon.ico")
		if faviconDotIcoURL == c.GetPublicPathPrefix()+"favicon.ico" {
			res := response.New(w)
			res.NotFound()
			return
		}
		http.Redirect(w, r, faviconDotIcoURL, http.StatusFound)
	}

	return middleware.ToHandlerMiddleware("/favicon.ico", methods, handlerFunc)
}
