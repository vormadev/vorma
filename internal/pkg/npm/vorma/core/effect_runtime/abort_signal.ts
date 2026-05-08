const abort_error_name = "AbortError";
const abort_error_message = "Aborted";

export function merge_abort_signals(
	signals: ReadonlyArray<AbortSignal | null | undefined>,
): AbortSignal {
	const live_signals = signals.filter((signal): signal is AbortSignal => {
		return signal !== null && signal !== undefined;
	});
	if (live_signals.length === 1) {
		return live_signals[0]!;
	}
	const controller = new AbortController();
	for (const signal of live_signals) {
		if (signal.aborted) {
			controller.abort();
			break;
		}
		signal.addEventListener(
			"abort",
			() => {
				controller.abort();
			},
			{ once: true },
		);
	}
	return controller.signal;
}

export function new_abort_error(): DOMException {
	return new DOMException(abort_error_message, abort_error_name);
}

export function is_abort_error(error: unknown): boolean {
	return error instanceof DOMException && error.name === abort_error_name;
}
