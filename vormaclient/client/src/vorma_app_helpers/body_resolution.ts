function normalizeArrayBufferViewBody(
	input: ArrayBufferView<ArrayBufferLike>,
): BodyInit {
	if (input.buffer instanceof ArrayBuffer) {
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
	if (ArrayBuffer.isView(input)) {
		return normalizeArrayBufferViewBody(input);
	}

	if (
		input == null ||
		typeof input === "string" ||
		input instanceof Blob ||
		input instanceof FormData ||
		input instanceof URLSearchParams ||
		input instanceof ReadableStream ||
		input instanceof ArrayBuffer
	) {
		return input;
	}
	return JSON.stringify(input);
}
