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
	GoSrcChanged             Trigger = "go_src_changed"
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
	CompileGo             Effect = "compile_go"
	RestartAppServer      Effect = "restart_app_server"
	NotifyVite            Effect = "notify_vite"
	ShowRebuildingOverlay Effect = "show_rebuilding_overlay"
	HardReloadBrowser     Effect = "hard_reload_browser"
	HotCSSRefresh         Effect = "hot_css_refresh"
	ClientSideRevalidate  Effect = "client_side_revalidate"
)

/////////////////////////////////////////////////////////////////////
/////// Derivation
/////////////////////////////////////////////////////////////////////

type DeriveOpts struct {
	Triggers          *set.Set[Trigger]
	HasPrivateStatic  bool
	HasPublicStatic   bool
	HasCriticalCSS    bool
	HasNonCriticalCSS bool
	UsingVite         bool
}

// Derive computes the full set of effects from a set of triggers.
// All implication chains are resolved here. Effects for unconfigured
// features (e.g. private static when HasPrivateStatic is false) are
// never emitted.
func Derive(opts DeriveOpts) *set.Set[Effect] {
	fx := &set.Set[Effect]{}
	has := func(t Trigger) bool { return opts.Triggers.Has(t) }

	// config change implies full rebuild of applicable targets
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
		fx.Add(CompileGo)
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
	if has(GoSrcChanged) {
		fx.Add(CompileGo)
	}

	// implication chains
	if fx.Has(CompileGo) {
		fx.Add(RestartAppServer)
	}
	if fx.Has(RestartAppServer) {
		fx.Add(ShowRebuildingOverlay)
		fx.Add(HardReloadBrowser)
	}

	// public static → vite or hard reload
	if has(PublicStaticSrcChanged) {
		if opts.UsingVite {
			fx.Add(NotifyVite)
		} else {
			fx.Add(HardReloadBrowser)
		}
	}

	// css refresh: only if css changed and no hard reload
	if !fx.Has(HardReloadBrowser) &&
		(fx.Has(BuildCriticalCSS) || fx.Has(BuildNonCriticalCSS)) {
		fx.Add(HotCSSRefresh)
	}

	return fx
}
