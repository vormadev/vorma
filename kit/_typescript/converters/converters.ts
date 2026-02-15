/////////////////////////////////////////////////////////////////////
/////// TYPE ALIASES
/////////////////////////////////////////////////////////////////////

export type Bytes = Uint8Array;
export type UTF8 = string;
export type Hex = string;
export type Base64 = string;
export type Base64URL = string;

/////////////////////////////////////////////////////////////////////
/////// BYTES --> X
/////////////////////////////////////////////////////////////////////

// --> UTF8
export function bytesToUTF8(bytes: Uint8Array): UTF8 {
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
export function bytesToBase64URL(bytes: Uint8Array): Base64URL {
	const base64 = bytesToBase64(bytes);
	return base64ToBase64URL(base64);
}

/////////////////////////////////////////////////////////////////////
/////// UTF8 --> X
/////////////////////////////////////////////////////////////////////

// --> BYTES
export function utf8ToBytes(utf8: UTF8): Uint8Array {
	return new TextEncoder().encode(utf8);
}

// --> HEX
export function utf8ToHex(utf8: UTF8): Hex {
	const bytes = utf8ToBytes(utf8);
	return bytesToHex(bytes);
}

// --> BASE64
export function utf8ToBase64(utf8: UTF8): Base64 {
	const bytes = utf8ToBytes(utf8);
	return bytesToBase64(bytes);
}

// --> BASE64URL
export function utf8ToBase64URL(utf8: UTF8): Base64URL {
	const bytes = utf8ToBytes(utf8);
	return bytesToBase64URL(bytes);
}

/////////////////////////////////////////////////////////////////////
/////// HEX --> X
/////////////////////////////////////////////////////////////////////

// --> BYTES
export function hexToBytes(hex: Hex): Uint8Array {
	const cleanHex = hex.startsWith("0x") ? hex.slice(2) : hex;
	const bytes =
		cleanHex.match(/.{1,2}/g)?.map((byte) => Number.parseInt(byte, 16)) ||
		[];
	return new Uint8Array(bytes);
}

// --> UTF8
export function hexToUTF8(hex: Hex): UTF8 {
	const bytes = hexToBytes(hex);
	return bytesToUTF8(bytes);
}

// --> BASE64
export function hexToBase64(hex: Hex): Base64 {
	const bytes = hexToBytes(hex);
	return bytesToBase64(bytes);
}

// --> BASE64URL
export function hexToBase64URL(hex: Hex): Base64URL {
	const bytes = hexToBytes(hex);
	return bytesToBase64URL(bytes);
}

/////////////////////////////////////////////////////////////////////
/////// BASE64 --> X
/////////////////////////////////////////////////////////////////////

// --> BYTES
export function base64ToBytes(base64: Base64): Uint8Array {
	return Uint8Array.from(atob(base64), (m) => m.codePointAt(0) || 0);
}

// --> UTF8
export function base64ToUTF8(base64: Base64): UTF8 {
	const bytes = base64ToBytes(base64);
	return bytesToUTF8(bytes);
}

// --> HEX
export function base64ToHex(base64: Base64): Hex {
	const bytes = base64ToBytes(base64);
	return bytesToHex(bytes);
}

// --> BASE64URL
export function base64ToBase64URL(base64: Base64): Base64URL {
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
export function base64URLToBytes(base64URL: Base64URL): Uint8Array {
	const base64 = base64URLToBase64(base64URL);
	return base64ToBytes(base64);
}

// --> UTF8
export function base64URLToUTF8(base64URL: Base64URL): UTF8 {
	const bytes = base64URLToBytes(base64URL);
	return bytesToUTF8(bytes);
}

// --> HEX
export function base64URLToHex(base64URL: Base64URL): Hex {
	const bytes = base64URLToBytes(base64URL);
	return bytesToHex(bytes);
}

// --> BASE64
export function base64URLToBase64(base64URL: Base64URL): Base64 {
	return base64URL
		.padEnd(base64URL.length + ((4 - (base64URL.length % 4)) % 4), "=")
		.replace(/-/g, "+")
		.replace(/_/g, "/");
}
