import {
	type Core6Scope,
	type Core6ScopeCommitResult,
	type Core6ScopeManager,
	create_core6_scope_manager,
	run_core6_scope_stage,
} from "./scope.ts";

export const core6_route_transaction_kind = {
	boot: "boot",
	navigation: "navigation",
	popstate: "popstate",
	prefetch: "prefetch",
	revalidation: "revalidation",
} as const;

export type Core6RouteTransactionKind =
	(typeof core6_route_transaction_kind)[keyof typeof core6_route_transaction_kind];

export type Core6RouteTransactionSpec<Kind extends string, Intent> = {
	kind: Kind;
	intent: Intent;
};

export type Core6RouteTransactionStageArgs<Intent> = {
	intent: Intent;
	signal: AbortSignal;
};

export type Core6RouteTransaction<Kind extends string, Intent> = {
	readonly id: Core6Scope<Kind>["id"];
	readonly intent: Intent;
	readonly kind: Kind;
	readonly signal: AbortSignal;
	cancel: () => boolean;
	commit: <T>(fn: (intent: Intent) => T) => Core6ScopeCommitResult<T>;
	complete: <T>(fn: (intent: Intent) => T) => Core6ScopeCommitResult<T>;
	live: () => boolean;
	on_cancel: (fn: () => void) => () => void;
	on_cleanup: (fn: () => void) => () => void;
	stage: <T>(
		fn: (args: Core6RouteTransactionStageArgs<Intent>) => Promise<T>,
	) => Promise<Core6ScopeCommitResult<T>>;
};

export type Core6RouteTransactionManager<Kind extends string, Intent> = {
	cancel_current: () => boolean;
	current: () => Core6RouteTransaction<Kind, Intent> | null;
	is_current: (transaction: Core6RouteTransaction<Kind, Intent>) => boolean;
	start: (
		spec: Core6RouteTransactionSpec<Kind, Intent>,
	) => Core6RouteTransaction<Kind, Intent>;
};

type TransactionRecord<Kind extends string, Intent> = {
	intent: Intent;
	scope: Core6Scope<Kind>;
	transaction: Core6RouteTransaction<Kind, Intent>;
};

export function create_core6_route_transaction_manager<
	Kind extends string = Core6RouteTransactionKind,
	Intent = unknown,
>(
	scope_manager: Core6ScopeManager<Kind> = create_core6_scope_manager<Kind>(),
): Core6RouteTransactionManager<Kind, Intent> {
	let current_record: TransactionRecord<Kind, Intent> | null = null;

	function make_transaction(
		record: TransactionRecord<Kind, Intent>,
	): Core6RouteTransaction<Kind, Intent> {
		const scope = record.scope;
		return {
			id: scope.id,
			intent: record.intent,
			kind: scope.kind,
			signal: scope.signal,
			cancel: () => {
				const cancelled = scope.cancel();
				if (cancelled && current_record === record) {
					current_record = null;
				}
				return cancelled;
			},
			commit: (fn) => {
				return scope.commit(() => {
					return fn(record.intent);
				});
			},
			complete: (fn) => {
				try {
					return scope.complete(() => {
						return fn(record.intent);
					});
				} finally {
					if (current_record === record && !scope.live()) {
						current_record = null;
					}
				}
			},
			live: () => {
				return scope.live();
			},
			on_cancel: (fn) => {
				return scope.on_cancel(fn);
			},
			on_cleanup: (fn) => {
				return scope.on_cleanup(fn);
			},
			stage: (fn) => {
				return run_core6_scope_stage(scope, async (signal) => {
					return await fn({ intent: record.intent, signal });
				});
			},
		};
	}

	function start(
		spec: Core6RouteTransactionSpec<Kind, Intent>,
	): Core6RouteTransaction<Kind, Intent> {
		const scope = scope_manager.start(spec.kind);
		const record: TransactionRecord<Kind, Intent> = {
			intent: spec.intent,
			scope,
			transaction: null as unknown as Core6RouteTransaction<Kind, Intent>,
		};
		record.transaction = make_transaction(record);
		current_record = record;
		return record.transaction;
	}

	return {
		cancel_current: () => {
			const cancelled = scope_manager.cancel_current();
			if (cancelled) {
				current_record = null;
			}
			return cancelled;
		},
		current: () => {
			const scope = scope_manager.current();
			if (!scope || current_record?.scope !== scope) {
				return null;
			}
			return current_record.transaction;
		},
		is_current: (transaction) => {
			return (
				current_record?.transaction === transaction &&
				transaction.live()
			);
		},
		start,
	};
}
