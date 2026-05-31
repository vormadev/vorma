/////////////////////////////////////////////////////////////////////
/////// TYPE ALIASES
/////////////////////////////////////////////////////////////////////

export type Bytes = Uint8Array;
export type Utf8 = string;
export type Hex = string;
export type Base64 = string;
export type Base64Url = string;

/////////////////////////////////////////////////////////////////////
/////// BYTES --> X
/////////////////////////////////////////////////////////////////////

// --> UTF8
export function bytesToUtf8(bytes: Uint8Array): Utf8 {
	return new TextDecoder().decode(bytes);
}

// --> HEX
export function bytesToHex(bytes: Uint8Array): Hex {
	return Array.from(bytes, (x) => x.toString(16).padStart(2, "0")).join("");
}

// --> BASE64
export function bytesToBase64(bytes: Uint8Array): Base64 {
	const callback = (x: number) => String.fromCodePoint(x);
	return btoa(Array.from(bytes, callback).join(""));
}

// --> BASE64URL
export function bytesToBase64Url(bytes: Uint8Array): Base64Url {
	const base64 = bytesToBase64(bytes);
	return base64ToBase64Url(base64);
}

/////////////////////////////////////////////////////////////////////
/////// UTF8 --> X
/////////////////////////////////////////////////////////////////////

// --> BYTES
export function utf8ToBytes(utf8: Utf8): Uint8Array {
	return new TextEncoder().encode(utf8);
}

// --> HEX
export function utf8ToHex(utf8: Utf8): Hex {
	const bytes = utf8ToBytes(utf8);
	return bytesToHex(bytes);
}

// --> BASE64
export function utf8ToBase64(utf8: Utf8): Base64 {
	const bytes = utf8ToBytes(utf8);
	return bytesToBase64(bytes);
}

// --> BASE64URL
export function utf8ToBase64Url(utf8: Utf8): Base64Url {
	const bytes = utf8ToBytes(utf8);
	return bytesToBase64Url(bytes);
}

/////////////////////////////////////////////////////////////////////
/////// HEX --> X
/////////////////////////////////////////////////////////////////////

// --> BYTES
export function hexToBytes(hex: Hex): Uint8Array {
	const cleanHex = hex.startsWith("0x") ? hex.slice(2) : hex;
	const bytes =
		cleanHex.match(/.{1,2}/g)?.map((byte) => Number.parseInt(byte, 16)) || [];
	return new Uint8Array(bytes);
}

// --> UTF8
export function hexToUtf8(hex: Hex): Utf8 {
	const bytes = hexToBytes(hex);
	return bytesToUtf8(bytes);
}

// --> BASE64
export function hexToBase64(hex: Hex): Base64 {
	const bytes = hexToBytes(hex);
	return bytesToBase64(bytes);
}

// --> BASE64URL
export function hexToBase64Url(hex: Hex): Base64Url {
	const bytes = hexToBytes(hex);
	return bytesToBase64Url(bytes);
}

/////////////////////////////////////////////////////////////////////
/////// BASE64 --> X
/////////////////////////////////////////////////////////////////////

// --> BYTES
export function base64ToBytes(base64: Base64): Uint8Array {
	return Uint8Array.from(atob(base64), (m) => m.codePointAt(0) || 0);
}

// --> UTF8
export function base64ToUtf8(base64: Base64): Utf8 {
	const bytes = base64ToBytes(base64);
	return bytesToUtf8(bytes);
}

// --> HEX
export function base64ToHex(base64: Base64): Hex {
	const bytes = base64ToBytes(base64);
	return bytesToHex(bytes);
}

// --> BASE64URL
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

// --> BYTES
export function base64UrlToBytes(base64Url: Base64Url): Uint8Array {
	const base64 = base64UrlToBase64(base64Url);
	return base64ToBytes(base64);
}

// --> UTF8
export function base64UrlToUtf8(base64Url: Base64Url): Utf8 {
	const bytes = base64UrlToBytes(base64Url);
	return bytesToUtf8(bytes);
}

// --> HEX
export function base64UrlToHex(base64Url: Base64Url): Hex {
	const bytes = base64UrlToBytes(base64Url);
	return bytesToHex(bytes);
}

// --> BASE64
export function base64UrlToBase64(base64Url: Base64Url): Base64 {
	return base64Url
		.padEnd(base64Url.length + ((4 - (base64Url.length % 4)) % 4), "=")
		.replace(/-/g, "+")
		.replace(/_/g, "/");
}
