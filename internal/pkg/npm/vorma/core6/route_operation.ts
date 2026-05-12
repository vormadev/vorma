import { run_core6_route_pipeline } from "./pipeline.ts";
import type { Core6ScopeCommitResult } from "./scope.ts";
import type {
	Core6RouteTransactionKind,
	Core6RouteTransactionManager,
} from "./transaction.ts";

export type Core6RouteOperationFetchArgs<Intent> = {
	intent: Intent;
	signal: AbortSignal;
};

export type Core6RouteOperationPrepareArgs<Intent, Fetched> = {
	fetched: Fetched;
	intent: Intent;
	signal: AbortSignal;
};

export type Core6RouteOperationPublishArgs<Intent, Prepared> = {
	intent: Intent;
	prepared: Prepared;
};

export type Core6RouteOperationHost<Intent, Fetched, Prepared, Published> = {
	fetch_route: (
		args: Core6RouteOperationFetchArgs<Intent>,
	) => Promise<Fetched>;
	prepare_route: (
		args: Core6RouteOperationPrepareArgs<Intent, Fetched>,
	) => Promise<Prepared>;
	publish_route: (
		args: Core6RouteOperationPublishArgs<Intent, Prepared>,
	) => Published;
};

export type Core6RouteOperationInput<
	Kind extends Core6RouteTransactionKind,
	Intent,
	Fetched,
	Prepared,
	Published,
> = {
	host: Core6RouteOperationHost<Intent, Fetched, Prepared, Published>;
	intent: Intent;
	kind: Kind;
	transaction_manager: Core6RouteTransactionManager<Kind, Intent>;
};

export async function run_core6_route_operation<
	Kind extends Core6RouteTransactionKind,
	Intent,
	Fetched,
	Prepared,
	Published,
>(
	input: Core6RouteOperationInput<Kind, Intent, Fetched, Prepared, Published>,
): Promise<Core6ScopeCommitResult<Published>> {
	const transaction = input.transaction_manager.start({
		intent: input.intent,
		kind: input.kind,
	});
	return await run_core6_route_pipeline(transaction, {
		fetch: async ({ intent, signal }) => {
			return await input.host.fetch_route({ intent, signal });
		},
		prepare: async ({ fetched, intent, signal }) => {
			return await input.host.prepare_route({
				fetched,
				intent,
				signal,
			});
		},
		publish: ({ intent, prepared }) => {
			return input.host.publish_route({ intent, prepared });
		},
	});
}
