export type GlobalCtorName =
	| "FormData"
	| "URLSearchParams"
	| "Blob"
	| "ArrayBuffer"
	| "ReadableStream";

export function isInstanceOfGlobal(
	value: unknown,
	ctorName: GlobalCtorName,
): boolean {
	const ctor = (globalThis as Record<string, unknown>)[ctorName];
	return typeof ctor === "function" && value instanceof (ctor as any);
}

export function isArrayBufferView(value: unknown): value is ArrayBufferView {
	return typeof ArrayBuffer !== "undefined" && ArrayBuffer.isView(value);
}
