package contextutil

import (
	"context"
	"net/http"

	"github.com/vormadev/vorma/kit/genericsutil"
)

// Store provides a typed key for context and request value access.
type Store[T any] struct {
	key *keyWrapper
}

type keyWrapper struct {
	name string
}

// NewStore creates a typed context store bound to a stable key name.
func NewStore[T any](key string) *Store[T] {
	return &Store[T]{key: &keyWrapper{name: key}}
}

// ContextWithValue returns a derived context carrying val.
func (s *Store[T]) ContextWithValue(c context.Context, val T) context.Context {
	return context.WithValue(c, s.key, val)
}

// Value reads a typed value from context or returns the zero value.
func (s *Store[T]) Value(c context.Context) T {
	return genericsutil.AssertOrZero[T](c.Value(s.key))
}

// RequestWithContextValue returns a request with val attached to its context.
func (s *Store[T]) RequestWithContextValue(
	r *http.Request,
	val T,
) *http.Request {
	return r.WithContext(s.ContextWithValue(r.Context(), val))
}
