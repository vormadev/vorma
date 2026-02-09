import { logError } from "./logging.ts";

export function isAbortError(error: unknown) {
	if (error instanceof Error && error.name === "AbortError") {
		return true;
	}

	if (
		error &&
		typeof error === "object" &&
		"name" in error &&
		(error as { name?: unknown }).name === "AbortError"
	) {
		return true;
	}

	return false;
}

export function panic(msg?: string): never {
	logError("Panic");
	throw new Error(msg ?? "panic");
}
