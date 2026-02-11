package vormabuild

import "testing"

func TestExtractRouteCalls_RecognizesBuildtimeImportOnly(t *testing.T) {
	t.Run("accepts route imported from vorma/buildtime", func(t *testing.T) {
		code := `
			import { route as defineRoute } from "vorma/buildtime";
			defineRoute("/ok", "./ok.tsx", "default");
		`

		routes, unresolved, err := extractRouteCalls(code)
		if err != nil {
			t.Fatalf("extractRouteCalls returned error: %v", err)
		}
		if len(unresolved) != 0 {
			t.Fatalf("expected no unresolved routes, got %d", len(unresolved))
		}
		if len(routes) != 1 {
			t.Fatalf("expected 1 route, got %d", len(routes))
		}

		route := routes[0]
		if route.Pattern != "/ok" {
			t.Fatalf("expected pattern /ok, got %q", route.Pattern)
		}
		if route.Module != "./ok.tsx" {
			t.Fatalf("expected module ./ok.tsx, got %q", route.Module)
		}
		if route.Key != "default" {
			t.Fatalf("expected key default, got %q", route.Key)
		}
	})

	t.Run("does not register route imported from vorma/client", func(t *testing.T) {
		code := `
			import { route } from "vorma/client";
			route("/legacy", "./legacy.tsx", "default");
		`

		routes, unresolved, err := extractRouteCalls(code)
		if err != nil {
			t.Fatalf("extractRouteCalls returned error: %v", err)
		}
		if len(unresolved) != 0 {
			t.Fatalf("expected no unresolved routes, got %d", len(unresolved))
		}
		if len(routes) != 0 {
			t.Fatalf("expected 0 routes for legacy import path, got %d", len(routes))
		}
	})
}
