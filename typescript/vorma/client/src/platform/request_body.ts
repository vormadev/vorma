import { isArrayBufferView, isInstanceOfGlobal } from "./safety.ts";

export type RequestBodyTransportResolution = {
	body: BodyInit | null | undefined;
	didSerializeJSON: boolean;
};

function normalizeArrayBufferViewBody(
	arrayBufferView: ArrayBufferView<ArrayBufferLike>,
): BodyInit {
	if (
		typeof ArrayBuffer !== "undefined" &&
		arrayBufferView.buffer instanceof ArrayBuffer
	) {
		return arrayBufferView as ArrayBufferView<ArrayBuffer>;
	}

	// SharedArrayBuffer-backed views are cloned into an ArrayBuffer-backed
	// Uint8Array so they remain valid body payloads under strict DOM typings.
	const clonedArrayBufferView = new Uint8Array(arrayBufferView.byteLength);
	clonedArrayBufferView.set(
		new Uint8Array(
			arrayBufferView.buffer,
			arrayBufferView.byteOffset,
			arrayBufferView.byteLength,
		),
	);

	return clonedArrayBufferView;
}

function isBodyTransportValueWithoutJSONSerialization(
	input: unknown,
): input is BodyInit | null | undefined {
	return (
		input == null ||
		typeof input === "string" ||
		isInstanceOfGlobal(input, "Blob") ||
		isInstanceOfGlobal(input, "FormData") ||
		isInstanceOfGlobal(input, "URLSearchParams") ||
		isInstanceOfGlobal(input, "ReadableStream") ||
		isInstanceOfGlobal(input, "ArrayBuffer")
	);
}

export function resolveRequestBodyForTransport(props: {
	input: unknown;
}): RequestBodyTransportResolution {
	const { input } = props;

	if (isArrayBufferView(input)) {
		return {
			body: normalizeArrayBufferViewBody(input),
			didSerializeJSON: false,
		};
	}

	if (isBodyTransportValueWithoutJSONSerialization(input)) {
		return {
			body: input,
			didSerializeJSON: false,
		};
	}

	return {
		body: JSON.stringify(input),
		didSerializeJSON: true,
	};
}
