/// <reference types="vite/client" />

import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import { registerPattern } from "vorma/kit/matcher/register";
import { is_abort_error } from "./abort.ts";
import { get_global } from "./global_state.ts";
import type {
	ClientLoaderWaitFn,
	Nullable,
	RuntimeRouteSnapshot,
	VormaTypedAdapterAddClientLoaderProps,
} from "./types.ts";

// ─── Unhandled Rejection Guard ───────────────────────────────────

const guarded = new WeakSet<Promise<unknown>>();
function suppress_unhandled(p: Promise<unknown>): void {
	if (guarded.has(p)) {
		return;
	}
	guarded.add(p);
	void p.catch(() => undefined);
}

// ─── Client Loader Prestart ─────────────────────────────────────

export type ClientLoaderPrestart = {
	matched_pattern: string;
	result_promise: Promise<unknown>;
	resolve_from_snapshot: (s: RuntimeRouteSnapshot) => void;
	abort_if_pending: () => void;
};

export function create_prestarts(props: {
	target_url: string;
	signal: AbortSignal;
}): ClientLoaderPrestart[] {
	const g = get_global();
	const pathname = new URL(props.target_url, window.location.href).pathname;
	const match_result = findNestedMatches(g.pattern_registry, pathname);
	if (!match_result) {
		return [];
	}

	const results: ClientLoaderPrestart[] = [];

	for (const match of match_result.matches) {
		const pattern = match.registeredPattern.originalPattern;
		const wait_fn = g.pattern_to_wait_fn[pattern];
		if (!wait_fn) {
			continue;
		}

		let settled = false;
		let resolve_server: (v: any) => void;
		let reject_server: (e: unknown) => void;
		const server_promise = new Promise<any>((res, rej) => {
			resolve_server = res;
			reject_server = rej;
		});
		suppress_unhandled(server_promise);

		const result_promise = Promise.resolve(
			wait_fn({
				params: match_result.params,
				splatValues: match_result.splatValues,
				serverDataPromise: server_promise,
				signal: props.signal,
			}),
		);
		suppress_unhandled(result_promise);

		const abort_if_pending = () => {
			if (settled) {
				return;
			}
			settled = true;
			reject_server!(
				Object.assign(
					new Error("Client loader serverDataPromise aborted."),
					{ name: "AbortError" },
				),
			);
		};
		props.signal.addEventListener("abort", abort_if_pending, {
			once: true,
		});

		results.push({
			matched_pattern: pattern,
			result_promise,
			resolve_from_snapshot: (s) => {
				if (settled) {
					return;
				}
				settled = true;
				const idx = s.matched_patterns.indexOf(pattern);
				resolve_server!({
					matchedPatterns: s.matched_patterns,
					rootData: s.has_root_data ? s.loaders_data[0] : null,
					loaderData: s.loaders_data[idx],
					clientBuildID: s.client_build_id,
				});
			},
			abort_if_pending,
		});
	}

	return results;
}

// ─── Complete Client Loaders ─────────────────────────────────────

export type ClientLoaderResult = {
	client_loaders_data: unknown[];
	outermost_client_error: unknown;
	outermost_client_error_idx: Nullable<number>;
};

export async function complete_client_loaders(props: {
	snapshot: RuntimeRouteSnapshot;
	signal: AbortSignal;
	prestarted?: Record<string, Promise<unknown>>;
}): Promise<ClientLoaderResult> {
	const wait_fns = get_global().pattern_to_wait_fn;
	const prestarted = props.prestarted ?? {};
	const s = props.snapshot;
	const n = s.matched_patterns.length;

	const promises: Promise<unknown>[] = [];
	const controllers: (AbortController | null)[] = [];

	for (let i = 0; i < n; i++) {
		const pattern = s.matched_patterns[i] as string;

		// skip client loaders at or beyond the server error
		// boundary — the server already errored at this index, so
		// running the client loader is both incorrect and wasteful.
		if (
			s.outermost_server_error_idx != null &&
			i >= s.outermost_server_error_idx
		) {
			promises.push(Promise.resolve(undefined));
			controllers.push(null);
			continue;
		}

		if (prestarted[pattern] !== undefined) {
			// Already running from prefetch or parallel start
			promises.push(prestarted[pattern]!);
			controllers.push(null);
			continue;
		}

		const wait_fn = wait_fns[pattern];
		if (!wait_fn) {
			promises.push(Promise.resolve(undefined));
			controllers.push(null);
			continue;
		}

		const controller = new AbortController();
		controllers.push(controller);

		// Wire up the parent navigation signal to this loader's controller
		if (props.signal.aborted) {
			controller.abort();
		} else {
			props.signal.addEventListener("abort", () => controller.abort(), {
				once: true,
			});
		}

		const server_data = Promise.resolve({
			matchedPatterns: s.matched_patterns,
			rootData: s.has_root_data ? s.loaders_data[0] : null,
			loaderData: s.loaders_data[i],
			clientBuildID: s.client_build_id,
		});

		promises.push(
			wait_fn({
				params: s.params,
				splatValues: s.splat_values,
				serverDataPromise: server_data,
				signal: controller.signal,
			}),
		);
	}

	// Wrap each promise so that a non-abort failure in one loader
	// immediately aborts all downstream (child) loaders.
	const wrapped = promises.map((promise, index) =>
		promise.catch((error: unknown) => {
			if (!is_abort_error(error)) {
				for (let j = index + 1; j < controllers.length; j++) {
					controllers[j]?.abort();
				}
			}
			throw error;
		}),
	);

	const results = await Promise.allSettled(wrapped);

	const data: unknown[] = [];
	let outermost_error: unknown;
	let outermost_idx: Nullable<number>;

	for (let i = 0; i < results.length; i++) {
		const result = results[i]!;
		if (result.status === "fulfilled") {
			data.push(result.value);
		} else {
			if (!is_abort_error(result.reason) && outermost_idx == null) {
				outermost_error =
					result.reason instanceof Error
						? result.reason.message
						: String(result.reason);
				outermost_idx = i;
			}
			data.push(undefined);
			// Stop at the first real error
			if (outermost_idx != null) {
				break;
			}
		}
	}

	// Fill remaining slots if we broke out early
	while (data.length < n) {
		data.push(undefined);
	}

	return {
		client_loaders_data: data,
		outermost_client_error: outermost_error,
		outermost_client_error_idx: outermost_idx,
	};
}

// ─── Register Client Loader (adapter API) ────────────────────────

export function register_client_loader(
	props: VormaTypedAdapterAddClientLoaderProps<any, any, any, any>,
): void {
	const g = get_global();
	g.pattern_to_wait_fn[props.pattern] =
		props.clientLoader as ClientLoaderWaitFn;
	registerPattern(g.pattern_registry, props.pattern);
	if (import.meta.env.DEV && props.reRunOnModuleChange) {
		void import("./_hmr_dev.ts")
			.then(({ register_for_hmr_rerun }) => {
				register_for_hmr_rerun({
					import_meta: props.reRunOnModuleChange!,
					pattern: props.pattern,
				});
			})
			.catch(() => undefined);
	}
}
