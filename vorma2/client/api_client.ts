/// <reference types="vite/client" />

import { submit } from "./public_api.ts";
import type {
	APIRequestInitDecorator,
	ExtractApp,
	SubmitResult,
	TypedAPIClient,
	VormaAppConfig,
	VormaMutationOutput,
	VormaMutationPattern,
	VormaMutationProps,
	VormaQueryOutput,
	VormaQueryPattern,
	VormaQueryProps,
} from "./types.ts";
import { build_mutation_url, build_query_url, resolve_body } from "./url.ts";

function merge_headers(base: RequestInit, override?: RequestInit): RequestInit {
	const m = new Headers(base.headers ?? undefined);
	new Headers(override?.headers ?? undefined).forEach((v, k) => m.set(k, v));
	return { ...base, ...override, headers: m };
}

export function makeTypedAPIClient<C extends VormaAppConfig>(
	cfg: C,
	decorator?: APIRequestInitDecorator<ExtractApp<C>>,
): TypedAPIClient<ExtractApp<C>> {
	type App = ExtractApp<C>;

	async function resolve_init(
		ctx: any,
		fallback: RequestInit,
	): Promise<RequestInit> {
		const decorated = decorator ? await decorator(ctx) : undefined;
		return merge_headers(
			merge_headers(fallback, decorated),
			ctx.requestInit,
		);
	}

	return {
		query: async <P extends VormaQueryPattern<App>>(
			qp: VormaQueryProps<App, P>,
		): Promise<SubmitResult<VormaQueryOutput<App, P>>> => {
			const a = qp as any;
			const init = await resolve_init(
				{
					type: "query",
					pattern: qp.pattern,
					requestInit: qp.requestInit,
					input: qp.input,
				},
				{ method: "GET" },
			);
			return submit<VormaQueryOutput<App, P>>(
				build_query_url(cfg, {
					pattern: qp.pattern,
					params: a.params,
					splat_values: a.splatValues,
					input: qp.input,
				}),
				init,
				qp.options,
			);
		},
		mutate: async <P extends VormaMutationPattern<App>>(
			mp: VormaMutationProps<App, P>,
		): Promise<SubmitResult<VormaMutationOutput<App, P>>> => {
			const a = mp as any;
			const init = await resolve_init(
				{
					type: "mutation",
					pattern: mp.pattern,
					requestInit: mp.requestInit,
					input: mp.input,
				},
				{
					method: "POST",
					body: resolve_body({ input: mp.input }),
				},
			);
			return submit<VormaMutationOutput<App, P>>(
				build_mutation_url(cfg, {
					pattern: mp.pattern,
					params: a.params,
					splat_values: a.splatValues,
				}),
				init,
				mp.options,
			);
		},
	};
}
