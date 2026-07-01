import {
	type Base64,
	type Base64Url,
	type Bytes,
	type Hex,
	type Utf8,
	base64ToBase64Url,
	base64ToBytes,
	base64ToHex,
	base64ToUtf8,
	base64UrlToBase64,
	base64UrlToBytes,
	base64UrlToHex,
	base64UrlToUtf8,
	bytesToBase64,
	bytesToBase64Url,
	bytesToHex,
	bytesToUtf8,
	hexToBase64,
	hexToBase64Url,
	hexToBytes,
	hexToUtf8,
	utf8ToBase64,
	utf8ToBase64Url,
	utf8ToBytes,
	utf8ToHex,
} from "vorma/kit/converters";
import { getClientCookie, setClientCookie } from "vorma/kit/cookies";
import { getCsrfToken } from "vorma/kit/csrf";
import { prettyJson } from "vorma/kit/fmt";
import {
	type SearchParamSchema,
	jsonDeepEquals,
	jsonStringifyStable,
	parseSearchParams,
	serializeToSearchParams,
} from "vorma/kit/json";
import { R, type Result } from "vorma/kit/result";

const diagnostics_cookie = "board_kit_diagnostics";
const csrf_cookie_name = "csrf_token";

type KitDiagnostics = {
	conversions: {
		utf8: Utf8;
		hex: Hex;
		base64: Base64;
		base64_url: Base64Url;
		round_trips: Record<string, string>;
	};
	cookies: {
		diagnostics_cookie: string | null;
		csrf_token: string | null;
	};
	search_params: {
		serialized: string;
		parsed: unknown;
		matches_expected_shape: boolean;
	};
	stable_json: string;
	pretty_json: string;
};

export function ensure_kit_diagnostic_cookies(): {
	diagnostics_cookie: string | null;
	csrf_token: string | null;
} {
	if (!getClientCookie(diagnostics_cookie)) {
		setClientCookie(diagnostics_cookie, "enabled");
	}
	if (
		import.meta.env.DEV &&
		!getCsrfToken({ isDev: true, cookieName: csrf_cookie_name })
	) {
		setClientCookie(`__Dev-${csrf_cookie_name}`, "board-dev-csrf");
	}
	return {
		diagnostics_cookie: getClientCookie(diagnostics_cookie) ?? null,
		csrf_token:
			getCsrfToken({
				isDev: import.meta.env.DEV,
				cookieName: csrf_cookie_name,
			}) ?? null,
	};
}

export function build_kit_diagnostics(raw_text: string): Result<KitDiagnostics> {
	const text: Utf8 = raw_text.trim() || "Vorma Board diagnostics";
	const bytes: Bytes = utf8ToBytes(text);
	const hex: Hex = bytesToHex(bytes);
	const base64: Base64 = bytesToBase64(bytes);
	const base64_url: Base64Url = bytesToBase64Url(bytes);
	const direct_hex: Hex = utf8ToHex(text);
	const direct_base64: Base64 = utf8ToBase64(text);
	const direct_base64_url: Base64Url = utf8ToBase64Url(text);

	if (
		!jsonDeepEquals(
			{ hex, base64, base64_url },
			{
				hex: direct_hex,
				base64: direct_base64,
				base64_url: direct_base64_url,
			},
		)
	) {
		return R.err("converter round trip mismatch");
	}

	const search_shape: SearchParamSchema = {
		q: "?s",
		page: "n",
		tags: ["s"],
		flags: ["*", "b"],
	};
	const expected_search = {
		flags: { archived: false, pinned: true },
		page: 1,
		q: text,
		tags: ["diagnostics", "kit"],
	};
	const serialized = serializeToSearchParams(expected_search);
	const parsed = parseSearchParams(search_shape, serialized);
	const matches_expected_shape = jsonDeepEquals(parsed, expected_search);

	const diagnostics: KitDiagnostics = {
		conversions: {
			utf8: bytesToUtf8(bytes),
			hex,
			base64,
			base64_url,
			round_trips: {
				base64_to_base64_url: base64ToBase64Url(base64),
				base64_to_hex: base64ToHex(base64),
				base64_to_utf8: base64ToUtf8(base64),
				base64_url_to_base64: base64UrlToBase64(base64_url),
				base64_url_to_hex: base64UrlToHex(base64_url),
				base64_url_to_utf8: base64UrlToUtf8(base64_url),
				hex_to_base64: hexToBase64(hex),
				hex_to_base64_url: hexToBase64Url(hex),
				hex_to_utf8: hexToUtf8(hex),
			},
		},
		cookies: ensure_kit_diagnostic_cookies(),
		search_params: {
			serialized: serialized.toString(),
			parsed,
			matches_expected_shape,
		},
		stable_json: "",
		pretty_json: "",
	};

	const stable = jsonStringifyStable({
		base64_bytes_hex: bytesToHex(base64ToBytes(base64)),
		base64_url_bytes_hex: bytesToHex(base64UrlToBytes(base64_url)),
		hex_bytes_utf8: bytesToUtf8(hexToBytes(hex)),
		search_params: diagnostics.search_params,
	});
	if (!stable.ok) {
		return stable;
	}

	diagnostics.stable_json = stable.val;
	diagnostics.pretty_json = prettyJson(diagnostics);
	return R.ok(diagnostics);
}
