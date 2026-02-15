import { createSignal } from "solid-js";
import {
	buildMutationURL,
	buildQueryURL,
	makeTypedNavigate,
	resolveBody,
	submit,
	type SubmitOptions,
} from "vorma/client";
import { addThemeChangeListener, getTheme } from "vorma/kit/theme";
import {
	makeTypedAddClientLoader,
	makeTypedLink,
	makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData,
	makeTypedUseRouterData,
} from "vorma/solid";
import {
	vormaAppConfig,
	type MutationOutput,
	type MutationPattern,
	type MutationProps,
	type QueryOutput,
	type QueryPattern,
	type QueryProps,
	type RouteProps,
	type VormaApp,
} from "./vorma.gen/index.ts";

type APIRequestInitResolverInput = {
	type: "query" | "mutation";
	pattern: string;
	requestInit?: RequestInit;
	input?: unknown;
};

type APIRequestInitResolver = (
	input: APIRequestInitResolverInput,
) => RequestInit | undefined;

let apiRequestInitResolver: APIRequestInitResolver | undefined;

export function setAPIRequestInitResolver(resolver?: APIRequestInitResolver) {
	apiRequestInitResolver = resolver;
}

export type { RouteProps, SubmitOptions };

export const useRouterData = makeTypedUseRouterData<VormaApp>();
export const useLoaderData = makeTypedUseLoaderData<VormaApp>();
export const usePatternLoaderData = makeTypedUsePatternLoaderData<VormaApp>();
export const addClientLoader = makeTypedAddClientLoader<VormaApp>();
export const navigate = makeTypedNavigate(vormaAppConfig);
export const Link = makeTypedLink(vormaAppConfig, {
	prefetch: "intent",
});

const [theme, setThemeSignal] = createSignal(getTheme());
addThemeChangeListener((event) => setThemeSignal(event.detail.theme));
export { theme };

export const api = {
	query,
	mutate,
	buildQueryURL: <P extends QueryPattern>(props: QueryProps<P>) =>
		buildQueryURL(vormaAppConfig, props),
	buildMutationURL: <P extends MutationPattern>(props: MutationProps<P>) =>
		buildMutationURL(vormaAppConfig, props),
	submit,
	resolveBody,
};

function mergeRequestInitWithHeaders(props: {
	baseRequestInit: RequestInit;
	overrideRequestInit?: RequestInit;
}): RequestInit {
	const { baseRequestInit, overrideRequestInit } = props;
	if (!overrideRequestInit) {
		return baseRequestInit;
	}

	const mergedHeaders = new Headers(baseRequestInit.headers ?? undefined);
	const overrideHeaders = new Headers(
		overrideRequestInit.headers ?? undefined,
	);
	overrideHeaders.forEach((value, key) => {
		mergedHeaders.set(key, value);
	});

	return {
		...baseRequestInit,
		...overrideRequestInit,
		headers: mergedHeaders,
	};
}

function resolveRequestInit(
	input: APIRequestInitResolverInput,
	fallbackRequestInit: RequestInit,
): RequestInit {
	const resolverRequestInit = apiRequestInitResolver?.(input);
	const baseRequestInit = mergeRequestInitWithHeaders({
		baseRequestInit: fallbackRequestInit,
		overrideRequestInit: resolverRequestInit,
	});
	return mergeRequestInitWithHeaders({
		baseRequestInit,
		overrideRequestInit: input.requestInit,
	});
}

async function query<P extends QueryPattern>(props: QueryProps<P>) {
	const requestInit = resolveRequestInit(
		{
			type: "query",
			pattern: props.pattern,
			requestInit: props.requestInit,
			input: props.input,
		},
		{ method: "GET" },
	);

	return await submit<QueryOutput<P>>(
		buildQueryURL(vormaAppConfig, props),
		requestInit,
		props.options,
	);
}

async function mutate<P extends MutationPattern>(props: MutationProps<P>) {
	const requestInit = resolveRequestInit(
		{
			type: "mutation",
			pattern: props.pattern,
			requestInit: props.requestInit,
			input: props.input,
		},
		{
			method: "POST",
			body: resolveBody(props),
		},
	);

	return await submit<MutationOutput<P>>(
		buildMutationURL(vormaAppConfig, props),
		requestInit,
		props.options,
	);
}
