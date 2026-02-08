import { render as renderSolid } from "solid-js/web";
import React from "react";
import { describe, expect, it, vi } from "vitest";

import {
	VormaLink as PreactVormaLink,
} from "../../preact/src/link.tsx";
import { makeTypedLink as makePreactTypedLink } from "../../preact/src/link.tsx";
import {
	VormaLink as ReactVormaLink,
} from "../../react/src/link.tsx";
import { makeTypedLink as makeReactTypedLink } from "../../react/src/link.tsx";
import {
	VormaLink as SolidVormaLink,
} from "../../solid/src/link.tsx";
import { makeTypedLink as makeSolidTypedLink } from "../../solid/src/link.tsx";

import * as clientModule from "./client.ts";
import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";
import { makeTypedNavigate } from "./ui_lib_impl_helpers/typed_navigate.ts";
import {
	__resolvePath,
	type VormaAppConfig,
} from "./vorma_app_helpers/vorma_app_helpers.ts";

const testConfig = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
} as const satisfies VormaAppConfig;

function unwrapMemoLikeComponent<T>(component: T): any {
	const maybeMemo = component as any;
	return maybeMemo?.type ?? maybeMemo;
}

function renderSolidAnchor(renderFn: () => unknown): {
	anchor: HTMLAnchorElement;
	cleanup: () => void;
} {
	const container = document.createElement("div");
	document.body.appendChild(container);
	const dispose = renderSolid(() => renderFn(), container);
	const anchor = container.querySelector("a");
	if (!(anchor instanceof HTMLAnchorElement)) {
		dispose();
		container.remove();
		throw new Error("expected Solid link render to produce an anchor");
	}
	return {
		anchor,
		cleanup: () => {
			dispose();
			container.remove();
		},
	};
}

function getRenderedAnchorPropsForAdapter(
	adapter: "react" | "preact",
	props: any,
): Record<string, any> {
	if (adapter === "react") {
		const component = unwrapMemoLikeComponent(ReactVormaLink);
		return component(props).props;
	}
	const component = unwrapMemoLikeComponent(PreactVormaLink);
	return component(props).props;
}

describe("UI adapter parity conformance", () => {
	it("FEC-UI-004_FE-UI-005_typed_link_resolves_equivalent_targets_across_react_preact_and_solid", () => {
		window.history.replaceState({}, "", "http://localhost:3000/current");

		const linkProps = {
			pattern: "/users/:id/*",
			params: { id: "42" },
			splatValues: ["profile", "settings"],
			search: "?tab=details",
			hash: "#top",
		};

		const expectedPath = __resolvePath({
			type: "loader",
			vormaAppConfig: testConfig,
			props: {
				pattern: linkProps.pattern,
				params: linkProps.params,
				splatValues: linkProps.splatValues,
			},
		});
		const expectedURL = new URL(expectedPath, window.location.origin);
		expectedURL.search = linkProps.search;
		expectedURL.hash = linkProps.hash;
		const expectedHref = expectedURL.href;

		const ReactTypedLink = makeReactTypedLink(testConfig);
		const reactVNode = unwrapMemoLikeComponent(ReactTypedLink)(linkProps);
		expect(reactVNode.props.href).toBe(expectedHref);

		const PreactTypedLink = makePreactTypedLink(testConfig);
		const preactVNode = unwrapMemoLikeComponent(PreactTypedLink)(linkProps);
		expect(preactVNode.props.href).toBe(expectedHref);

		const SolidTypedLink = makeSolidTypedLink(testConfig);
		const rendered = renderSolidAnchor(() => SolidTypedLink(linkProps as any));
		expect(rendered.anchor.href).toBe(expectedHref);
		rendered.cleanup();
	});

	it("FEC-UI-004_FE-UI-005_typed_navigate_uses_same_base_path_resolution_contract", async () => {
		const navigateSpy = vi
			.spyOn(clientModule, "vormaNavigate")
			.mockResolvedValue(undefined);

		const typedNavigate = makeTypedNavigate(testConfig);
		await typedNavigate({
			pattern: "/users/:id/*",
			params: { id: "42" },
			splatValues: ["profile", "settings"],
			search: "?tab=details",
			hash: "#top",
			replace: true,
			scrollToTop: false,
			state: { from: "conformance" },
		} as any);

		const expectedPath = __resolvePath({
			type: "loader",
			vormaAppConfig: testConfig,
			props: {
				pattern: "/users/:id/*",
				params: { id: "42" },
				splatValues: ["profile", "settings"],
			},
		});
		expect(navigateSpy).toHaveBeenCalledWith(expectedPath, {
			replace: true,
			scrollToTop: false,
			search: "?tab=details",
			hash: "#top",
			state: { from: "conformance" },
		});
	});
});

describeNavigationTestSuite(() => {
	describe("UI adapter parity conformance", () => {
		it("FEC-LINK-005_FE-LINK-007_vorma_link_click_behavior_is_equivalent_across_react_preact_and_solid", async () => {
			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					importURLs: [],
					cssBundles: [],
				}),
			);

			const adapters = ["react", "preact", "solid"] as const;
			for (const adapter of adapters) {
				const callbacks: string[] = [];
				const props = {
					href: "/adapter-parity",
					prefetch: "intent" as const,
					beforeBegin: () => {
						callbacks.push("beforeBegin");
					},
					beforeRender: () => {
						callbacks.push("beforeRender");
					},
					afterRender: () => {
						callbacks.push("afterRender");
					},
				};

				if (adapter === "solid") {
					const rendered = renderSolidAnchor(() => SolidVormaLink(props));
					const e = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
					});
					rendered.anchor.dispatchEvent(e);
					await vi.runAllTimersAsync();
					expect(e.defaultPrevented).toBe(true);
					expect(callbacks).toEqual([
						"beforeBegin",
						"beforeRender",
						"afterRender",
					]);
					rendered.cleanup();
					continue;
				}

				const anchor = document.createElement("a");
				anchor.href = "/adapter-parity";
				document.body.appendChild(anchor);
				const rendered = getRenderedAnchorPropsForAdapter(adapter, props);
				const e = new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
				});
				Object.defineProperty(e, "target", { value: anchor });
				Object.defineProperty(e, "currentTarget", { value: anchor });

				await rendered.onClick(e);
				expect(e.defaultPrevented).toBe(true);
				expect(callbacks).toEqual([
					"beforeBegin",
					"beforeRender",
					"afterRender",
				]);
				document.body.removeChild(anchor);
			}
		});
	});
});
