package wavebuild

import (
	"errors"

	"github.com/vormadev/vorma/kit/tasks"
)

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p1_Effects struct {
	planP1_RequestedEffects *tasks.Task[p1_BatchInput, p1_RequestedEffects]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p1_EffectsDef = p1_Effects{
	planP1_RequestedEffects: tasks.NewTask(
		func(
			_ *tasks.Ctx,
			input p1_BatchInput,
		) (p1_RequestedEffects, error) {
			if input.p1 == nil {
				return p1_RequestedEffects{}, errors.New(
					"wavebuild: phase-1 input is required",
				)
			}
			p1_Facts, p1_FactsError := input.p1.buildFacts()
			if p1_FactsError != nil {
				return p1_RequestedEffects{}, p1_FactsError
			}
			return p1_Facts.deriveP1_RequestedEffects(), nil
		},
	),
}
