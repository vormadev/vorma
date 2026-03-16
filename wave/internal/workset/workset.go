package workset

import "github.com/vormadev/vorma/kit/set"

/////////////////////////////////////////////////////////////////////
/////// Triggers
/////////////////////////////////////////////////////////////////////

type Trigger string

const (
	ConfigChanged            Trigger = "config_changed"
	PrivateStaticSrcChanged  Trigger = "private_static_src_changed"
	PublicStaticSrcChanged   Trigger = "public_static_src_changed"
	CriticalCSSSrcChanged    Trigger = "critical_css_src_changed"
	NonCriticalCSSSrcChanged Trigger = "non_critical_css_src_changed"
)

/////////////////////////////////////////////////////////////////////
/////// Effects
/////////////////////////////////////////////////////////////////////

type Effect string

const (
	BuildPrivateFilemap   Effect = "build_private_filemap"
	BuildPublicFilemap    Effect = "build_public_filemap"
	BuildCriticalCSS      Effect = "build_critical_css"
	BuildNonCriticalCSS   Effect = "build_non_critical_css"
	RestartAppServer      Effect = "restart_app_server"
	ShowRebuildingOverlay Effect = "show_rebuilding_overlay"
	HardReloadBrowser     Effect = "hard_reload_browser"
	ClientDataRevalidate  Effect = "client_data_revalidate"
)

/////////////////////////////////////////////////////////////////////
/////// Derivation
/////////////////////////////////////////////////////////////////////

type DeriveOpts struct {
	Triggers                               *set.Set[Trigger]
	HasPrivateStatic                       bool
	HasPublicStatic                        bool
	HasCriticalCSS                         bool
	HasNonCriticalCSS                      bool
	PluginOwnsPublicStaticFrontendSettling bool
}

// Derive computes the set of asset-related effects from a set of
// triggers. All implication chains within the asset pipeline are
// resolved here. Effects for unconfigured features (e.g. private
// static when HasPrivateStatic is false) are never emitted.
//
// Restart/reload effects are not produced here — they originate
// from lifecycle hooks via DownstreamEffect and are merged into the
// effect set by the hook scheduler.
func Derive(opts DeriveOpts) *set.Set[Effect] {
	fx := &set.Set[Effect]{}
	has := func(t Trigger) bool { return opts.Triggers.Has(t) }

	// config change implies full rebuild of applicable asset targets
	if has(ConfigChanged) {
		if opts.HasPrivateStatic {
			fx.Add(BuildPrivateFilemap)
		}
		if opts.HasPublicStatic {
			fx.Add(BuildPublicFilemap)
		}
		if opts.HasCriticalCSS {
			fx.Add(BuildCriticalCSS)
		}
		if opts.HasNonCriticalCSS {
			fx.Add(BuildNonCriticalCSS)
		}
	}

	// direct trigger → action
	if has(PrivateStaticSrcChanged) && opts.HasPrivateStatic {
		fx.Add(BuildPrivateFilemap)
	}
	if has(PublicStaticSrcChanged) && opts.HasPublicStatic {
		fx.Add(BuildPublicFilemap)
		if opts.HasCriticalCSS {
			fx.Add(BuildCriticalCSS)
		}
		if opts.HasNonCriticalCSS {
			fx.Add(BuildNonCriticalCSS)
		}
	}
	if has(CriticalCSSSrcChanged) && opts.HasCriticalCSS {
		fx.Add(BuildCriticalCSS)
	}
	if has(NonCriticalCSSSrcChanged) && opts.HasNonCriticalCSS {
		fx.Add(BuildNonCriticalCSS)
	}

	// public static changes get a hard reload unless a plugin has
	// claimed ownership of public static settling (e.g. to do
	// finer-grained vite notification instead)
	if has(PublicStaticSrcChanged) &&
		!opts.PluginOwnsPublicStaticFrontendSettling {
		fx.Add(HardReloadBrowser)
	}

	return fx
}
