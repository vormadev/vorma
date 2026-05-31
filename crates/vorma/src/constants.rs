/// Query key used by JSON loader requests.
pub const QUERY_KEY_VORMA_JSON: &str = "vorma-json";

/// Header carrying the current client build ID.
pub const X_VORMA_CLIENT_BUILD_ID: &str = "X-Vorma-Client-Build-Id";
/// Header marking a stale client build.
pub const X_VORMA_BUILD_SKEW: &str = "X-Vorma-Build-Skew";

/// Prefix for hashed public static output filenames.
pub const PUBLIC_STATIC_OUT_NAME_PREFIX: &str = "vorma_out_";
/// Temporary Vite manifest filename written during production builds.
pub const PROD_TMP_VITE_MANIFEST_FILENAME: &str = "vorma_internal_tmp_vite_manifest.json";

/// DOM id for the critical CSS style element.
pub const CRITICAL_CSS_EL_ID: &str = "vorma-critical-css";
/// DOM id for the Vorma root element.
pub const VORMA_ROOT_EL_ID: &str = "vorma-root";
/// DOM id for the embedded loader payload script.
pub const VORMA_DATA_JSON_SCRIPT_EL_ID: &str = "vorma-data-json";

/// Attribute used to identify CSS bundle links.
pub const CSS_BUNDLE_ATTR: &str = "data-vorma-css-bundle";

/// Route pattern prefix for dynamic params.
pub const DYNAMIC_PARAM_PREFIX: char = ':';
/// Route pattern segment for splat captures.
pub const SPLAT_SEGMENT_IDENTIFIER: char = '*';
/// Route pattern segment for explicit index children.
pub const EXPLICIT_INDEX_SEGMENT_IDENTIFIER: &str = "_index";

/// Environment key set for dev app server processes.
pub const ENV_KEY_IS_DEV: &str = "__VORMA_IS_DEV";
/// Environment key set for build/live-state processes.
pub const ENV_KEY_IS_BUILD: &str = "__VORMA_IS_BUILD";
