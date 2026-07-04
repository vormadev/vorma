import type { RouteRenderEntry, ViewDefinition } from "./create_client_core.ts";
import type { RouteErrorState } from "./types.ts";

type ComponentFn = (props: any) => any;
type ErrorBoundaryFn = (props: { error: unknown }) => any;

/**
 * What `RootOutlet` renders at a given depth, as decided by
 * `resolve_outlet_slot` — adapter-internal (each `RootOutlet`
 * implementation reads this to pick its render branch); an app never
 * constructs or reads one directly.
 *
 * - `"component"`: the matched view's own component.
 * - `"error"`: an error boundary, for a depth at or past where the current
 *   route's error originated.
 * - `"pass_through"`: no component defined at this depth (a route-less
 *   layout segment) — render the next-deeper outlet directly.
 * - `"empty"`: nothing more to render (past the leaf of the matched
 *   chain, or a plain layout with no children matched).
 */
export type OutletSlot =
	| { kind: "component"; component: ComponentFn }
	| {
			kind: "error";
			error: unknown;
			boundary: ErrorBoundaryFn;
	  }
	| { kind: "pass_through" }
	| { kind: "empty" };

export type OutletSlotResolver = (
	entries: RouteRenderEntry[],
	error: RouteErrorState | null,
	idx: number,
	default_error_boundary: ErrorBoundaryFn | undefined,
) => OutletSlot;

type StableFnEntry<F extends (props: any) => any> = {
	wrapper: F;
	holder: { current: F };
};

function get_stable_fn<F extends (props: any) => any>(
	refs: Map<string, StableFnEntry<F>>,
	key: string,
	impl: F,
): F {
	let entry = refs.get(key);
	if (!entry) {
		const holder = { current: impl };
		const wrapper = ((props: Parameters<F>[0]) => {
			return holder.current(props);
		}) as F;
		entry = { wrapper, holder };
		refs.set(key, entry);
	}
	entry.holder.current = impl;
	return entry.wrapper;
}

export function create_outlet_slot_resolver(): OutletSlotResolver {
	const component_refs = new Map<string, StableFnEntry<ComponentFn>>();
	const error_refs = new Map<string, StableFnEntry<ErrorBoundaryFn>>();

	return (
		entries: RouteRenderEntry[],
		error: RouteErrorState | null,
		idx: number,
		default_error_boundary: ErrorBoundaryFn | undefined,
	): OutletSlot => {
		if (idx >= entries.length) {
			return { kind: "empty" };
		}

		if (error !== null && idx >= error.idx) {
			const error_entry = entries[error.idx]!;
			const def = error_entry.module.default as ViewDefinition | undefined;
			const raw_boundary =
				def?.error_boundary ?? default_error_boundary ?? fallback_error_boundary;
			const boundary = get_stable_fn(error_refs, error_entry.pattern, raw_boundary);
			return { kind: "error", error: error.error, boundary };
		}

		const entry = entries[idx]!;
		const def = entry.module.default as ViewDefinition | undefined;

		if (!def?.component) {
			if (idx + 1 < entries.length) {
				return { kind: "pass_through" };
			}
			return { kind: "empty" };
		}

		const component = get_stable_fn(component_refs, entry.pattern, def.component);
		return { kind: "component", component };
	};
}

/**
 * A stable React/Solid/Preact list key for one matched route entry —
 * pattern plus module URL, so HMR module swaps and pattern changes both
 * produce a genuinely new key (forcing a clean remount) while an unchanged
 * entry keeps its identity across re-renders.
 */
export function get_entry_key(entry: RouteRenderEntry): string {
	return `${entry.pattern}::${entry.module_url}`;
}

function fallback_error_boundary(props: { error: unknown }): string {
	if (props.error instanceof Error) {
		return `Error: ${props.error.message}`;
	}
	if (typeof props.error === "string") {
		return `Error: ${props.error}`;
	}
	return "An unexpected error occurred.";
}
