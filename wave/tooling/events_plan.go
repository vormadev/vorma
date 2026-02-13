package tooling

import "github.com/vormadev/vorma/wave"

// eventWithHooks pairs a classified event with its sorted hooks.
type eventWithHooks struct {
	classified         classifiedEvent
	hooks              *wave.SortedHooks
	hookCtx            *wave.HookContext
	runOnChangeOnly    bool
	needsHardReload    bool
	skipDuplicateHooks bool
}
