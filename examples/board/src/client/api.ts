import {
	type UseMutationOptions,
	queryOptions,
	useMutation,
	useQuery,
} from "@tanstack/react-query";
import type {
	ApiClientOutput,
	ToMutationArgs,
	ToMutationError,
	ToMutationMethod,
	ToMutationPattern,
	ToQueryArgs,
	ToQueryError,
} from "vorma/react";
import { apiClient } from "./app.tsx";
import type { vormaClientSeed } from "./vorma.gen.ts";

/*
These wrappers are ordinary app code on top of Vorma's generated
`apiClient`. Vorma owns typed request/response conversion; React Query
owns caching, pending state, retries, and mutation lifecycle callbacks.

Queries receive their full arguments at hook creation because the
arguments are the cache identity. `apiClient.toIdentityArray()` gives the
same stable identity Vorma uses internally, so the React Query key stays
aligned with the generated API contract.

Mutations receive endpoint identity at hook creation and variables at
`mutate()` time. That lets one mutation hook serve an entire list of
stories while each click supplies a different route param or input body.

Do not manually revalidate route data after normal mutations. Vorma
mutations revalidate the active route by default; opt out at the call
site only for deliberate diagnostics or fire-and-forget actions.
*/

type BoardClientSeed = typeof vormaClientSeed;

/*
Exported separately from `useApiQuery` so call sites that are not React
components can still build the exact same react-query options object.
`query_client.prefetchQuery(apiQueryOptions(args))` or
`query_client.ensureQueryData(apiQueryOptions(args))` warms react-query's
cache for a specific typed call ahead of the component that will read it
with `useApiQuery` — the react-query-owned counterpart to Vorma's own
`prefetch()`, which warms route/view data instead. Both forms of prefetch
are legitimate and answer different questions: `prefetch()` asks "does the
next route already have its server-rendered data," `apiQueryOptions` asks
"does this specific ad-hoc API call already have a cached result."
*/
export function apiQueryOptions<const Args extends ToQueryArgs<BoardClientSeed>>(
	args: Args,
) {
	return queryOptions<
		ApiClientOutput<BoardClientSeed, Args>,
		ToQueryError<BoardClientSeed, Args>,
		ApiClientOutput<BoardClientSeed, Args>,
		unknown[]
	>({
		queryKey: apiClient.toIdentityArray(args),
		queryFn: ({ signal }) => {
			return apiClient.queryOrThrow({ ...args, signal } as Args);
		},
	});
}

export function useApiQuery<const Args extends ToQueryArgs<BoardClientSeed>>(args: Args) {
	return useQuery(apiQueryOptions(args));
}

export function useApiMutation<
	const Method extends ToMutationMethod<BoardClientSeed>,
	const Pattern extends ToMutationPattern<BoardClientSeed, Method>,
>(
	route: { method: Method; pattern: Pattern },
	options?: Omit<
		UseMutationOptions<
			ApiClientOutput<
				BoardClientSeed,
				ToMutationArgs<BoardClientSeed, Method, Pattern>
			>,
			ToMutationError<
				BoardClientSeed,
				ToMutationArgs<BoardClientSeed, Method, Pattern>
			>,
			Omit<ToMutationArgs<BoardClientSeed, Method, Pattern>, "method" | "pattern">
		>,
		"mutationFn"
	>,
) {
	type Args = ToMutationArgs<BoardClientSeed, Method, Pattern>;
	return useMutation({
		...options,
		mutationFn: (vars: Omit<Args, "method" | "pattern">) => {
			return apiClient.mutateOrThrow({ ...route, ...vars } as Args);
		},
	});
}
