package middleware

import (
	"net/http"
	"slices"
)

type Middleware func(http.Handler) http.Handler

func ToHandlerMiddleware(endpoint string, methods []string, handlerFunc http.HandlerFunc) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if slices.Contains(methods, r.Method) && r.URL.Path == endpoint {
				handlerFunc(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
