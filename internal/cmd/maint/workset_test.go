package main

import "testing"

func TestMaintTasks_DryRunGate(t *testing.T) {
	app := maint_app{root: "/repo", dry_run: true}
	if err := app.run_work(work_gate); err != nil {
		t.Fatalf("run_work(%s) error = %v", work_gate, err)
	}
}

func TestMaintTasks_DryRunStressUsesIntensity(t *testing.T) {
	app := maint_app{root: "/repo", dry_run: true}
	if err := app.run_stress_work(stress_options{intensity: 7}, work_stress); err != nil {
		t.Fatalf("run_stress_work(%s) error = %v", work_stress, err)
	}
}

func TestMaintTasks_RejectUnknownWork(t *testing.T) {
	app := maint_app{root: "/repo", dry_run: true}
	if err := app.run_work("missing"); err == nil {
		t.Fatal("expected unknown work error")
	}
}
