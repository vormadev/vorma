package vormaruntime

import (
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
)

func TestVormaPathsStageFiles(t *testing.T) {
	if got, want := runtimepaths.GetVormaPathsStageOneJSONPath(), "vorma_out/vorma_paths_stage_1.json"; got != want {
		t.Fatalf(
			"runtimepaths.GetVormaPathsStageOneJSONPath() = %q, want %q",
			got,
			want,
		)
	}
	if got, want := runtimepaths.GetVormaPathsStageTwoJSONPath(), "vorma_out/vorma_paths_stage_2.json"; got != want {
		t.Fatalf(
			"runtimepaths.GetVormaPathsStageTwoJSONPath() = %q, want %q",
			got,
			want,
		)
	}
}
