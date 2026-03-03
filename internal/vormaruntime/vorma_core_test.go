package vormaruntime

import (
	"encoding/json"
	"github.com/vormadev/vorma/internal/testhelpers/waveoutputtest"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/lab/tsgen"
)

func TestGetterSnapshotsAreDefensiveCopies(t *testing.T) {
	stage := defaultPathsFile("build-copy", map[string]*Path{
		"/": {
			OriginalPattern: "/",
			SrcPath:         "frontend/src/routes/root.tsx",
			OutPath:         waveoutputtest.TestWaveOutputPath("root.js"),
			ExportKey:       "default",
			Deps:            []string{waveoutputtest.TestWaveOutputPath("root-dep.js")},
		},
	})
	stage.ClientEntryDeps = []string{
		waveoutputtest.TestWaveOutputPath("client-a.js"),
		waveoutputtest.TestWaveOutputPath("client-b.js"),
	}
	stage.DepToCSSBundleMap = map[string][]string{
		waveoutputtest.TestWaveOutputPath("client-entry.js"): {waveoutputtest.TestWaveOutputPath("client.css")},
		waveoutputtest.TestWaveOutputPath("client-a.js"):     {waveoutputtest.TestWaveOutputPath("client-a.css")},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	pathsA := app.Paths()
	pathsA["/new"] = &Path{OriginalPattern: "/new"}
	pathsA["/"].Deps[0] = "mutated-dep.js"

	pathsB := app.Paths()
	if _, ok := pathsB["/new"]; ok {
		t.Fatal("Paths leaked caller mutation into runtime state")
	}
	if got, want := pathsB["/"].Deps[0], waveoutputtest.TestWaveOutputPath("root-dep.js"); got != want {
		t.Fatalf(
			"path deps mutated through snapshot: got %q want %q",
			got,
			want,
		)
	}

	entryDepsA := app.ClientEntryDeps()
	entryDepsA[0] = "mutated-client-dep.js"
	entryDepsB := app.ClientEntryDeps()
	if got, want := entryDepsB[0], waveoutputtest.TestWaveOutputPath("client-a.js"); got != want {
		t.Fatalf(
			"client entry deps mutated through getter: got %q want %q",
			got,
			want,
		)
	}

	cssMapA := app.DepToCSSBundleMap()
	cssMapA[waveoutputtest.TestWaveOutputPath("client-entry.js")][0] = "mutated.css"
	cssMapA[waveoutputtest.TestWaveOutputPath("new.js")] = []string{"new.css"}

	cssMapB := app.DepToCSSBundleMap()
	if got, want := cssMapB[waveoutputtest.TestWaveOutputPath("client-entry.js")][0], waveoutputtest.TestWaveOutputPath("client.css"); got != want {
		t.Fatalf(
			"dep->css map mutated through getter: got %q want %q",
			got,
			want,
		)
	}
	if _, ok := cssMapB[waveoutputtest.TestWaveOutputPath("new.js")]; ok {
		t.Fatal("dep->css map leaked caller-added key into runtime state")
	}
}

func TestGettersPreserveNilShape(t *testing.T) {
	app := &Vorma{}

	if got := app.Paths(); got != nil {
		t.Fatalf(
			"Paths with nil internal paths should return nil, got %#v",
			got,
		)
	}
	if got := app.ClientEntryDeps(); got != nil {
		t.Fatalf(
			"GetClientEntryDeps with nil internal deps should return nil, got %#v",
			got,
		)
	}
	if got := app.DepToCSSBundleMap(); got != nil {
		t.Fatalf(
			"GetDepToCSSBundleMap with nil internal map should return nil, got %#v",
			got,
		)
	}

	app.WithLock(func(lv *LockedVorma) {
		lv.SetPaths(map[string]*Path{"/": {OriginalPattern: "/"}})
	})
	paths := app.Paths()
	if !reflect.DeepEqual(
		paths,
		map[string]*Path{"/": {OriginalPattern: "/"}},
	) {
		t.Fatalf("unexpected paths snapshot after set: %#v", paths)
	}
}

func TestLockedVormaGettersAndSetters(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app
	app.SetIsDev(true)

	initialBuildID := app.BuildID()
	initialRouteManifestFile := app.RouteManifestFile()
	initialRootTemplate := app.RootTemplate()
	if initialRootTemplate == nil {
		t.Fatal("expected initial root template to be non-nil")
	}

	app.WithLock(func(lv *LockedVorma) {
		lv.SetPaths(map[string]*Path{
			"/locked": {
				OriginalPattern: "/locked",
				SrcPath:         "frontend/src/routes/locked.tsx",
				OutPath:         waveoutputtest.TestWaveOutputPath("routes/locked.js"),
				ExportKey:       "default",
			},
		})
	})

	if got := app.BuildID(); got != initialBuildID {
		t.Fatalf("BuildID() = %q, want %q", got, initialBuildID)
	}
	if got := app.RouteManifestFile(); got != initialRouteManifestFile {
		t.Fatalf(
			"RouteManifestFile() = %q, want %q",
			got,
			initialRouteManifestFile,
		)
	}
	if got := app.RootTemplate(); got == nil {
		t.Fatal("RootTemplate() returned nil")
	}
	if got := app.Paths(); got["/locked"] == nil {
		t.Fatal("Paths() missing /locked after SetPaths")
	}

	app.WithRLock(func(lv *ReadLockedVorma) {
		if got := lv.Vorma(); got != app {
			t.Fatal(
				"LockedVorma.Vorma() did not return underlying app instance",
			)
		}
		if got := lv.BuildID(); got != initialBuildID {
			t.Fatalf("LockedVorma.BuildID() = %q, want %q", got, initialBuildID)
		}
		if got := lv.RouteManifestFile(); got != initialRouteManifestFile {
			t.Fatalf(
				"LockedVorma.RouteManifestFile() = %q, want %q",
				got,
				initialRouteManifestFile,
			)
		}
		if got := lv.RootTemplate(); got == nil {
			t.Fatal("LockedVorma.RootTemplate() returned nil")
		}
		if got := lv.Paths(); got["/locked"] == nil {
			t.Fatal("LockedVorma.Paths() missing /locked")
		}
		if !lv.IsDev() {
			t.Fatal("LockedVorma.IsDev() = false, want true")
		}
	})
}

func TestSetIsDev_InvalidatesRouteDataCacheWhenModeChanges(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	cacheKey := routepipeline.BuildRouteDataCacheKey(
		nil,
		app.IsDevMode(),
		app.BuildID(),
		routeDataSnapshotVersionForTest(app),
	)
	putRouteDataCacheEntryForTest(
		app,
		cacheKey,
		&routepipeline.CachedItemSubset{ImportURLs: []string{"/cached.js"}},
	)

	snapshotVersionBefore := routeDataSnapshotVersionForTest(app)
	app.SetIsDev(!app.IsDevMode())
	snapshotVersionAfter := routeDataSnapshotVersionForTest(app)

	if got := routeDataCacheLenForTest(app); got != 0 {
		t.Fatalf(
			"route-data cache size = %d, want 0 after SetIsDev mode change",
			got,
		)
	}
	if snapshotVersionAfter <= snapshotVersionBefore {
		t.Fatalf(
			"route-data snapshot version should increase on mode change: before=%d after=%d",
			snapshotVersionBefore,
			snapshotVersionAfter,
		)
	}
}

func TestLockedVormaGetPaths_DoesNotExposeMutableInternalState(t *testing.T) {
	stage := defaultPathsFile("build-locked-getpaths-clone", map[string]*Path{
		"/locked": {
			OriginalPattern: "/locked",
			SrcPath:         "frontend/src/routes/locked.tsx",
			OutPath:         waveoutputtest.TestWaveOutputPath("routes/locked.js"),
			ExportKey:       "default",
			Deps:            []string{waveoutputtest.TestWaveOutputPath("chunk-locked.js")},
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	var leakedPaths map[string]*Path
	app.WithRLock(func(lv *ReadLockedVorma) {
		leakedPaths = lv.Paths()
	})

	leakedPaths["/new"] = &Path{OriginalPattern: "/new"}
	leakedPaths["/locked"].SrcPath = "frontend/src/routes/mutated.tsx"
	leakedPaths["/locked"].Deps[0] = waveoutputtest.TestWaveOutputPath("chunk-mutated.js")

	pathsSnapshot := app.Paths()
	if _, exists := pathsSnapshot["/new"]; exists {
		t.Fatalf(
			"LockedVorma.GetPaths leaked caller-added key into runtime state: %#v",
			pathsSnapshot,
		)
	}
	if got, want := pathsSnapshot["/locked"].SrcPath, "frontend/src/routes/locked.tsx"; got != want {
		t.Fatalf(
			"LockedVorma.GetPaths leaked SrcPath mutation: got %q want %q",
			got,
			want,
		)
	}
	if got, want := pathsSnapshot["/locked"].Deps[0], waveoutputtest.TestWaveOutputPath("chunk-locked.js"); got != want {
		t.Fatalf(
			"LockedVorma.GetPaths leaked deps mutation: got %q want %q",
			got,
			want,
		)
	}
}

func TestLockedVormaSetPaths_InvalidatesRouteDataCacheAndClonesInput(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-same", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         waveoutputtest.TestWaveOutputPath("routes/products.$id.old.js"),
			ExportKey:       "default",
			Deps:            []string{waveoutputtest.TestWaveOutputPath("chunk-old.js")},
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
				return map[string]string{"id": rd.Params()["id"]}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqOld := httptest.NewRequest(
		http.MethodGet,
		"/products/1?vorma_json=build-same",
		nil,
	)
	recOld := httptest.NewRecorder()
	handler.ServeHTTP(recOld, reqOld)
	if recOld.Code != http.StatusOK {
		t.Fatalf(
			"old response status = %d, want %d",
			recOld.Code,
			http.StatusOK,
		)
	}

	var oldData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recOld.Body.Bytes(), &oldData); err != nil {
		t.Fatalf("decode old route data: %v", err)
	}
	if got, want := oldData.ImportURLs, []string{waveoutputtest.TestWaveOutputURLPath("routes/products.$id.old.js")}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("old ImportURLs = %#v, want %#v", got, want)
	}
	if !containsString(oldData.Deps, waveoutputtest.TestWaveOutputURLPath("chunk-old.js")) {
		t.Fatalf("old Deps missing old chunk: %#v", oldData.Deps)
	}

	updatedPaths := map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         waveoutputtest.TestWaveOutputPath("routes/products.$id.new.js"),
			ExportKey:       "default",
			Deps:            []string{waveoutputtest.TestWaveOutputPath("chunk-new.js")},
		},
	}

	app.WithLock(func(lv *LockedVorma) {
		lv.SetPaths(updatedPaths)
	})

	updatedPaths["/products/:id"].SrcPath = "frontend/src/routes/products.$id.mutated.tsx"
	updatedPaths["/products/:id"].OutPath = waveoutputtest.TestWaveOutputPath("routes/products.$id.mutated.js")
	updatedPaths["/products/:id"].Deps[0] = waveoutputtest.TestWaveOutputPath("chunk-mutated.js")

	reqNew := httptest.NewRequest(
		http.MethodGet,
		"/products/2?vorma_json=build-same",
		nil,
	)
	recNew := httptest.NewRecorder()
	handler.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf(
			"new response status = %d, want %d",
			recNew.Code,
			http.StatusOK,
		)
	}

	var newData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recNew.Body.Bytes(), &newData); err != nil {
		t.Fatalf("decode new route data: %v", err)
	}
	if got, want := newData.ImportURLs, []string{waveoutputtest.TestWaveOutputURLPath("routes/products.$id.new.js")}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("new ImportURLs = %#v, want %#v", got, want)
	}
	if !containsString(newData.Deps, waveoutputtest.TestWaveOutputURLPath("chunk-new.js")) {
		t.Fatalf("new Deps missing new chunk: %#v", newData.Deps)
	}
	if containsString(newData.Deps, waveoutputtest.TestWaveOutputURLPath("chunk-old.js")) {
		t.Fatalf("new Deps should not include old chunk: %#v", newData.Deps)
	}
	if containsString(newData.Deps, waveoutputtest.TestWaveOutputURLPath("chunk-mutated.js")) {
		t.Fatalf(
			"new Deps should not include post-set caller mutation: %#v",
			newData.Deps,
		)
	}
}

func TestCoreAccessorsAndServerAddr(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	if got := app.ServerAddr(); got != ":8080" {
		t.Fatalf("ServerAddr() = %q, want %q", got, ":8080")
	}
	if app.LoadersRouter() == nil {
		t.Fatal("LoadersRouter() should not return nil")
	}
	if app.ActionsRouter() == nil {
		t.Fatal("ActionsRouter() should not return nil")
	}
}

func TestTSGenerationGettersReflectConfiguredValues(t *testing.T) {
	adHoc := []*tsgen.AdHocType{
		{
			TypeInstance: struct {
				Name string
			}{},
			TSTypeName: "UserDTO",
		},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		adHocTypes:  adHoc,
		extraTSCode: "export type Extra = string;",
	})
	app := fixture.app

	gotTypes := app.AdHocTypes()
	if len(gotTypes) != 1 {
		t.Fatalf("GetAdHocTypes length = %d, want %d", len(gotTypes), 1)
	}
	if gotTypes[0].TSTypeName != "UserDTO" {
		t.Fatalf(
			"GetAdHocTypes[0].TSTypeName = %q, want %q",
			gotTypes[0].TSTypeName,
			"UserDTO",
		)
	}
	if got := app.ExtraTSCode(); got != "export type Extra = string;" {
		t.Fatalf(
			"ExtraTSCode() = %q, want %q",
			got,
			"export type Extra = string;",
		)
	}
}

func TestGetAdHocTypes_DoesNotExposeMutableInternalState(t *testing.T) {
	inputAdHocTypes := []*tsgen.AdHocType{
		{
			TypeInstance: struct {
				Name string
			}{},
			TSTypeName: "UserDTO",
		},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		adHocTypes: inputAdHocTypes,
	})
	app := fixture.app

	// Mutating caller-owned input after app construction must not mutate runtime state.
	inputAdHocTypes[0].TSTypeName = "CallerMutated"
	_ = append(inputAdHocTypes, &tsgen.AdHocType{TSTypeName: "CallerAdded"})

	firstRead := app.AdHocTypes()
	if len(firstRead) != 1 {
		t.Fatalf(
			"GetAdHocTypes length after caller mutation = %d, want %d",
			len(firstRead),
			1,
		)
	}
	if firstRead[0] == nil {
		t.Fatal("GetAdHocTypes[0] should not be nil")
	}
	if firstRead[0].TSTypeName != "UserDTO" {
		t.Fatalf(
			"GetAdHocTypes[0].TSTypeName = %q, want %q",
			firstRead[0].TSTypeName,
			"UserDTO",
		)
	}

	// Mutating a getter result must not leak back into runtime state.
	firstRead[0].TSTypeName = "GetterMutated"
	_ = append(firstRead, &tsgen.AdHocType{TSTypeName: "GetterAdded"})

	secondRead := app.AdHocTypes()
	if len(secondRead) != 1 {
		t.Fatalf(
			"GetAdHocTypes length after getter mutation = %d, want %d",
			len(secondRead),
			1,
		)
	}
	if secondRead[0] == nil {
		t.Fatal("GetAdHocTypes[0] should not be nil")
	}
	if secondRead[0].TSTypeName != "UserDTO" {
		t.Fatalf(
			"GetAdHocTypes[0].TSTypeName after getter mutation = %q, want %q",
			secondRead[0].TSTypeName,
			"UserDTO",
		)
	}
}
