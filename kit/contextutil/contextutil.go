package contextutil

import (
	"context"
	"net/http"

	"github.com/vormadev/vorma/kit/genericsutil"
)

type Store[T any] struct {
	key *keyWrapper
}

type keyWrapper struct {
	name string
}

func NewStore[T any](key string) *Store[T] {
	return &Store[T]{key: &keyWrapper{name: key}}
}

func (s *Store[T]) ContextWithValue(c context.Context, val T) context.Context {
	return context.WithValue(c, s.key, val)
}

func (s *Store[T]) Value(c context.Context) T {
	return genericsutil.AssertOrZero[T](c.Value(s.key))
}

func (s *Store[T]) RequestWithContextValue(
	r *http.Request,
	val T,
) *http.Request {
	return r.WithContext(s.ContextWithValue(r.Context(), val))
}
