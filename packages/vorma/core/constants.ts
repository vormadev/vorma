// Wire-protocol constants are generated from the Rust definitions; see
// wire_contracts.gen.ts (never hand-edited — see that file's own header).
// Re-exported here so import sites stay stable. `BUILD_ID_HEADER` and
// `DATA_SCRIPT_ID` are public (re-exported through `_index.ts`) and
// re-declared below with real doc comments, since a bare `export { X }
// from "./generated.ts"` cannot itself carry documentation the generated
// file doesn't have; the rest stay plain re-exports as internal
// wire-protocol plumbing.
export {
	VORMA_JSON_KEY,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
	X_VORMA_RESOURCE_BODY,
} from "./wire_contracts.gen.ts";
import {
	BUILD_ID_HEADER as GENERATED_BUILD_ID_HEADER,
	DATA_SCRIPT_ID as GENERATED_DATA_SCRIPT_ID,
} from "./wire_contracts.gen.ts";

/** Response header carrying the server's current client build id — the build-skew detection signal (see `BuildSkewDetectedEvent`). */
export const BUILD_ID_HEADER = GENERATED_BUILD_ID_HEADER;

/** Element id of the embedded `<script>` payload `boot()` reads on initial load. */
export const DATA_SCRIPT_ID = GENERATED_DATA_SCRIPT_ID;

export const VERCEL_X_DEPLOYMENT_ID = "x-deployment-id";
export const VERCEL_DPL_QUERY_PARAM_KEY = "dpl";

export const SCROLL_STORAGE_KEY = "vorma-scroll-state";
export const SCROLL_STORAGE_RELOAD_KEY = "vorma-scroll-state-reload";
export const VORMA_ROOT_EL_ID = "vorma-root";

export const HISTORY_KEY_FIELD = "vorma-history-key";
export const HISTORY_USER_STATE_FIELD = "vorma-user-state";

export const API_IDENTITY_ARRAY_PREFIX = "vorma-api";

/*
`data-*` attribute names `Link` sets on its rendered anchor when
active/pending (see `LinkAttributeMatchRules`/`LinkPropsBase`
`attributeMatchRules` for the comparison rules that decide these) — style
off these selectors rather than a class prop. Public: an app CSS file
selecting `[data-vorma-active-exact]` reads these literal strings, so the
constants exist for code that needs the name programmatically (a custom
`Link` wrapper, a test) rather than as a hidden implementation detail.
*/

/** Set when a `Link`'s target exactly matches the current route (see the module comment above). */
export const LINK_ACTIVE_EXACT_ATTR = "data-vorma-active-exact";
/** Set when a `Link`'s target is a still-matched ancestor of the current route. */
export const LINK_ACTIVE_ANCESTOR_ATTR = "data-vorma-active-ancestor";
/** Set when a `Link`'s target exactly matches the in-flight navigation's destination. */
export const LINK_PENDING_EXACT_ATTR = "data-vorma-pending-exact";
/** Set when a `Link`'s target is an ancestor of the in-flight navigation's destination. */
export const LINK_PENDING_ANCESTOR_ATTR = "data-vorma-pending-ancestor";

export const CSS_BUNDLE_ATTR = "data-vorma-css-bundle";
export const CSS_PRELOAD_ATTR = "data-vorma-css-preload";
export const CSS_PRELOAD_SETTLED_ATTR = "data-vorma-css-settled";
