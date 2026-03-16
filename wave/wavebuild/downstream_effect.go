package wavebuild

type DownstreamEffect string

func (d DownstreamEffect) Str() string { return string(d) }

const (
	DownstreamEffectNone                 DownstreamEffect = ""
	DownstreamEffectAppRestart           DownstreamEffect = "app_restart"
	DownstreamEffectHardReloadBrowser    DownstreamEffect = "hard_reload_browser"
	DownstreamEffectClientDataRevalidate DownstreamEffect = "client_data_revalidate"
)

func is_valid_downstream_effect(s DownstreamEffect) bool {
	switch s {
	case DownstreamEffectNone,
		DownstreamEffectAppRestart,
		DownstreamEffectHardReloadBrowser,
		DownstreamEffectClientDataRevalidate:
		return true
	default:
		return false
	}
}
