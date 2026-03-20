/// <reference types="vite/client" />

const PREFIX = "__vorma_error__:";

export const ABORT_REASON = {
	superseded_nav: "superseded_by_new_navigation",
	superseded_revalidate: "superseded_by_new_revalidation",
	superseded_dedupe: "superseded_by_submit_dedupe",
	clear_all: "clear_all",
	prefetch_stopped: "prefetch_stopped",
} as const;

type AbortReasonCode = (typeof ABORT_REASON)[keyof typeof ABORT_REASON];

const VALID_CODES = new Set<string>(Object.values(ABORT_REASON));

export function encode_abort_reason(code: AbortReasonCode): string {
	return `${PREFIX}${code}`;
}

export function is_abort_error(error: unknown): boolean {
	// Check our structured reasons first
	if (typeof error === "string" && error.startsWith(PREFIX)) {
		const code = error.slice(PREFIX.length);
		if (VALID_CODES.has(code)) return true;
	}
	// Standard AbortError
	if (!error || typeof error !== "object") return false;
	const e = error as Record<string, any>;
	return (
		e.name === "AbortError" ||
		String(e.message ?? "")
			.toLowerCase()
			.includes("abort")
	);
}

export function panic(msg: string): never {
	throw new Error(msg);
}
