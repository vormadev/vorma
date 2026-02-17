import { beforeEach, describe, expect, it } from "vitest";
import {
	buildTypedLinkDisplayName,
	buildTypedLinkHrefForRouteResolution,
	buildTypedLinkResolvedProps,
} from "./typed_link_props.ts";

const TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
};

describe("typed link props helpers", () => {
	beforeEach(() => {
		window.history.replaceState({}, "", "/");
	});

	it("builds href from route resolution input with params, search, and hash", () => {
		const href = buildTypedLinkHrefForRouteResolution({
			vormaAppConfig: TEST_VORMA_APP_CONFIG,
			routeResolutionInput: {
				pattern: "/users/:id",
				params: { id: "a/b" },
				search: "?tab=activity",
				hash: "#details",
			},
		});

		expect(href).toBe(
			`${window.location.origin}/users/a%2Fb?tab=activity#details`,
		);
	});

	it("resolves href and strips routing fields from merged link props", () => {
		const resolvedProps = buildTypedLinkResolvedProps({
			vormaAppConfig: TEST_VORMA_APP_CONFIG,
			mergedProps: {
				pattern: "/docs/*",
				splatValues: ["guides", "intro"],
				search: "?view=full",
				hash: "#top",
				state: { from: "sidebar" },
				className: "typed-link",
				prefetch: "intent" as const,
				replace: true,
				scrollToTop: false,
			},
		});

		expect(resolvedProps.href).toBe(
			`${window.location.origin}/docs/guides/intro?view=full#top`,
		);
		expect(resolvedProps.state).toEqual({ from: "sidebar" });
		expect(resolvedProps.linkProps).toEqual({
			className: "typed-link",
			prefetch: "intent",
			replace: true,
			scrollToTop: false,
		});
	});

	it("builds typed link display names from default prop keys", () => {
		expect(
			buildTypedLinkDisplayName({
				defaultProps: {
					className: "default-link",
					prefetch: "intent",
				},
			}),
		).toBe("TypedLink(className, prefetch)");

		expect(
			buildTypedLinkDisplayName({
				defaultProps: undefined,
			}),
		).toBe("TypedLink()");
	});
});
