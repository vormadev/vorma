package wavebuild

import (
	"errors"

	"github.com/vormadev/vorma/kit/tasks"
)

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type phase1Effects struct {
	planPhase1RequestedEffects *tasks.Task[phase1BatchInput, phase1RequestedEffects]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var phase1EffectsDef = phase1Effects{
	planPhase1RequestedEffects: tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			input phase1BatchInput,
		) (phase1RequestedEffects, error) {
			if input.phase1 == nil {
				return phase1RequestedEffects{}, errors.New(
					"wavebuild: phase-1 input is required",
				)
			}
			phase1Facts, phase1FactsError := input.phase1.buildFacts()
			if phase1FactsError != nil {
				return phase1RequestedEffects{}, phase1FactsError
			}
			return phase1Facts.derivePhase1RequestedEffects(), nil
		},
	),
}
