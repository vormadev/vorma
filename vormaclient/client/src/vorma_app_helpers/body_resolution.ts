import {
	isArrayBufferView,
	isInstanceOfGlobal,
} from "../utils/global_constructors.ts";

function normalizeArrayBufferViewBody(
	input: ArrayBufferView<ArrayBufferLike>,
): BodyInit {
	if (
		typeof ArrayBuffer !== "undefined" &&
		input.buffer instanceof ArrayBuffer
	) {
		return input as ArrayBufferView<ArrayBuffer>;
	}

	// SharedArrayBuffer-backed views are cloned into an ArrayBuffer-backed
	// Uint8Array so they remain valid body payloads under strict DOM typings.
	const cloned = new Uint8Array(input.byteLength);
	cloned.set(
		new Uint8Array(input.buffer, input.byteOffset, input.byteLength),
	);
	return cloned;
}

export function resolveVormaRequestBody(
	input: unknown,
): BodyInit | null | undefined {
	if (isArrayBufferView(input)) {
		return normalizeArrayBufferViewBody(input);
	}

	if (
		input == null ||
		typeof input === "string" ||
		isInstanceOfGlobal(input, "Blob") ||
		isInstanceOfGlobal(input, "FormData") ||
		isInstanceOfGlobal(input, "URLSearchParams") ||
		isInstanceOfGlobal(input, "ReadableStream") ||
		isInstanceOfGlobal(input, "ArrayBuffer")
	) {
		return input as BodyInit | null | undefined;
	}
	return JSON.stringify(input);
}
