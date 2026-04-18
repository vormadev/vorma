import type {
	AppConfig,
	MakeTypedActionSubmitOutput,
	MakeTypedActionSubmitProps,
	MakeTypedAPIClient,
	MakeTypedAPIDecorator,
	MakeTypedAPIDecoratorContext,
	SubmitOptions,
	SubmitResult,
} from "./types.ts";
import { build_action_url, resolve_body } from "./url.ts";

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
		submit: async <Props extends MakeTypedActionSubmitProps<A>>(
			props: Props,
		): Promise<SubmitResult<MakeTypedActionSubmitOutput<A, Props>>> => {
			const {
				dedupeKey,
				input,
				method: raw_method,
				params,
				pattern,
				revalidate,
				skipProgressIndicator,
				splatValues,
				...request_init
			} = props as any;
			const method = String(raw_method ?? "GET")
				.toUpperCase()
				.trim();
			const is_get = method === "GET" || method === "HEAD";
			const url = build_action_url(
				actions_mount_root,
				pattern,
				params,
				splatValues,
				is_get ? input : undefined,
			);
			const ctx = {
				input,
				method,
				pattern,
				requestInit: request_init,
			} as MakeTypedAPIDecoratorContext<A>;
			const decorated = decorator
				? ((await (decorator as any)(ctx)) ?? {})
				: {};
			const init: RequestInit = { ...decorated, ...request_init };
			const headers = new Headers(decorated.headers ?? undefined);
			new Headers(request_init.headers ?? undefined).forEach((v, k) => {
				headers.set(k, v);
			});
			init.headers = headers;
			init.method = method;
			if (is_get) {
				delete init.body;
			} else {
				init.body = resolve_body(input);
			}
			const options: SubmitOptions = {};
			if (dedupeKey !== undefined) {
				options.dedupeKey = dedupeKey;
			}
			if (revalidate !== undefined) {
				options.revalidate = revalidate;
			}
			if (skipProgressIndicator !== undefined) {
				options.skipProgressIndicator = skipProgressIndicator;
			}
			return submit_fn<MakeTypedActionSubmitOutput<A, Props>>(
				url,
				init,
				options,
			);
		},
	};
}
