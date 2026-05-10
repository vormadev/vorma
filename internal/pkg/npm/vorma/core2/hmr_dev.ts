import type { ClientLoaderFn, ViewDefinition } from "./client_contract.ts";
import type {
	BrowserListenerHandlers,
	HMRRoutePrepareRequest,
} from "./host.ts";
import type { PreparedHMRRoute } from "./model.ts";

const hmr_client_loader_trigger = "revalidation";

type HMRWindow = Window & {
	__vorma_hmr_route_update?: (
		raw_url: string,
		mod: Record<string, unknown>,
	) => Promise<void> | void;
};

export function install_hmr_route_update_handler(input: {
	handlers: BrowserListenerHandlers;
	module_cache: Map<string, Record<string, unknown>>;
	now_ms: () => number;
}): () => void {
	const hmr_window = window as HMRWindow;
	const on_hmr_update = async (
		raw_url: string,
		mod: Record<string, unknown>,
	): Promise<void> => {
		const module_url = normalize_module_url(raw_url);
		input.module_cache.set(module_url, mod);
		input.handlers.on_hmr_update({
			module: mod,
			module_url,
			now_ms: input.now_ms(),
		});
	};
	hmr_window.__vorma_hmr_route_update = on_hmr_update;
	return () => {
		if (hmr_window.__vorma_hmr_route_update === on_hmr_update) {
			hmr_window.__vorma_hmr_route_update = undefined;
		}
	};
}

export async function prepare_hmr_route_dev(
	request: HMRRoutePrepareRequest,
): Promise<PreparedHMRRoute> {
	const idx = request.render.entries.findIndex((entry) => {
		return entry.module_url === request.module_url;
	});
	if (idx === -1) {
		return {
			render: request.render,
			route: request.route,
		};
	}
	const entry = request.render.entries[idx]!;
	let client_loader_data = entry.client_loader_data;
	if (request.rerun_client_loader) {
		const def = request.module.default as ViewDefinition | undefined;
		const loader = def?.client_loader;
		if (loader) {
			try {
				client_loader_data = await run_hmr_client_loader(
					loader,
					request,
					idx,
					entry.input,
					entry.pattern,
				);
			} catch {}
		}
	}
	const entries = request.render.entries.map((candidate, candidate_idx) => {
		if (candidate_idx !== idx) {
			return candidate;
		}
		return {
			...candidate,
			client_loader_data,
			module: request.module,
		};
	});
	const matches = request.route.matches.map((match, match_idx) => {
		if (match_idx !== idx) {
			return match;
		}
		return {
			...match,
			clientLoaderData: client_loader_data,
		};
	});
	return {
		render: {
			...request.render,
			entries,
		},
		route: {
			...request.route,
			matches,
		},
	};
}

function run_hmr_client_loader(
	loader: ClientLoaderFn,
	request: HMRRoutePrepareRequest,
	idx: number,
	input: unknown,
	pattern: string,
): Promise<unknown> {
	const known_matches = request.route.matches.map((match) => {
		return { pattern: match.pattern, input: match.input };
	});
	return loader({
		current: request.route,
		href: request.position.href,
		historyState: request.position.state,
		input,
		knownMatches: known_matches,
		params: request.route.params,
		pattern,
		serverPromise: Promise.resolve({
			clientBuildID: request.route.clientBuildID,
			matches: request.route.matches.map((match) => {
				return {
					pattern: match.pattern,
					input: match.input,
					loaderData: match.loaderData,
				};
			}),
			outermostServerError:
				request.route.error?.source === "server" &&
				request.route.error.idx === idx
					? request.route.error.error
					: null,
			loaderData: request.route.matches[idx]?.loaderData,
		}),
		signal: request.signal,
		splatValues: request.route.splatValues,
		trigger: hmr_client_loader_trigger,
	});
}

function normalize_module_url(raw_url: string): string {
	try {
		const url = new URL(raw_url, window.location.href);
		return `${url.pathname}${url.search}${url.hash}`;
	} catch {
		return raw_url;
	}
}
