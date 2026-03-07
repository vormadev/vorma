package wavebuild

import (
	"errors"

	"github.com/vormadev/vorma/kit/tasks"
)

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p1_effects struct {
	plan_p1_requested_effects *tasks.Task[p1_batch_input, p1_requested_effects]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p1_effects_def = p1_effects{
	plan_p1_requested_effects: tasks.NewTask(
		func(
			_ *tasks.Ctx,
			input p1_batch_input,
		) (p1_requested_effects, error) {
			if input.p1 == nil {
				return p1_requested_effects{}, errors.New(
					"wavebuild: phase-1 input is required",
				)
			}
			p1_facts, err := input.p1.build_facts()
			if err != nil {
				return p1_requested_effects{}, err
			}
			return p1_facts.derive_p1_requested_effects(), nil
		},
	),
}
