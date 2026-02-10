export function resolveVormaRequestBody(
	input: unknown,
): BodyInit | null | undefined {
	if (
		input == null ||
		typeof input === "string" ||
		input instanceof Blob ||
		input instanceof FormData ||
		input instanceof URLSearchParams ||
		input instanceof ReadableStream ||
		input instanceof ArrayBuffer ||
		ArrayBuffer.isView(input)
	) {
		return input;
	}
	return JSON.stringify(input);
}
