import type {
	AppConfig,
	MakeTypedAPIClient,
	MakeTypedAPIDecorator,
	MakeTypedAPIDecoratorContext,
	MakeTypedMutationO,
	MakeTypedMutationPattern,
	MakeTypedMutationProps,
	MakeTypedQueryO,
	MakeTypedQueryPattern,
	MakeTypedQueryProps,
	SubmitOptions,
	SubmitResult,
} from "./types.ts";
import { build_mutation_url, build_query_url, resolve_body } from "./url.ts";

type SubmitFn = <T>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
) => Promise<SubmitResult<T>>;

export function create_typed_api_client<A extends AppConfig>(
	actions_mount_root: string,
	submit_fn: SubmitFn,
	decorator?: MakeTypedAPIDecorator<A>,
): MakeTypedAPIClient<A> {
	return {
		query: async <P extends MakeTypedQueryPattern<A>>(
			props: MakeTypedQueryProps<A, P>,
		): Promise<SubmitResult<MakeTypedQueryO<A, P>>> => {
			const a = props as any;
			const url = build_query_url(
				actions_mount_root,
				a.pattern,
				a.params,
				a.splatValues,
				a.input,
			);
			const init = await resolve_init(
				decorator,
				{
					type: "query",
					pattern: a.pattern,
					requestInit: a.requestInit,
					input: a.input,
				},
				{ method: "GET" },
			);
			return submit_fn<MakeTypedQueryO<A, P>>(url, init, a.options);
		},
		mutate: async <P extends MakeTypedMutationPattern<A>>(
			props: MakeTypedMutationProps<A, P>,
		): Promise<SubmitResult<MakeTypedMutationO<A, P>>> => {
			const a = props as any;
			const url = build_mutation_url(
				actions_mount_root,
				a.pattern,
				a.params,
				a.splatValues,
			);
			const init = await resolve_init(
				decorator,
				{
					type: "mutation",
					pattern: a.pattern,
					requestInit: a.requestInit,
					input: a.input,
				},
				{
					method: a.requestInit?.method ?? "POST",
					body: resolve_body(a.input),
				},
			);
			return submit_fn<MakeTypedMutationO<A, P>>(url, init, a.options);
		},
	};
}

async function resolve_init<A extends AppConfig>(
	decorator: MakeTypedAPIDecorator<A> | undefined,
	ctx: MakeTypedAPIDecoratorContext<A> & { requestInit?: RequestInit },
	fallback: RequestInit,
): Promise<RequestInit> {
	const decorated = decorator ? await (decorator as any)(ctx) : undefined;
	return merge_headers(merge_headers(fallback, decorated), ctx.requestInit);
}

function merge_headers(
	base: RequestInit,
	override?: RequestInit | Record<string, unknown>,
): RequestInit {
	if (!override) {
		return base;
	}
	const merged = new Headers(base.headers ?? undefined);
	new Headers((override as RequestInit).headers ?? undefined).forEach(
		(v, k) => {
			merged.set(k, v);
		},
	);
	return { ...base, ...override, headers: merged };
}
