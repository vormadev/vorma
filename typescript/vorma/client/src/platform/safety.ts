export function logInfo(message?: unknown, ...optionalParams: Array<unknown>) {
	console.log("Vorma:", message, ...optionalParams);
}

export function logError(message?: unknown, ...optionalParams: Array<unknown>) {
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

const objectTagByGlobalCtorName: Record<GlobalCtorName, string> = {
	FormData: "[object FormData]",
	URLSearchParams: "[object URLSearchParams]",
	Blob: "[object Blob]",
	ArrayBuffer: "[object ArrayBuffer]",
	ReadableStream: "[object ReadableStream]",
};

function isCrossRealmInstanceOfGlobal(
	value: unknown,
	ctorName: GlobalCtorName,
): boolean {
	if (
		(typeof value !== "object" && typeof value !== "function") ||
		value === null
	) {
		return false;
	}

	const expectedTag = objectTagByGlobalCtorName[ctorName];
	return Object.prototype.toString.call(value) === expectedTag;
}

export function isInstanceOfGlobal(
	value: unknown,
	ctorName: GlobalCtorName,
): boolean {
	const ctor = (globalThis as Record<string, unknown>)[ctorName];
	if (typeof ctor !== "function") {
		return false;
	}

	const typedCtor = ctor as new (...args: Array<never>) => object;
	if (value instanceof typedCtor) {
		return true;
	}

	return isCrossRealmInstanceOfGlobal(value, ctorName);
}

export function isArrayBufferView(value: unknown): value is ArrayBufferView {
	return typeof ArrayBuffer !== "undefined" && ArrayBuffer.isView(value);
}

export function observePromiseRejection<T>(promise: Promise<T>): Promise<T> {
	void promise.catch(() => {});
	return promise;
}
