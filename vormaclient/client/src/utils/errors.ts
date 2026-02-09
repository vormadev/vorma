import { logError } from "./logging.ts";

export function isAbortError(error: unknown) {
	return error instanceof Error && error.name === "AbortError";
}

export function panic(msg?: string): never {
	logError("Panic");
	throw new Error(msg ?? "panic");
}
