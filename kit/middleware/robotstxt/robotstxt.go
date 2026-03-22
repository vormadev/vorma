package robotstxt

import (
	"net/http"

	"github.com/vormadev/vorma/kit/middleware"
	"github.com/vormadev/vorma/kit/response"
)

// Allow is a middleware that responds with a barebones robots.txt file that
// allows all user agents to access any path.
func Allow(next http.Handler) http.Handler {
	return Content("User-agent: *\nAllow: /")(next)
}

// Disallow is a middleware that responds with a barebones robots.txt file that
// disallows all user agents from accessing any path.
func Disallow(next http.Handler) http.Handler {
	return Content("User-agent: *\nDisallow: /")(next)
}

// Content returns a middleware that responds with a robots.txt file containing the
// given content.
func Content(content string) middleware.Middleware {
	endpoint := "/robots.txt"

	methods := []string{http.MethodGet, http.MethodHead}

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)
		res.Text(content)
	}

	return middleware.ToHandlerMiddleware(endpoint, methods, handlerFunc)
}
