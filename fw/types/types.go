package types

import "github.com/vormadev/vorma/kit/mux"

// Path represents a route path with its associated metadata.
type Path struct {
	NestedRoute mux.AnyNestedRoute `json:"-"`

	// Both stages one and two
	OriginalPattern string `json:"originalPattern"`
	SrcPath         string `json:"srcPath"`
	ExportKey       string `json:"exportKey"`
	ErrorExportKey  string `json:"errorExportKey,omitempty"`

	// Stage two only
	OutPath string   `json:"outPath,omitempty"`
	Deps    []string `json:"deps,omitempty"`
}

// UIVariant represents the UI framework variant.
type UIVariant string

var UIVariants = struct {
	React  UIVariant
	Preact UIVariant
	Solid  UIVariant
}{
	React:  "react",
	Preact: "preact",
	Solid:  "solid",
}

// VormaConfig holds Vorma-specific configuration.
type VormaConfig struct {
	IncludeDefaults            *bool  `json:"IncludeDefaults,omitempty"`
	UIVariant                  string `json:"UIVariant"`
	HTMLTemplateLocation       string `json:"HTMLTemplateLocation"`
	ClientEntry                string `json:"ClientEntry"`
	ClientRouteDefsFile        string `json:"ClientRouteDefsFile"`
	TSGenOutDir                string `json:"TSGenOutDir"`
	BuildtimePublicURLFuncName string `json:"BuildtimePublicURLFuncName,omitempty"`
}

// PathsFile represents the serialized paths data written to disk.
type PathsFile struct {
	Stage             string           `json:"stage"`
	BuildID           string           `json:"buildID,omitempty"`
	ClientEntrySrc    string           `json:"clientEntrySrc"`
	Paths             map[string]*Path `json:"paths"`
	RouteManifestFile string           `json:"routeManifestFile"`

	// Stage two only
	ClientEntryOut    string            `json:"clientEntryOut,omitempty"`
	ClientEntryDeps   []string          `json:"clientEntryDeps,omitempty"`
	DepToCSSBundleMap map[string]string `json:"depToCSSBundleMap,omitempty"`
}
