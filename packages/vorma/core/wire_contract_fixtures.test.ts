// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import { route_response } from "./_test_helpers.ts";
import { BUILD_ID_HEADER, X_CLIENT_REDIRECT, X_VORMA_BUILD_SKEW } from "./constants.ts";
import fixture_file from "./wire_contract_fixtures.json";

/**
 * These fixtures are captured from a real in-memory Rust app by
 * crates/vorma/tests/wire_contract_fixtures.rs. The Rust suite fails when the
 * server's wire output drifts from the committed file; this suite fails when
 * the client's constants or test-helper payload shapes drift from it. Together
 * they pin both runtimes to the same bytes.
 */
type WireFixture = {
	name: string;
	request: { method: string; path: string };
	response: {
		status: number;
		headers: Record<string, Array<string>>;
		body_json?: unknown;
		body_text?: string;
	};
};

const fixtures = fixture_file.fixtures as unknown as Array<WireFixture>;

function fixture(name: string): WireFixture {
	const found = fixtures.find((candidate) => candidate.name === name);
	if (!found) {
		throw new Error(`missing wire fixture ${name}`);
	}
	return found;
}

function is_view_payload(candidate: WireFixture): boolean {
	return (
		typeof candidate.response.body_json === "object" &&
		candidate.response.body_json !== null &&
		"matched_patterns" in candidate.response.body_json
	);
}

const view_payload_fixtures = fixtures.filter(is_view_payload);

/**
 * Complete view-payload key universe the server can emit. When the server
 * grows or renames a payload field, the Rust capture test regenerates the
 * fixtures and this list must be updated in the same change — which is the
 * point: the client suite cannot silently ignore a wire change.
 */
const KNOWN_VIEW_PAYLOAD_KEYS = [
	"css_bundles",
	"deps",
	"import_urls",
	"matched_patterns",
	"meta_head_els",
	"outermost_server_err",
	"outermost_server_err_idx",
	"params",
	"rest_head_els",
	"search_schemas",
	"splat_values",
	"title",
	"views_data",
];

/**
 * Captured keys absent from the hand-written `route_response()` defaults in
 * `_test_helpers.ts`. Shrinking this list (modeling these fields in the
 * helper) is good; growing it means the server added a field the frontend
 * test fixtures do not model — handle the field, then update the list.
 */
const HELPER_DEFAULTS_UNMODELED_KEYS = [
	"outermost_server_err",
	"outermost_server_err_idx",
	"search_schemas",
	"title",
];

async function helper_default_payload_keys(): Promise<Array<string>> {
	// JSON round-trip drops undefined fields, mirroring real serialization.
	const body = (await route_response().json()) as Record<string, unknown>;
	return Object.keys(body).sort();
}

function captured_payload_keys(): Array<string> {
	const keys = new Set<string>();
	for (const candidate of view_payload_fixtures) {
		for (const key of Object.keys(
			candidate.response.body_json as Record<string, unknown>,
		)) {
			keys.add(key);
		}
	}
	return [...keys].sort();
}

describe("wire contract fixtures (captured from the Rust server)", () => {
	it("captures the expected fixture set", () => {
		expect(fixtures.length).toBeGreaterThanOrEqual(11);
		expect(view_payload_fixtures.length).toBeGreaterThanOrEqual(4);
	});

	it("view payloads stay within the known contract key set", () => {
		for (const candidate of view_payload_fixtures) {
			for (const key of Object.keys(
				candidate.response.body_json as Record<string, unknown>,
			)) {
				expect(
					KNOWN_VIEW_PAYLOAD_KEYS,
					`${candidate.name} emitted unknown payload key "${key}"`,
				).toContain(key);
			}
		}
	});

	it("test-helper payload keys are all keys the server actually emits", async () => {
		const captured = captured_payload_keys();
		for (const key of await helper_default_payload_keys()) {
			expect(
				captured,
				`route_response() emits key "${key}" the server never emits`,
			).toContain(key);
		}
	});

	it("captured keys missing from the test helper are explicitly tracked", async () => {
		const helper_keys = new Set(await helper_default_payload_keys());
		const missing = captured_payload_keys().filter((key) => !helper_keys.has(key));
		expect(missing).toEqual(HELPER_DEFAULTS_UNMODELED_KEYS);
	});

	it("every captured response carries the client's build-id header constant", () => {
		for (const candidate of fixtures) {
			expect(
				candidate.response.headers[BUILD_ID_HEADER.toLowerCase()],
				`${candidate.name} is missing ${BUILD_ID_HEADER}`,
			).toEqual(["vorma-test-build"]);
		}
	});

	it("redirect and build-skew responses use the client's header constants", () => {
		const native = fixture("view_redirect_native");
		expect(native.response.status).toBe(303);
		expect(native.response.headers.location).toEqual(["/stories/1"]);

		const client = fixture("view_redirect_client");
		expect(client.response.status).toBe(200);
		expect(client.response.headers[X_CLIENT_REDIRECT.toLowerCase()]).toEqual([
			"/stories/1",
		]);

		const skew = fixture("view_payload_build_skew");
		expect(skew.response.headers[X_VORMA_BUILD_SKEW.toLowerCase()]).toEqual(["1"]);
	});

	it("nested payloads align views_data positionally with matched_patterns", () => {
		const nested = fixture("view_payload_nested_with_input").response
			.body_json as Record<string, unknown>;
		expect(nested.matched_patterns).toEqual(["/", "/stories/:id"]);
		expect(nested.views_data).toEqual([
			{ section: "fx-root" },
			{ story: "fx-story-5", query: "ada" },
		]);
		expect(nested.params).toEqual({ id: "5" });
	});

	it("splat payloads carry splat_values", () => {
		const splat = fixture("view_payload_splat").response.body_json as Record<
			string,
			unknown
		>;
		expect(splat.splat_values).toEqual(["docs", "readme.md"]);
	});

	it("server-error payloads carry the error boundary fields", () => {
		const broken = fixture("view_payload_server_error").response.body_json as Record<
			string,
			unknown
		>;
		expect(broken.outermost_server_err_idx).toBeTypeOf("number");
		expect(broken.outermost_server_err).toBeTruthy();
	});
});
