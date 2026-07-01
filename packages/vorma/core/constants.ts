// Wire-protocol constants are generated from the Rust definitions; see
// wire_contracts.gen.ts. Re-exported here so import sites stay stable.
export {
	BUILD_ID_HEADER,
	DATA_SCRIPT_ID,
	VORMA_JSON_KEY,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
	X_VORMA_RESOURCE_BODY,
} from "./wire_contracts.gen.ts";

export const VERCEL_X_DEPLOYMENT_ID = "x-deployment-id";
export const VERCEL_DPL_QUERY_PARAM_KEY = "dpl";

export const SCROLL_STORAGE_KEY = "vorma-scroll-state";
export const SCROLL_STORAGE_RELOAD_KEY = "vorma-scroll-state-reload";
export const VORMA_ROOT_EL_ID = "vorma-root";

export const HISTORY_KEY_FIELD = "vorma-history-key";
export const HISTORY_USER_STATE_FIELD = "vorma-user-state";

export const API_IDENTITY_ARRAY_PREFIX = "vorma-api";

export const LINK_ACTIVE_EXACT_ATTR = "data-vorma-active-exact";
export const LINK_ACTIVE_ANCESTOR_ATTR = "data-vorma-active-ancestor";
export const LINK_PENDING_EXACT_ATTR = "data-vorma-pending-exact";
export const LINK_PENDING_ANCESTOR_ATTR = "data-vorma-pending-ancestor";

export const CSS_BUNDLE_ATTR = "data-vorma-css-bundle";
export const CSS_PRELOAD_ATTR = "data-vorma-css-preload";
export const CSS_PRELOAD_SETTLED_ATTR = "data-vorma-css-settled";
