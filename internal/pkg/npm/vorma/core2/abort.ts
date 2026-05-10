export function is_abort_cause(cause: unknown): boolean {
	if (cause instanceof DOMException && cause.name === "AbortError") {
		return true;
	}
	if (cause instanceof Error && cause.name === "AbortError") {
		return true;
	}
	return false;
}

export function new_abort_error(): DOMException {
	return new DOMException("Aborted", "AbortError");
}
