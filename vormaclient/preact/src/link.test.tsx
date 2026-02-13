import { h, render } from "preact";
import { act } from "preact/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { makeFinalLinkPropsSpy, resolvePathSpy } = vi.hoisted(() => {
	return {
		makeFinalLinkPropsSpy: vi.fn(),
		resolvePathSpy: vi.fn(),
	};
});

vi.mock("vorma/client", () => {
	return {
		__makeFinalLinkProps: makeFinalLinkPropsSpy,
		__resolvePath: resolvePathSpy,
	};
});

import { VormaLink, makeTypedLink } from "./link.tsx";

describe("preact link adapter", () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		container = document.createElement("div");
		document.body.appendChild(container);
	});

	afterEach(() => {
		render(null, container);
		container.remove();
	});

	it("forwards final link handlers onto rendered anchors", async () => {
		const finalProps = {
			dataExternal: "external-link",
			onPointerEnter: vi.fn(),
			onFocus: vi.fn(),
			onPointerLeave: vi.fn(),
			onBlur: vi.fn(),
			onTouchCancel: vi.fn(),
			onClick: vi.fn(),
		};
		makeFinalLinkPropsSpy.mockReturnValue(finalProps);

		await act(async () => {
			render(
				h(VormaLink, {
					href: undefined,
					id: "docs-link",
					children: "Docs",
					prefetch: "intent",
					scrollToTop: false,
					replace: true,
					state: { from: "test" },
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		if (!anchor) {
			throw new Error("Expected link to render an anchor");
		}

		expect(makeFinalLinkPropsSpy).toHaveBeenCalledTimes(1);
		expect(anchor.getAttribute("data-external")).toBe("external-link");
		expect(anchor.id).toBe("docs-link");

		anchor.dispatchEvent(
			new MouseEvent("click", {
				bubbles: true,
			}),
		);
		expect(finalProps.onClick).toHaveBeenCalledTimes(1);
	});

	it("builds typed hrefs with search and hash", async () => {
		resolvePathSpy.mockReturnValue("/typed/path");
		makeFinalLinkPropsSpy.mockReturnValue({
			dataExternal: undefined,
			onPointerEnter: vi.fn(),
			onFocus: vi.fn(),
			onPointerLeave: vi.fn(),
			onBlur: vi.fn(),
			onTouchCancel: vi.fn(),
			onClick: vi.fn(),
		});

		const TypedLink = makeTypedLink({} as any, {
			className: "default-class",
		});
		await act(async () => {
			render(
				h(TypedLink as any, {
					pattern: "/typed/:id",
					params: { id: "42" },
					search: "?q=abc",
					hash: "#panel",
					state: { from: "typed-test" },
					className: "custom-class",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		if (!anchor) {
			throw new Error("Expected typed link to render an anchor");
		}

		expect(resolvePathSpy).toHaveBeenCalledTimes(1);
		expect(resolvePathSpy).toHaveBeenCalledWith({
			vormaAppConfig: {},
			type: "loader",
			props: {
				pattern: "/typed/:id",
				params: { id: "42" },
			},
		});
		expect(anchor.getAttribute("href")).toBe(
			`${window.location.origin}/typed/path?q=abc#panel`,
		);
		expect(anchor.className).toBe("custom-class");
	});
});
