import { type UseMutationOptions, useMutation } from "@tanstack/react-query";
import type {
	ApiClientOutput,
	ToMutationArgs,
	ToMutationError,
	ToMutationMethod,
	ToMutationPattern,
} from "vorma/react";
import { apiClient } from "./app.tsx";
import type { vormaClientSeed } from "./vorma.gen.ts";

/*
react-query over the vorma api client. The contract (maintainer-
ratified): mutations take ENDPOINT IDENTITY at the hook — method is
always explicit for mutations — and mutate() takes whatever varies at
the call site (params/input), which is react-query's TVariables doing
its intended job. One hook serves a whole list. The query twin
(useApiQuery: full args at hook = the cache identity, queryKey via
apiClient.toIdentityArray) lands with the search feature.

No manual route revalidation anywhere: vorma mutations auto-revalidate
route data by default.
*/

type V = typeof vormaClientSeed;

export function useApiMutation<
	const M extends ToMutationMethod<V>,
	const P extends ToMutationPattern<V, M>,
>(
	route: { method: M; pattern: P },
	options?: Omit<
		UseMutationOptions<
			ApiClientOutput<V, ToMutationArgs<V, M, P>>,
			ToMutationError<V, ToMutationArgs<V, M, P>>,
			Omit<ToMutationArgs<V, M, P>, "method" | "pattern">
		>,
		"mutationFn"
	>,
) {
	type Args = ToMutationArgs<V, M, P>;
	return useMutation({
		...options,
		mutationFn: (vars: Omit<Args, "method" | "pattern">) => {
			return apiClient.mutateOrThrow({ ...route, ...vars } as Args);
		},
	});
}
