package main

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	phaseStatus, err := specutil.ParsePhaseStatusFile("spec/PHASE_STATUS.md")
	if err != nil {
		fail("%v", err)
	}
	if phaseStatus.Phase1Status != "COMPLETE" {
		return
	}

	dispatch, err := specutil.ParseDispatchFile("spec/MINING_DISPATCH.md")
	if err != nil {
		fail("%v", err)
	}
	for _, row := range dispatch.Rows {
		if row.Status != "DONE" {
			fail("phase1_status=COMPLETE requires all dispatch rows to be DONE; %s is %s", row.SlotID, row.Status)
		}
	}

	decisions, err := specutil.ParseDecisionsFile("spec/DECISIONS.md")
	if err != nil {
		fail("%v", err)
	}
	for _, row := range decisions {
		if row.Status == "OPEN" {
			fail("phase1_status=COMPLETE requires all decisions to be RESOLVED; %s is OPEN", row.DecisionID)
		}
	}
}
