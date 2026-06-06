import type { RouteRenderEntry, ViewDefinition } from "./create_client_core.ts";
import type { RouteErrorState } from "./types.ts";

export type OutletSlot =
	| { kind: "component"; component: (props: any) => any }
	| {
			kind: "error";
			error: unknown;
			boundary: (props: { error: unknown }) => any;
	  }
	| { kind: "pass_through" }
	| { kind: "empty" };

const component_refs = new Map<
	string,
	{
		wrapper: (props: any) => any;
		holder: { current: (props: any) => any };
	}
>();

const error_refs = new Map<
	string,
	{
		wrapper: (props: { error: unknown }) => any;
		holder: { current: (props: { error: unknown }) => any };
	}
>();

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

function get_stable_component(
	key: string,
	impl: (props: any) => any,
): (props: any) => any {
	return get_stable_fn(component_refs, key, impl);
}

function get_stable_error_boundary(
	key: string,
	impl: (props: { error: unknown }) => any,
): (props: { error: unknown }) => any {
	return get_stable_fn(error_refs, key, impl);
}

export function resolve_outlet_slot(
	entries: RouteRenderEntry[],
	error: RouteErrorState | null,
	idx: number,
	default_error_boundary: ((props: { error: unknown }) => any) | undefined,
): OutletSlot {
	if (idx >= entries.length) {
		return { kind: "empty" };
	}

	if (error !== null && idx >= error.idx) {
		const error_entry = entries[error.idx]!;
		const def = error_entry.module.default as ViewDefinition | undefined;
		const raw_boundary =
			def?.error_boundary ?? default_error_boundary ?? fallback_error_boundary;
		const boundary = get_stable_error_boundary(error_entry.pattern, raw_boundary);
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

	const component = get_stable_component(entry.pattern, def.component);
	return { kind: "component", component };
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
