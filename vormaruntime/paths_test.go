package vormaruntime

import "testing"

func TestVormaPathsStageFiles(t *testing.T) {
	if got, want := VormaPaths.StageOneJSON(), "vorma_out/vorma_paths_stage_1.json"; got != want {
		t.Fatalf("StageOneJSON() = %q, want %q", got, want)
	}
	if got, want := VormaPaths.StageTwoJSON(), "vorma_out/vorma_paths_stage_2.json"; got != want {
		t.Fatalf("StageTwoJSON() = %q, want %q", got, want)
	}
}
