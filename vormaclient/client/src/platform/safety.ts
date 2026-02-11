export function logInfo(message?: any, ...optionalParams: Array<any>) {
	console.log("Vorma:", message, ...optionalParams);
}

export function logError(message?: any, ...optionalParams: Array<any>) {
	console.error("Vorma:", message, ...optionalParams);
}

export function isAbortError(error: unknown) {
	return error instanceof Error && error.name === "AbortError";
}

export function panic(msg?: string): never {
	logError("Panic");
	throw new Error(msg ?? "panic");
}

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

export function observePromiseRejection<T>(promise: Promise<T>): Promise<T> {
	void promise.catch(() => {});
	return promise;
}
