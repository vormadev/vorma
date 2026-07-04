/**
 * The router's read model for in-flight activity — what `useWorkState()`
 * (and the imperative `getWorkState()`) return. This is what drives
 * `workIndicator` internally and what a custom loading UI reads directly
 * when the nprogress-style bar is not the right shape.
 *
 * Each field is `null`/empty when nothing of that kind is in flight:
 *
 * - `navigation`: the one in-flight navigation, if any (`source`
 *   distinguishes a programmatic `navigate()`/`Link` click from a
 *   `popstate` back/forward from a soft redirect the server issued).
 * - `revalidation`: an in-flight or scheduled route-data refresh —
 *   `"debouncing"` (waiting out the short window before firing),
 *   `"running"` (fetch in flight), or `"retrying"` (a previous attempt
 *   failed and a backoff-delayed retry is scheduled; `attempt` counts from
 *   1).
 * - `prefetch`: the at-most-one in-flight prefetch (Vorma coalesces to a
 *   single prefetch at a time; a new one cancels the previous).
 * - `apiRequests`: every concurrent `apiClient` call still in flight,
 *   independent of navigation — this is the array a react-query wrapper's
 *   mutations/queries show up in alongside Vorma's own route fetches, since
 *   {@link ToApiClient} calls are included here no matter who orchestrates
 *   them.
 *
 * Vorma calls (navigation, revalidation, `apiRequests`) count toward
 * `workIndicator` by default; `WorkIndicatorOptions`'s
 * `skipNavigations`/`skipApiRequests`/`skipRevalidations` exclude whole
 * categories, and a per-call `skipWorkIndicator` excludes one call.
 */
export type WorkState = {
	navigation: null | {
		href: string;
		replace: boolean;
		source: "navigate" | "popstate" | "redirect";
	};
	revalidation: null | {
		status: "debouncing" | "running" | "retrying";
		attempt: number;
	};
	prefetch: null | {
		href: string;
	};
	apiRequests: Array<{
		key: string;
		method: string;
		href: string;
	}>;
};

/** An empty {@link WorkState} — nothing in flight. Used as the pre-boot default. */
export function create_empty_work_state(): WorkState {
	return {
		navigation: null,
		revalidation: null,
		prefetch: null,
		apiRequests: [],
	};
}
