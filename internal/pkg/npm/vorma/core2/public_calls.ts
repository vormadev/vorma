import type { PublicCallRegistry } from "./host.ts";
import type { PublicCallID } from "./model.ts";

type PublicCallWaiter = {
	reject: (cause: unknown) => void;
	resolve: (result: unknown) => void;
};

export type PublicCallStore = PublicCallRegistry & {
	wait: (public_call_id: PublicCallID) => Promise<unknown>;
};

export function create_public_call_store(): PublicCallStore {
	const waiters = new Map<PublicCallID, PublicCallWaiter>();

	function wait(public_call_id: PublicCallID): Promise<unknown> {
		if (waiters.has(public_call_id)) {
			throw new Error(`Duplicate public call ID: ${public_call_id}`);
		}
		return new Promise((resolve, reject) => {
			waiters.set(public_call_id, { reject, resolve });
		});
	}

	function resolve(public_call_id: PublicCallID, result: unknown): void {
		const waiter = waiters.get(public_call_id);
		if (!waiter) {
			return;
		}
		waiters.delete(public_call_id);
		waiter.resolve(result);
	}

	function reject(public_call_id: PublicCallID, cause: unknown): void {
		const waiter = waiters.get(public_call_id);
		if (!waiter) {
			return;
		}
		waiters.delete(public_call_id);
		waiter.reject(cause);
	}

	return { reject, resolve, wait };
}
