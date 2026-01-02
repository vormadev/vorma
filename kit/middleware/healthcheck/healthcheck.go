package healthcheck

import (
	"net/http"

	"github.com/vormadev/vorma/kit/middleware"
	"github.com/vormadev/vorma/kit/response"
)

// Healthz is a middleware that responds with an HTTP 200 OK status code and the
// string "OK" in the response body for GET and HEAD requests to the "/healthz" endpoint.
var Healthz = OK("/healthz")

// OK returns a middleware that responds with an HTTP 200 OK status code and the
// string "OK" in the response body for GET and HEAD requests to the given endpoint.
func OK(endpoint string) middleware.Middleware {
	methods := []string{http.MethodGet, http.MethodHead}

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)
		res.OKText()
	}

	return middleware.ToHandlerMiddleware(endpoint, methods, handlerFunc)
}
