export function is_abort_error(e: unknown): boolean {
	return e instanceof DOMException && e.name === "AbortError";
}

export function new_abort_error(): DOMException {
	return new DOMException("Aborted", "AbortError");
}

export function to_error_string(err: unknown): string {
	return err instanceof Error ? err.message : String(err);
}
