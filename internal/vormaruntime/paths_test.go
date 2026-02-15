package vormaruntime

import "testing"

func TestVormaPathsStageFiles(t *testing.T) {
	if got, want := VormaPathsStageOneJSONPath(), "vorma_out/vorma_paths_stage_1.json"; got != want {
		t.Fatalf("VormaPathsStageOneJSONPath() = %q, want %q", got, want)
	}
	if got, want := VormaPathsStageTwoJSONPath(), "vorma_out/vorma_paths_stage_2.json"; got != want {
		t.Fatalf("VormaPathsStageTwoJSONPath() = %q, want %q", got, want)
	}
}
