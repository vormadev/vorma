package contextutil

import (
	"context"
	"net/http"
	"testing"
)

func genericTest[T comparable](t *testing.T, val T) {
	store := NewStore[T]("key")

	ctx := store.ContextWithValue(context.Background(), val)

	if store.Value(ctx) != val {
		t.Error("expected value in context, got", store.Value(ctx))
	}

	r := store.RequestWithContextValue(&http.Request{}, val)

	if store.Value(r.Context()) != val {
		t.Error(
			"expected value in request context, got",
			store.Value(r.Context()),
		)
	}
}

func Test(t *testing.T) {
	genericTest(t, "hello")
	genericTest(t, "world")
	genericTest(t, 42)
	genericTest(t, 3.14)
	genericTest(t, true)
	genericTest(t, false)
	genericTest(t, struct{}{})
	genericTest(t, struct{ Name string }{Name: "Bob"})
}

func TestStoreKeysDoNotCollideAcrossInstances(t *testing.T) {
	storeA := NewStore[string]("shared-key")
	storeB := NewStore[string]("shared-key")

	ctx := storeA.ContextWithValue(context.Background(), "a")
	ctx = storeB.ContextWithValue(ctx, "b")

	if got := storeA.Value(ctx); got != "a" {
		t.Fatalf("storeA value mismatch: got %q", got)
	}
	if got := storeB.Value(ctx); got != "b" {
		t.Fatalf("storeB value mismatch: got %q", got)
	}
}
