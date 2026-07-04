/*
Pairwise converters between five common encodings — bytes, UTF-8 text, hex,
base64, and URL-safe base64 — named `<from>To<to>`. Every conversion not
directly through bytes composes through it internally (e.g. `hexToBase64`
decodes hex to bytes, then re-encodes), so bytes is the hub encoding.
Base64/Base64Url conversions use the platform `atob`/`btoa`; this module
has no external dependency and no Vorma/backend awareness — generic
byte-shuffling utilities usable in any TypeScript project.
*/

/////////////////////////////////////////////////////////////////////
/////// TYPE ALIASES
/////////////////////////////////////////////////////////////////////

/** Raw bytes — the hub encoding every converter here routes through. */
export type Bytes = Uint8Array;
/** Plain UTF-8 text (a type alias over `string`, for signature clarity — not a distinct runtime type). */
export type Utf8 = string;
/** Lowercase hex-encoded string (a type alias over `string`). */
export type Hex = string;
/** Standard base64-encoded string, `+`/`/` alphabet with `=` padding (a type alias over `string`). */
export type Base64 = string;
/** URL-safe base64-encoded string, `-`/`_` alphabet with padding stripped (a type alias over `string`). */
export type Base64Url = string;

/////////////////////////////////////////////////////////////////////
/////// BYTES --> X
/////////////////////////////////////////////////////////////////////

/** Decode bytes as UTF-8 text. */
export function bytesToUtf8(bytes: Uint8Array): Utf8 {
	return new TextDecoder().decode(bytes);
}

/** Encode bytes as lowercase hex. */
export function bytesToHex(bytes: Uint8Array): Hex {
	return Array.from(bytes, (x) => x.toString(16).padStart(2, "0")).join("");
}

/** Encode bytes as standard base64. */
export function bytesToBase64(bytes: Uint8Array): Base64 {
	const callback = (x: number) => String.fromCodePoint(x);
	return btoa(Array.from(bytes, callback).join(""));
}

/** Encode bytes as URL-safe base64. */
export function bytesToBase64Url(bytes: Uint8Array): Base64Url {
	const base64 = bytesToBase64(bytes);
	return base64ToBase64Url(base64);
}

/////////////////////////////////////////////////////////////////////
/////// UTF8 --> X
/////////////////////////////////////////////////////////////////////

/** Encode UTF-8 text as bytes. */
export function utf8ToBytes(utf8: Utf8): Uint8Array {
	return new TextEncoder().encode(utf8);
}

/** Encode UTF-8 text as lowercase hex. */
export function utf8ToHex(utf8: Utf8): Hex {
	const bytes = utf8ToBytes(utf8);
	return bytesToHex(bytes);
}

/** Encode UTF-8 text as standard base64. */
export function utf8ToBase64(utf8: Utf8): Base64 {
	const bytes = utf8ToBytes(utf8);
	return bytesToBase64(bytes);
}

/** Encode UTF-8 text as URL-safe base64. */
export function utf8ToBase64Url(utf8: Utf8): Base64Url {
	const bytes = utf8ToBytes(utf8);
	return bytesToBase64Url(bytes);
}

/////////////////////////////////////////////////////////////////////
/////// HEX --> X
/////////////////////////////////////////////////////////////////////

/** Decode hex (an optional leading `0x` is stripped) as bytes. */
export function hexToBytes(hex: Hex): Uint8Array {
	const clean_hex = hex.startsWith("0x") ? hex.slice(2) : hex;
	const bytes =
		clean_hex.match(/.{1,2}/g)?.map((byte) => Number.parseInt(byte, 16)) || [];
	return new Uint8Array(bytes);
}

/** Decode hex as UTF-8 text. */
export function hexToUtf8(hex: Hex): Utf8 {
	const bytes = hexToBytes(hex);
	return bytesToUtf8(bytes);
}

/** Re-encode hex as standard base64. */
export function hexToBase64(hex: Hex): Base64 {
	const bytes = hexToBytes(hex);
	return bytesToBase64(bytes);
}

/** Re-encode hex as URL-safe base64. */
export function hexToBase64Url(hex: Hex): Base64Url {
	const bytes = hexToBytes(hex);
	return bytesToBase64Url(bytes);
}

/////////////////////////////////////////////////////////////////////
/////// BASE64 --> X
/////////////////////////////////////////////////////////////////////

/** Decode standard base64 as bytes. */
export function base64ToBytes(base64: Base64): Uint8Array {
	return Uint8Array.from(atob(base64), (m) => m.codePointAt(0) || 0);
}

/** Decode standard base64 as UTF-8 text. */
export function base64ToUtf8(base64: Base64): Utf8 {
	const bytes = base64ToBytes(base64);
	return bytesToUtf8(bytes);
}

/** Re-encode standard base64 as lowercase hex. */
export function base64ToHex(base64: Base64): Hex {
	const bytes = base64ToBytes(base64);
	return bytesToHex(bytes);
}

/** Convert standard base64 to URL-safe base64 (swap `+`/`/` for `-`/`_`, strip `=` padding and whitespace). */
export function base64ToBase64Url(base64: Base64): Base64Url {
	return base64
		.replace(/[\r\n\t ]+/g, "")
		.replace(/\+/g, "-")
		.replace(/\//g, "_")
		.replace(/=+$/, "");
}

/////////////////////////////////////////////////////////////////////
/////// BASE64URL --> X
/////////////////////////////////////////////////////////////////////

/** Decode URL-safe base64 as bytes. */
export function base64UrlToBytes(base64Url: Base64Url): Uint8Array {
	const base64 = base64UrlToBase64(base64Url);
	return base64ToBytes(base64);
}

/** Decode URL-safe base64 as UTF-8 text. */
export function base64UrlToUtf8(base64Url: Base64Url): Utf8 {
	const bytes = base64UrlToBytes(base64Url);
	return bytesToUtf8(bytes);
}

/** Re-encode URL-safe base64 as lowercase hex. */
export function base64UrlToHex(base64Url: Base64Url): Hex {
	const bytes = base64UrlToBytes(base64Url);
	return bytesToHex(bytes);
}

/** Convert URL-safe base64 to standard base64 (swap `-`/`_` for `+`/`/`, restore `=` padding). */
export function base64UrlToBase64(base64Url: Base64Url): Base64 {
	return base64Url
		.padEnd(base64Url.length + ((4 - (base64Url.length % 4)) % 4), "=")
		.replace(/-/g, "+")
		.replace(/_/g, "/");
}
