package app

import (
	"github.com/vormadev/vorma"
)

type (
	View[I, O any]     = vorma.View[I, O, *RequestCtx[I], RequestCtx[I]]
	APIRoute[I, O any] = vorma.APIRoute[I, O, *RequestCtx[I], RequestCtx[I]]
)

type RequestCtx[I any] struct {
	*vorma.RequestCtx[I]
	// Add whatever else you want in the request context here
}

// Wrap satisfies the vorma.RequestCtxWrapper interface.
func (RequestCtx[I]) Wrap(c *vorma.RequestCtx[I]) *RequestCtx[I] {
	return &RequestCtx[I]{
		RequestCtx: c,
		// Add whatever else you want in the request context here
	}
}
