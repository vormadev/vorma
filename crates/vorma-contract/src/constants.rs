//! Shared public and internal framework constants.

/// Environment key set while running a Vorma build/live-state entry.
pub const ENV_KEY_IS_BUILD: &str = "__VORMA_IS_BUILD";
/// Environment key set while running an app server under Vorma dev mode.
pub const ENV_KEY_IS_DEV: &str = "__VORMA_IS_DEV";
/// Standard enabled value for Vorma-owned boolean environment flags.
pub const ENV_VALUE_ENABLED: &str = "true";
/// Dev-only app server health endpoint path.
pub const DEV_HEALTH_PATH: &str = "/.vorma/healthz";
/// Prefix added to hashed public static asset output filenames.
pub const PUBLIC_STATIC_OUT_NAME_PREFIX: &str = "vorma_out_";
/// Browser root element id owned by Vorma.
pub const VORMA_ROOT_EL_ID: &str = "vorma-root";
/// SSR payload script element id consumed by the browser runtime.
pub const VORMA_DATA_JSON_SCRIPT_EL_ID: &str = "vorma-data-json";
/// Platform `FormData` TypeScript type name used by generated contracts.
pub const FORM_DATA_TYPE_NAME: &str = "FormData";
