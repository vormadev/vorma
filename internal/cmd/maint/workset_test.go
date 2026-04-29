package main

import (
	"reflect"
	"testing"
)

func TestWorkCatalogResolve_DedupesOverlappingRequests(t *testing.T) {
	app := maint_app{root: "/repo"}
	plan, err := app.work_catalog(framework_options{}, stress_options{}).resolve(
		work_gate,
		work_test_go,
		work_test_framework,
		work_test_go_framework,
	)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	ids := plan.ids()
	if ids.count(work_test_go_framework) != 1 {
		t.Fatalf("expected %s once, got ids %v", work_test_go_framework, ids)
	}
	if ids.count(work_install_js_framework) != 1 {
		t.Fatalf("expected %s once, got ids %v", work_install_js_framework, ids)
	}
	if ids.count(work_test_ts_framework) != 1 {
		t.Fatalf("expected %s once, got ids %v", work_test_ts_framework, ids)
	}
	if ids.count(work_typecheck_ts_core) != 1 {
		t.Fatalf("expected %s once, got ids %v", work_typecheck_ts_core, ids)
	}
}

func TestWorkCatalogResolve_UsesCanonicalOrder(t *testing.T) {
	app := maint_app{root: "/repo"}
	plan, err := app.work_catalog(framework_options{}, stress_options{}).resolve(
		work_test_framework,
		work_install_js,
		work_test_go_framework,
	)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	got := plan.ids()
	want := work_id_list{
		work_install_js_root,
		work_install_js_npm,
		work_install_js_create,
		work_install_js_framework,
		work_install_js_docs,
		work_typecheck_ts_core,
		work_typecheck_ts_preact,
		work_typecheck_ts_react,
		work_typecheck_ts_solid,
		work_typecheck_ts_vite,
		work_typecheck_ts_framework_tests,
		work_test_go_framework,
		work_test_ts_framework,
		work_typecheck_framework_fixture_ts,
		work_test_framework_prod,
		work_test_framework_dev,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolve() ids = %v, want %v", got, want)
	}
}

func TestWorkCatalogResolve_AppliesCoverage(t *testing.T) {
	app := maint_app{root: "/repo"}
	plan, err := app.work_catalog(framework_options{}, stress_options{}).resolve(
		work_test_lab,
		work_test_kit,
	)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	got := plan.ids()
	want := work_id_list{work_test_kit}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolve() ids = %v, want %v", got, want)
	}
}

func TestWorkCatalogResolve_StressUsesOwnershipDomains(t *testing.T) {
	app := maint_app{root: "/repo"}
	plan, err := app.work_catalog(framework_options{}, stress_options{intensity: 7}).resolve(
		work_stress,
	)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	got := plan.ids()
	want := work_id_list{
		work_install_js_framework,
		work_typecheck_ts_core,
		work_typecheck_ts_preact,
		work_typecheck_ts_react,
		work_typecheck_ts_solid,
		work_typecheck_ts_vite,
		work_typecheck_ts_framework_tests,
		work_test_ts_framework,
		work_typecheck_framework_fixture_ts,
		work_stress_root_go,
		work_stress_docs_go,
		work_stress_go_framework,
		work_stress_framework_prod,
		work_stress_framework_dev,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolve() ids = %v, want %v", got, want)
	}
}

func TestWorkCatalogResolve_RejectsCycles(t *testing.T) {
	catalog := work_catalog{
		items: map[work_id]work_item{
			"a": {id: "a", deps: []work_id{"b"}},
			"b": {id: "b", deps: []work_id{"a"}},
		},
		order: []work_id{"a", "b"},
	}

	if _, err := catalog.resolve("a"); err == nil {
		t.Fatal("expected cycle error")
	}
}

func (plan work_plan) ids() work_id_list {
	ids := make([]work_id, 0, len(plan))
	for _, item := range plan {
		ids = append(ids, item.id)
	}
	return ids
}

type work_id_list []work_id

func (ids work_id_list) count(target work_id) int {
	count := 0
	for _, id := range ids {
		if id == target {
			count++
		}
	}
	return count
}
