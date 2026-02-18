package vormaruntime

import "testing"

func TestVormaPathsStageFiles(t *testing.T) {
	if got, want := GetVormaPathsStageOneJSONPath(), "vorma_out/vorma_paths_stage_1.json"; got != want {
		t.Fatalf("GetVormaPathsStageOneJSONPath() = %q, want %q", got, want)
	}
	if got, want := GetVormaPathsStageTwoJSONPath(), "vorma_out/vorma_paths_stage_2.json"; got != want {
		t.Fatalf("GetVormaPathsStageTwoJSONPath() = %q, want %q", got, want)
	}
}
