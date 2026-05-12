import type { Core6ScopeCommitResult } from "./scope.ts";
import type { Core6RouteTransaction } from "./transaction.ts";

export type Core6RoutePipelineFetchArgs<Intent> = {
	intent: Intent;
	signal: AbortSignal;
};

export type Core6RoutePipelinePrepareArgs<Intent, Fetched> = {
	fetched: Fetched;
	intent: Intent;
	signal: AbortSignal;
};

export type Core6RoutePipelinePublishArgs<Intent, Prepared> = {
	intent: Intent;
	prepared: Prepared;
};

export type Core6RoutePipeline<Intent, Fetched, Prepared, Published> = {
	fetch: (args: Core6RoutePipelineFetchArgs<Intent>) => Promise<Fetched>;
	prepare: (
		args: Core6RoutePipelinePrepareArgs<Intent, Fetched>,
	) => Promise<Prepared>;
	publish: (
		args: Core6RoutePipelinePublishArgs<Intent, Prepared>,
	) => Published;
};

export async function run_core6_route_pipeline<
	Kind extends string,
	Intent,
	Fetched,
	Prepared,
	Published,
>(
	transaction: Core6RouteTransaction<Kind, Intent>,
	pipeline: Core6RoutePipeline<Intent, Fetched, Prepared, Published>,
): Promise<Core6ScopeCommitResult<Published>> {
	try {
		const fetched = await transaction.stage(async ({ intent, signal }) => {
			return await pipeline.fetch({ intent, signal });
		});
		if (!fetched.ok) {
			return fetched;
		}

		const prepared = await transaction.stage(async ({ intent, signal }) => {
			return await pipeline.prepare({
				fetched: fetched.value,
				intent,
				signal,
			});
		});
		if (!prepared.ok) {
			return prepared;
		}

		return transaction.complete((intent) => {
			return pipeline.publish({
				intent,
				prepared: prepared.value,
			});
		});
	} catch (error) {
		transaction.cancel();
		throw error;
	}
}
