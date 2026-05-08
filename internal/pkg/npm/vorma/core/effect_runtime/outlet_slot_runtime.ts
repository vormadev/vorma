import { Effect } from "effect";
import type {
	RouteRenderEntry,
	ViewDefinition,
} from "../create_client_core.ts";
import type { RouteErrorState } from "../types.ts";

export type OutletSlot =
	| { kind: "component"; component: (props: any) => any }
	| {
			kind: "error";
			error: unknown;
			boundary: (props: { error: unknown }) => any;
	  }
	| { kind: "pass_through" }
	| { kind: "empty" };

type StableWrapperEntry<Props> = {
	holder: { current: (props: Props) => any };
	wrapper: (props: Props) => any;
};

type StableWrapperCache<Props> = Map<string, StableWrapperEntry<Props>>;

export type OutletSlotRuntime = {
	resolve_outlet_slot: (
		entries: RouteRenderEntry[],
		error: RouteErrorState | null,
		idx: number,
		default_error_boundary:
			| ((props: { error: unknown }) => any)
			| undefined,
	) => OutletSlot;
};

function get_stable_wrapper<Props>(
	cache: StableWrapperCache<Props>,
	key: string,
	impl: (props: Props) => any,
): (props: Props) => any {
	let entry = cache.get(key);
	if (!entry) {
		const holder = { current: impl };
		const wrapper = (props: Props) => {
			return holder.current(props);
		};
		entry = { wrapper, holder };
		cache.set(key, entry);
	}
	entry.holder.current = impl;
	return entry.wrapper;
}

export function make_outlet_slot_runtime(): Effect.Effect<
	OutletSlotRuntime,
	never
> {
	return Effect.sync(() => {
		const component_refs: StableWrapperCache<any> = new Map();
		const error_refs: StableWrapperCache<{ error: unknown }> = new Map();

		return {
			resolve_outlet_slot: (
				entries,
				error,
				idx,
				default_error_boundary,
			) => {
				if (idx >= entries.length) {
					return { kind: "empty" };
				}

				if (error !== null && idx >= error.idx) {
					const error_entry = entries[error.idx]!;
					const def = error_entry.module.default as
						| ViewDefinition
						| undefined;
					const raw_boundary =
						def?.error_boundary ??
						default_error_boundary ??
						fallback_error_boundary;
					const boundary = get_stable_wrapper(
						error_refs,
						error_entry.pattern,
						raw_boundary,
					);
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

				const component = get_stable_wrapper(
					component_refs,
					entry.pattern,
					def.component,
				);
				return { kind: "component", component };
			},
		};
	});
}

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
