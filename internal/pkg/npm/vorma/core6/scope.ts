export const core6_scope_stale_reason = "stale";
export const core6_scope_cancelled_reason = "cancelled";

const core6_scope_completed_reason = "completed";
const scope_id_brand: unique symbol = Symbol("core6.scope_id");

export type Core6ScopeID = number & { readonly [scope_id_brand]: true };

export type Core6ScopeCommitResult<T> =
	| { ok: true; value: T }
	| { ok: false; reason: typeof core6_scope_stale_reason }
	| { ok: false; reason: typeof core6_scope_cancelled_reason };

export type Core6Scope<Kind extends string> = {
	readonly id: Core6ScopeID;
	readonly kind: Kind;
	readonly signal: AbortSignal;
	cancel: () => boolean;
	commit: <T>(fn: () => T) => Core6ScopeCommitResult<T>;
	complete: <T>(fn: () => T) => Core6ScopeCommitResult<T>;
	live: () => boolean;
	on_cancel: (fn: () => void) => () => void;
	on_cleanup: (fn: () => void) => () => void;
};

export type Core6ScopeManager<Kind extends string> = {
	cancel_current: () => boolean;
	current: () => Core6Scope<Kind> | null;
	is_current: (scope: Core6Scope<Kind>) => boolean;
	start: (kind: Kind) => Core6Scope<Kind>;
};

type ScopeRecord<Kind extends string> = {
	cancel_listeners: Set<() => void>;
	cleanups: Set<() => void>;
	controller: AbortController;
	id: Core6ScopeID;
	kind: Kind;
	scope: Core6Scope<Kind>;
	terminal_reason: ScopeTerminalReason | null;
};

type ScopeTerminalReason =
	| typeof core6_scope_cancelled_reason
	| typeof core6_scope_completed_reason
	| typeof core6_scope_stale_reason;

export function create_core6_scope_manager<Kind extends string>(
	abort_factory: () => AbortController = () => new AbortController(),
): Core6ScopeManager<Kind> {
	let next_id = 0;
	let current_record: ScopeRecord<Kind> | null = null;

	function run_callbacks(callbacks: Array<() => void>): unknown[] {
		const errors: unknown[] = [];
		for (const callback of callbacks) {
			try {
				callback();
			} catch (error) {
				errors.push(error);
			}
		}
		return errors;
	}

	function run_cleanups(record: ScopeRecord<Kind>): unknown[] {
		const cleanups = [...record.cleanups];
		record.cleanups.clear();
		return run_callbacks(cleanups);
	}

	function finish_record(
		record: ScopeRecord<Kind>,
		terminal_reason: ScopeTerminalReason,
	): boolean {
		if (current_record !== record) {
			return false;
		}
		current_record = null;
		record.terminal_reason = terminal_reason;
		const should_abort = terminal_reason !== core6_scope_completed_reason;
		if (should_abort && !record.controller.signal.aborted) {
			record.controller.abort();
		}
		const errors: unknown[] = [];
		if (should_abort) {
			const listeners = [...record.cancel_listeners];
			record.cancel_listeners.clear();
			errors.push(...run_callbacks(listeners));
		}
		errors.push(...run_cleanups(record));
		if (errors.length > 0) {
			throw errors[0];
		}
		return true;
	}

	function inactive_result<T>(
		record: ScopeRecord<Kind>,
	): Core6ScopeCommitResult<T> {
		if (record.terminal_reason === core6_scope_cancelled_reason) {
			return { ok: false, reason: core6_scope_cancelled_reason };
		}
		return { ok: false, reason: core6_scope_stale_reason };
	}

	function commit_result<T>(
		record: ScopeRecord<Kind>,
		fn: () => T,
		complete: boolean,
	): Core6ScopeCommitResult<T> {
		if (current_record !== record) {
			return inactive_result(record);
		}
		if (record.controller.signal.aborted) {
			return inactive_result(record);
		}
		if (complete) {
			try {
				const value = fn();
				return { ok: true, value };
			} finally {
				finish_record(record, core6_scope_completed_reason);
			}
		}
		const value = fn();
		return { ok: true, value };
	}

	function make_scope(record: ScopeRecord<Kind>): Core6Scope<Kind> {
		return {
			id: record.id,
			kind: record.kind,
			signal: record.controller.signal,
			cancel: () => {
				return finish_record(record, core6_scope_cancelled_reason);
			},
			commit: (fn) => {
				return commit_result(record, fn, false);
			},
			complete: (fn) => {
				return commit_result(record, fn, true);
			},
			live: () => {
				return (
					current_record === record &&
					!record.controller.signal.aborted
				);
			},
			on_cancel: (fn) => {
				if (
					record.terminal_reason === core6_scope_cancelled_reason ||
					record.terminal_reason === core6_scope_stale_reason
				) {
					fn();
					return () => {};
				}
				if (current_record !== record) {
					return () => {};
				}
				record.cancel_listeners.add(fn);
				return () => {
					record.cancel_listeners.delete(fn);
				};
			},
			on_cleanup: (fn) => {
				if (
					record.terminal_reason !== null ||
					current_record !== record ||
					record.controller.signal.aborted
				) {
					fn();
					return () => {};
				}
				record.cleanups.add(fn);
				return () => {
					record.cleanups.delete(fn);
				};
			},
		};
	}

	function start(kind: Kind): Core6Scope<Kind> {
		if (current_record) {
			finish_record(current_record, core6_scope_stale_reason);
		}
		next_id++;
		const record = {
			cancel_listeners: new Set<() => void>(),
			cleanups: new Set<() => void>(),
			controller: abort_factory(),
			id: next_id as Core6ScopeID,
			kind,
			scope: null as unknown as Core6Scope<Kind>,
			terminal_reason: null,
		};
		record.scope = make_scope(record);
		current_record = record;
		return record.scope;
	}

	return {
		cancel_current: () => {
			if (!current_record) {
				return false;
			}
			return finish_record(current_record, core6_scope_cancelled_reason);
		},
		current: () => {
			return current_record?.scope ?? null;
		},
		is_current: (scope) => {
			return current_record?.scope === scope;
		},
		start,
	};
}

export async function run_core6_scope_stage<Kind extends string, T>(
	scope: Core6Scope<Kind>,
	fn: (signal: AbortSignal) => Promise<T>,
): Promise<Core6ScopeCommitResult<T>> {
	if (!scope.live()) {
		return scope.commit(() => {
			throw new Error("Inactive scope cannot commit");
		}) as Core6ScopeCommitResult<T>;
	}
	let value: T;
	try {
		value = await fn(scope.signal);
	} catch (error) {
		if (!scope.live()) {
			return scope.commit(() => {
				throw error;
			}) as Core6ScopeCommitResult<T>;
		}
		throw error;
	}
	return scope.commit(() => {
		return value;
	});
}
