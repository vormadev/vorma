package routeparse

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

func FuzzExtractRouteCalls_IsDeterministicAndSafe(f *testing.F) {
	f.Add(
		`import { route } from "vorma/buildtime"; route("/", "./root.tsx", "default");`,
	)
	f.Add(
		`import { route as defineRoute } from "vorma/buildtime"; const p = "./x.tsx"; defineRoute("/x", p);`,
	)
	f.Add(`import { route } from "vorma/client"; route("/", "./ignored.tsx");`)
	f.Add(
		`import { route } from "vorma/buildtime"; route("/a", import("./a.tsx"), "A");`,
	)
	f.Add(`import { route } from "vorma/buildtime"; route("/bad", getPath());`)

	f.Fuzz(func(t *testing.T, source string) {
		routesA, unresolvedA, errA := extractRouteCalls(source)
		routesB, unresolvedB, errB := extractRouteCalls(source)

		if (errA != nil) != (errB != nil) {
			t.Fatalf("error mismatch: errA=%v errB=%v", errA, errB)
		}
		if errA != nil && errB != nil && errA.Error() != errB.Error() {
			t.Fatalf(
				"error text mismatch: %q vs %q",
				errA.Error(),
				errB.Error(),
			)
		}
		if !reflect.DeepEqual(routesA, routesB) {
			t.Fatalf("routes mismatch: %#v vs %#v", routesA, routesB)
		}
		if !reflect.DeepEqual(unresolvedA, unresolvedB) {
			t.Fatalf(
				"unresolved mismatch: %#v vs %#v",
				unresolvedA,
				unresolvedB,
			)
		}

		for _, unresolved := range unresolvedA {
			if strings.TrimSpace(unresolved.Reason) == "" {
				t.Fatalf(
					"unresolved route reason should not be empty: %#v",
					unresolved,
				)
			}
		}
	})
}

func FuzzParseClientRoutes_IsSafeForRouteFileInputs(f *testing.F) {
	f.Add(
		`import { route } from "vorma/buildtime"; route("/", "./components/root.tsx", "default");`,
	)
	f.Add(
		`import { route } from "vorma/buildtime"; const m = "./components/root.tsx"; route("/a", m);`,
	)
	f.Add(
		`import { route } from "vorma/buildtime"; route("/b", import("./components/root.tsx"));`,
	)
	f.Add(`import { route } from "vorma/buildtime"; route("/c", getPath());`)
	f.Add(`import { route } from "vorma/buildtime";`)

	f.Fuzz(func(t *testing.T, routeSource string) {
		rootDir := t.TempDir()
		t.Chdir(rootDir)

		mustWriteFile(
			t,
			"frontend/src/components/root.tsx",
			[]byte("export default function Root() {}"),
		)
		mustWriteFile(t, "frontend/src/vorma.routes.ts", []byte(routeSource))

		v := &vormaruntime.Vorma{
			Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
				ClientRouteDefinitionPatterns: []string{
					"frontend/src/**/*vorma.routes.ts",
				},
			}),
			Log: testLogger(),
		}

		paths, err := ParseClientRoutes(v)
		if err != nil {
			return
		}
		for pattern, p := range paths {
			if p == nil {
				t.Fatalf("path entry for %q is nil", pattern)
			}
			if p.OriginalPattern != pattern {
				t.Fatalf(
					"original pattern mismatch: map key=%q value=%q",
					pattern,
					p.OriginalPattern,
				)
			}
			if strings.TrimSpace(p.SrcPath) == "" {
				t.Fatalf("src path should not be empty for pattern %q", pattern)
			}
		}
	})
}
