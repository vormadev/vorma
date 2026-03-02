// Package waveartifacts defines build artifact naming and directory conventions.
//
// These symbols are consumed by Wave build/dev tooling and are intentionally
// separate from the end-user runtime `wave` API surface.
package waveartifacts

const (
	// AssetsDirname is the static assets directory segment.
	AssetsDirname = "assets"
	// PublicDirname is the static-public directory segment.
	PublicDirname = "public"
	// PrivateDirname is the static-private directory segment.
	PrivateDirname = "private"
	// InternalDirname is the static-internal directory segment.
	InternalDirname = "internal"
)

// PrehashedDirname is the static directory segment for prehashed assets.
const PrehashedDirname = "prehashed"

// NohashDirname is the static directory segment for assets that should not be
// content-hashed.
const NohashDirname = "__nohash"

// HashedOutputPrefix is the generated filename prefix for hashed artifacts.
const HashedOutputPrefix = "wave_out_"

// HashedOutputDirname is the static-public directory name holding hashed
// outputs.
const HashedOutputDirname = "wave_out"

const (
	// CriticalCSSFileName is the internal file containing critical CSS output.
	CriticalCSSFileName = "critical.css"
	// NormalCSSRefFileName points to the current non-critical CSS artifact.
	NormalCSSRefFileName = "normal_css_file_ref.txt"
	// PublicFileMapRefFileName points to the current public file map artifact.
	PublicFileMapRefFileName = "public_file_map_file_ref.txt"
	// PublicFileMapGobFileName is the gob-encoded public file map snapshot.
	PublicFileMapGobFileName = "public_filemap.gob"
	// PrivateFileMapGobFileName is the gob-encoded private file map snapshot.
	PrivateFileMapGobFileName = "private_filemap.gob"
	// BuildOutputLedgerFileName tracks recorded build outputs.
	BuildOutputLedgerFileName = "build_output_ledger.txt"
)

const (
	// WaveInternalDirname is the private dist directory for Wave-owned internal
	// build artifacts.
	WaveInternalDirname = "wave_owned"
	// ViteManifestFileName is the Vite manifest filename under Wave-owned
	// private artifacts.
	ViteManifestFileName = "vite_manifest.json"
)

// ownedArtifactOutputPrefix is the generated filename prefix for Wave-owned
// internal artifacts.
const ownedArtifactOutputPrefix = "wave_owned_"

// PublicFileMapJSONName is the generated JSON public filemap name.
const PublicFileMapJSONName = "filemap.json"

// ApplyWaveFileOutputPrefix adds Wave's hashed-output filename prefix.
func ApplyWaveFileOutputPrefix(fileNameSuffix string) string {
	return HashedOutputPrefix + fileNameSuffix
}

// ApplyWaveOwnedFileOutputPrefix adds Wave-owned internal-artifact filename
// prefix.
func ApplyWaveOwnedFileOutputPrefix(fileNameSuffix string) string {
	return ownedArtifactOutputPrefix + fileNameSuffix
}

// ViteManifestFilePath returns the relative path under static/private for Vite
// manifest output.
func ViteManifestFilePath() string {
	return WaveInternalDirname + "/" + ViteManifestFileName
}
