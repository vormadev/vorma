//! Shared public and internal framework constants.
//!
//! Framework-integration surface: these are the fixed strings `vorma` and
//! `vorma-build` both need to agree on without importing from each other —
//! environment variable names, reserved DOM ids, and the TypeScript names
//! for platform types the [`crate::tsgen`] model does not otherwise carry a
//! Rust type for. An application author does not read this module; `vorma`
//! re-exports the couple of values (like
//! [`PUBLIC_STATIC_OUT_NAME_PREFIX`](crate::constants::PUBLIC_STATIC_OUT_NAME_PREFIX))
//! that app code has any reason to see.

/// Environment key set while running a Vorma build/live-state entry.
///
/// A build-entry process (the same app binary, invoked to emit its
/// [`crate::live_state`] JSON instead of serving requests) sets this so
/// application code can distinguish "I am being asked for my graph" from
/// "I am serving traffic" — see [`ENV_VALUE_ENABLED`] for the value it is
/// set to.
pub const ENV_KEY_IS_BUILD: &str = "__VORMA_IS_BUILD";
/// Environment key set while running an app server under Vorma dev mode.
///
/// Distinguishes a dev-mode app server process from a production one, for
/// the same reasons as [`ENV_KEY_IS_BUILD`].
pub const ENV_KEY_IS_DEV: &str = "__VORMA_IS_DEV";
/// Standard enabled value for Vorma-owned boolean environment flags.
///
/// Both [`ENV_KEY_IS_BUILD`] and [`ENV_KEY_IS_DEV`] are set to this value
/// when true and left unset (rather than set to a false-ish value) when
/// false.
pub const ENV_VALUE_ENABLED: &str = "true";
/// Dev-only app server health endpoint path.
///
/// The dev orchestrator polls this path to detect when a rebuilt app
/// server binary has come back up and is ready to receive requests again.
pub const DEV_HEALTH_PATH: &str = "/.vorma/healthz";
/// Prefix added to hashed public static asset output filenames.
///
/// Every public static asset is content-hashed and renamed with this
/// prefix at build time, so the public static base can be served with
/// far-future cache headers without risking a stale-content collision.
pub const PUBLIC_STATIC_OUT_NAME_PREFIX: &str = "vorma_out_";
/// Browser root element id owned by Vorma.
///
/// The DOM node the client runtime mounts the app into. Reserved: an
/// application's own document markup must not declare an element with
/// this id.
pub const VORMA_ROOT_EL_ID: &str = "vorma-root";
/// SSR payload script element id consumed by the browser runtime.
///
/// Identifies the inline `<script>` element carrying the serialized SSR
/// bootstrap payload the browser runtime hydrates from. Reserved, like
/// [`VORMA_ROOT_EL_ID`].
pub const VORMA_DATA_JSON_SCRIPT_EL_ID: &str = "vorma-data-json";
/// Platform `FormData` TypeScript type name used by generated contracts.
///
/// [`crate::contracts::TypeRefContract::FormData`] renders as this
/// identifier — the browser's built-in `FormData` type, not a
/// framework-generated one.
pub const FORM_DATA_TYPE_NAME: &str = "FormData";
/// Platform `Blob` TypeScript type name used by generated contracts.
///
/// [`crate::contracts::TypeRefContract::Blob`] renders as this
/// identifier — the browser's built-in `Blob` type, not a
/// framework-generated one.
pub const BLOB_TYPE_NAME: &str = "Blob";
