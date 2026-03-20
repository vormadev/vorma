package constants

const (
	/////// BUILD OUTPUTS
	DIST_DIRNAME                  = ".waveout"                           // .waveout/
	STATIC_DIRNAME                = "static"                             // .waveout/static/
	KEEP_FILENAME                 = ".keep"                              // .waveout/static/.keep
	SCHEMA_JSON_FILENAME          = "schema.json"                        // .waveout/schema.json
	RUNTIME_CFG_JSON_FILENAME     = "runtime_config.json"                // .waveout/static/internal/runtime_config.json
	PUBLIC_FILEMAP_JSON_FILENAME  = "public_filemap.json"                // .waveout/static/internal/public_filemap.json
	PRIVATE_FILEMAP_JSON_FILENAME = "private_filemap.json"               // .waveout/static/internal/private_filemap.json
	CRITICAL_CSS_FILENAME         = "critical.css"                       // .waveout/static/internal/critical.css
	STATIC_ASSETS_PRIVATE_DIR     = "static/assets/private"              // .waveout/static/assets/private/
	STATIC_ASSETS_PUBLIC_DIR      = "static/assets/public"               // .waveout/static/assets/public/
	PUBLIC_STATIC_FILE_PREFIX     = "wave_out_"                          // .waveout/static/assets/public/wave_out_<hash>.<ext>
	NON_CRITICAL_CSS_FILENAME     = "wave_internal_non_critical_css.css" // .waveout/static/assets/public/wave_out_wave_internal_non_critical_css_<hash>.css
	PUBLIC_FILEMAP_FILENAME       = "wave_internal_public_filemap.json"  // .waveout/static/assets/public/wave_out_wave_internal_public_filemap_<hash>.json
	STATIC_INTERNAL_DIR           = "static/internal"                    // .waveout/static/internal/

	/////// LOCKS AND PIDS
	WAVE_LOCK_FILENAME = "wave.lock" // .waveout/wave.lock
	APP_PID_FILENAME   = "app.pid"   // .waveout/app.pid
	VITE_PID_FILENAME  = "vite.pid"  // .waveout/vite.pid

	/////// CLIENT-SIDE RUNTIME
	CRITICAL_CSS_EL_ID             = "wave-critical-css"       // <style id="wave-critical-css">
	NON_CRITICAL_CSS_EL_ID         = "wave-non-critical-css"   // <link rel="stylesheet" id="wave-non-critical-css">
	REBUILDING_OVERLAY_EL_ID       = "wave-rebuilding-overlay" // <div id="wave-rebuilding-overlay">
	DATA_REVALIDATE_FN_SYMBOL_KEY  = "wave-data-revalidate-fn" // window[Symbol.for("wave-data-revalidate-fn")]
	PUBLIC_FILEMAP_META_EL_ID      = "wave-public-filemap-url" // <meta id="wave-public-filemap-url" data-url="...">
	DEV_REFRESH_EVENTS_PATH_PREFIX = "/wave-dev-refresh-"      // e.g. /wave-dev-refresh-<token>

	/////// PROD RUNTIME ENV VARS -- host's responsibility to set
	ENV_KEY_RUNTIME_PORT = "PORT"

	/////// DEV RUNTIME ENV VARS -- injected via dev app supervisor
	ENV_KEY_DEV_RUNTIME_IS_DEV        = "WAVE_IS_DEV" // same as builtime, separated for semantic clarity
	ENV_KEY_DEV_RUNTIME_REFRESH_PORT  = "WAVE_DEV_REFRESH_PORT"
	ENV_KEY_DEV_RUNTIME_REFRESH_TOKEN = "WAVE_DEV_REFRESH_TOKEN"
	ENV_KEY_DEV_RUNTIME_STATIC_DIR    = "WAVE_DEV_STATIC_DIR"
	ENV_KEY_DEV_RUNTIME_VITE_PORT     = "WAVE_DEV_VITE_PORT"

	/////// BUILDTIME ENV VARS -- injected into hook shell commands
	ENV_KEY_DEV_BUILDTIME_IS_DEV      = "WAVE_IS_DEV" // same as runtime, separated for semantic clarity
	ENV_KEY_BUILDTIME_BIN_OUTPUT_PATH = "WAVE_BIN_OUTPUT_PATH"
	ENV_KEY_BUILDTIME_ROOT_DIR        = "WAVE_ROOT_DIR"
	ENV_KEY_BUILDTIME_BUILD_TAGS      = "WAVE_BUILD_TAGS"

	/////// PUBLIC STATIC ASSET HASHING EXCLUDE DIR
	PUBLIC_STATIC_EXCLUDE_DIR = "__prehashed" // <user-public-assets-src>/__prehashed/*
)
