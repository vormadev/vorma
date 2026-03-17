package wavebuild

import "github.com/vormadev/vorma/kit/set"

type trigger string

const (
	config_changed               trigger = "config_changed"
	private_static_src_changed   trigger = "private_static_src_changed"
	public_static_src_changed    trigger = "public_static_src_changed"
	critical_css_src_changed     trigger = "critical_css_src_changed"
	non_critical_css_src_changed trigger = "non_critical_css_src_changed"
)

type Effect string

func (e Effect) Str() string { return string(e) }

const (
	// Hook-settable effects.
	EffectRestartApp                    Effect = "restart_app"
	EffectHardReloadBrowser             Effect = "hard_reload_browser"
	EffectRevalidateClientData          Effect = "revalidate_client_data"
	EffectProcessPrivateStatic          Effect = "process_private_static"
	EffectShowFrontendRebuildingOverlay Effect = "show_frontend_rebuilding_overlay"
	EffectNoFrontendSettling            Effect = "no_frontend_settling"

	// Wave-owned internal effects.
	effect_build_private_filemap  Effect = "build_private_filemap"
	effect_build_public_filemap   Effect = "build_public_filemap"
	effect_build_critical_css     Effect = "build_critical_css"
	effect_build_non_critical_css Effect = "build_non_critical_css"
)

func is_valid_effect(s Effect) bool {
	switch s {
	case EffectRestartApp,
		EffectHardReloadBrowser,
		EffectRevalidateClientData,
		EffectProcessPrivateStatic,
		EffectShowFrontendRebuildingOverlay,
		EffectNoFrontendSettling,
		effect_build_private_filemap,
		effect_build_public_filemap,
		effect_build_critical_css,
		effect_build_non_critical_css:
		return true
	default:
		return false
	}
}

func is_valid_hook_effect(s Effect) bool {
	switch s {
	case EffectRestartApp,
		EffectHardReloadBrowser,
		EffectRevalidateClientData,
		EffectProcessPrivateStatic,
		EffectShowFrontendRebuildingOverlay,
		EffectNoFrontendSettling:
		return true
	default:
		return false
	}
}

type derive_workset_opts struct {
	triggers             *set.Set[trigger]
	has_private_static   bool
	has_public_static    bool
	has_critical_css     bool
	has_non_critical_css bool
	no_frontend_settling bool
}

// derive_workset computes Wave-owned cycle effects from the current trigger set.
// Hook effects are merged separately by the hook scheduler.
func derive_workset(opts derive_workset_opts) *set.Set[Effect] {
	fx := &set.Set[Effect]{}
	has := func(t trigger) bool { return opts.triggers.Has(t) }

	if has(config_changed) {
		if opts.has_private_static {
			fx.Add(effect_build_private_filemap)
		}
		if opts.has_public_static {
			fx.Add(effect_build_public_filemap)
		}
		if opts.has_critical_css {
			fx.Add(effect_build_critical_css)
		}
		if opts.has_non_critical_css {
			fx.Add(effect_build_non_critical_css)
		}
	}

	if has(private_static_src_changed) && opts.has_private_static {
		fx.Add(effect_build_private_filemap)
	}
	if has(public_static_src_changed) && opts.has_public_static {
		fx.Add(effect_build_public_filemap)
		if opts.has_critical_css {
			fx.Add(effect_build_critical_css)
		}
		if opts.has_non_critical_css {
			fx.Add(effect_build_non_critical_css)
		}
	}
	if has(critical_css_src_changed) && opts.has_critical_css {
		fx.Add(effect_build_critical_css)
	}
	if has(non_critical_css_src_changed) && opts.has_non_critical_css {
		fx.Add(effect_build_non_critical_css)
	}

	if has(public_static_src_changed) && !opts.no_frontend_settling {
		fx.Add(EffectHardReloadBrowser)
	}

	return fx
}
