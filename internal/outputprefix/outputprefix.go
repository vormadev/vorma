// Package outputprefix centralizes shared generated-file prefix constants used
// by Wave and Vorma runtime/buildtime artifacts.
//
// Keeping these prefixes in one place avoids drift across artifact producers
// and runtime consumers that must agree on file naming contracts.
package outputprefix

import "github.com/vormadev/vorma/wave/waveartifacts"

const (
	// HashedOutputPrefix is the generated filename prefix for hashed artifacts.
	HashedOutputPrefix = waveartifacts.HashedOutputPrefix
	// VitePrehashedFilePrefix is the prefix used for pre-hashed Vite outputs.
	VitePrehashedFilePrefix = HashedOutputPrefix + "vite_"
	// VormaRouteManifestPrefix is the prefix for generated Vorma route manifest
	// artifacts.
	VormaRouteManifestPrefix = HashedOutputPrefix + "vorma_internal_route_manifest_"
)
