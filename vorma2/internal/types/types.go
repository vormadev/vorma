package types

type PrivateFSRelPath string
type RoutePattern string
type SitePublicPath string

func (p PrivateFSRelPath) Str() string { return string(p) }
func (p RoutePattern) Str() string     { return string(p) }
func (s SitePublicPath) Str() string   { return string(s) }

type RoutePath struct {
	OriginalPattern RoutePattern
	ImportPath      SitePublicPath
	ExportKey       string
	ErrorExportKey  string
	Deps            []SitePublicPath
}

type RuntimeSnapshot struct {
	ServerBuildID     string
	ClientBuildID     string
	RootTemplatePath  PrivateFSRelPath
	UIVariant         string
	ClientEntryPath   SitePublicPath
	ClientEntryDeps   []SitePublicPath
	DepToCSSBundleMap map[SitePublicPath][]SitePublicPath
	Paths             map[RoutePattern]*RoutePath
}
