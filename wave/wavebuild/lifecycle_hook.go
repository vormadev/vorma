package wavebuild

import (
	"fmt"
	"strings"

	"github.com/vormadev/vorma/kit/strict"
)

func (h LifecycleHook) to_validated_hook(
	label string,
	root_dir strict.CWDRelPath,
) (*validated_lifecycle_hook, error) {
	if len(h.WatchIncludePatterns) == 0 {
		return nil, fmt.Errorf(
			"%s must have at least one WatchIncludePattern",
			label,
		)
	}

	name := label
	trimmed_given_name := strings.TrimSpace(h.Name)
	if trimmed_given_name != "" {
		name = trimmed_given_name
	}

	v := &validated_lifecycle_hook{
		name:                   name,
		plugin_name:            "", // set later for plugin hooks
		watch_include_patterns: h.WatchIncludePatterns,
		watch_exclude_patterns: h.WatchExcludePatterns,
		cmd:                    strings.TrimSpace(h.Cmd),
		fn:                     h.Fn,
		is_go_compile:          h.IsGoCompile,
		start_at:               CheckpointOrder(h.StartAt),
		finish_by:              CheckpointOrder(h.FinishBy),
		effects:                h.Effects,
		dev_only:               h.DevOnly,
		prod_only:              h.ProdOnly,
	}

	if v.cmd != "" && v.fn != nil {
		return nil, fmt.Errorf(
			"%s cannot set both Cmd and Fn", label,
		)
	}

	if v.is_go_compile {
		if h.StartAt != "" || h.FinishBy != "" || len(h.Effects) > 0 {
			return nil, fmt.Errorf(
				"%s: IsGoCompile cannot be combined with StartAt, FinishBy, or Effects",
				label,
			)
		}
		v.start_at = CheckpointOrder(Checkpoint_4_GoCompile)
		v.finish_by = CheckpointOrder(Checkpoint_4_GoCompile)
		v.effects = []Effect{EffectRestartApp}
	}

	if !v.is_go_compile {
		if h.StartAt == "" {
			v.start_at = CheckpointOrder(Checkpoint_1_CycleStart)
		}
		if h.FinishBy == "" {
			v.finish_by = CheckpointOrder(Checkpoint_7_CycleEnd)
		}
	}

	if !is_valid_checkpoint_ord(v.start_at) {
		return nil, fmt.Errorf(
			"%s has invalid StartAt checkpoint: %s",
			label,
			h.StartAt,
		)
	}
	if !is_valid_checkpoint_ord(v.finish_by) {
		return nil, fmt.Errorf(
			"%s has invalid FinishBy checkpoint: %s",
			label,
			h.FinishBy,
		)
	}
	if v.start_at > v.finish_by {
		return nil, fmt.Errorf(
			"%s has StartAt checkpoint that is after FinishBy checkpoint: StartAt=%s, FinishBy=%s",
			label,
			h.StartAt,
			h.FinishBy,
		)
	}
	for _, effect := range v.effects {
		if !is_valid_hook_effect(effect) {
			return nil, fmt.Errorf(
				"%s has invalid Effect: %s", label, effect,
			)
		}
	}
	if v.dev_only && v.prod_only {
		return nil, fmt.Errorf(
			"%s cannot be both DevOnly and ProdOnly", label,
		)
	}

	for j, pattern := range v.watch_include_patterns {
		v.watch_include_patterns[j] = to_catch_all_pattern_if_dir(
			root_dir.Join(pattern.MustNormalize().Str()),
		)
	}
	for j, pattern := range v.watch_exclude_patterns {
		v.watch_exclude_patterns[j] = to_catch_all_pattern_if_dir(
			root_dir.Join(pattern.MustNormalize().Str()),
		)
	}

	return v, nil
}

type validated_lifecycle_hook struct {
	name        string // for logging
	plugin_name string // non-empty for plugin hooks (used in PluginCtx)

	watch_include_patterns []strict.CWDRelPath
	watch_exclude_patterns []strict.CWDRelPath

	cmd string
	fn  func(ctx *PluginCtx) (*PluginResult, error)

	is_go_compile bool
	start_at      CheckpointOrd
	finish_by     CheckpointOrd
	effects       []Effect
	dev_only      bool
	prod_only     bool
}
