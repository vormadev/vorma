import { h, render as renderPreact } from "preact";
import { act as actPreact } from "preact/test-utils";
import { render as renderSolid } from "solid-js/web";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { installDistTestVormaGlobal } from "./dist_test_harness.ts";

const {
	makeFinalLinkPropsSpy,
	resolvePathSpy,
	registerClientLoaderForAdapterSpy,
} = vi.hoisted(() => {
	return {
		makeFinalLinkPropsSpy: vi.fn(),
		resolvePathSpy: vi.fn(),
		registerClientLoaderForAdapterSpy: vi.fn(),
	};
});

vi.mock("vorma/client/__internal", async (importOriginal) => {
	const actual =
		(await importOriginal()) as typeof import("vorma/client/__internal");
	return {
		...actual,
		makeFinalLinkProps: makeFinalLinkPropsSpy,
		resolvePath: resolvePathSpy,
		registerClientLoaderForAdapter: registerClientLoaderForAdapterSpy,
	};
});

function invokeReactMemoComponent(component: unknown, props: unknown) {
	if (typeof component === "function") {
		return (component as (nextProps: unknown) => unknown)(props);
	}

	const maybeMemo = component as {
		type?: (nextProps: unknown) => unknown;
	};
	if (typeof maybeMemo.type === "function") {
		return maybeMemo.type(props);
	}

	throw new Error("React component is not invokable");
}

describe("npm_dist adapter mocked helper/link contracts", () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		vi.resetModules();
		vi.clearAllMocks();
		installDistTestVormaGlobal();
		container = document.createElement("div");
		document.body.appendChild(container);
	});

	afterEach(() => {
		renderPreact(null, container);
		container.remove();
	});

	it("react makeTypedAddClientLoader delegates registration through registerClientLoaderForAdapter", async () => {
		const reactAdapter = await import("vorma/react");
		const addClientLoader = reactAdapter.makeTypedAddClientLoader();
		const clientLoader = vi.fn(async () => "ok");
		const reRunOnModuleChange = { url: "file:///tmp/mod.ts" } as ImportMeta;

		addClientLoader({
			pattern: "/dashboard" as any,
			clientLoader: clientLoader as any,
			reRunOnModuleChange,
		});

		expect(registerClientLoaderForAdapterSpy).toHaveBeenCalledTimes(1);
		expect(registerClientLoaderForAdapterSpy).toHaveBeenCalledWith({
			pattern: "/dashboard",
			waitFn: clientLoader,
			reRunOnModuleChange,
		});
	});

	it("preact makeTypedAddClientLoader delegates registration through registerClientLoaderForAdapter", async () => {
		const preactAdapter = await import("vorma/preact");
		const addClientLoader = preactAdapter.makeTypedAddClientLoader();
		const clientLoader = vi.fn(async () => "ok");
		const reRunOnModuleChange = { url: "file:///tmp/mod.ts" } as ImportMeta;

		addClientLoader({
			pattern: "/dashboard" as any,
			clientLoader: clientLoader as any,
			reRunOnModuleChange,
		});

		expect(registerClientLoaderForAdapterSpy).toHaveBeenCalledTimes(1);
		expect(registerClientLoaderForAdapterSpy).toHaveBeenCalledWith({
			pattern: "/dashboard",
			waitFn: clientLoader,
			reRunOnModuleChange,
		});
	});

	it("solid makeTypedAddClientLoader delegates registration through registerClientLoaderForAdapter", async () => {
		const solidAdapter = await import("vorma/solid");
		const addClientLoader = solidAdapter.makeTypedAddClientLoader();
		const clientLoader = vi.fn(async () => "ok");
		const reRunOnModuleChange = { url: "file:///tmp/mod.ts" } as ImportMeta;

		addClientLoader({
			pattern: "/dashboard" as any,
			clientLoader: clientLoader as any,
			reRunOnModuleChange,
		});

		expect(registerClientLoaderForAdapterSpy).toHaveBeenCalledTimes(1);
		expect(registerClientLoaderForAdapterSpy).toHaveBeenCalledWith({
			pattern: "/dashboard",
			waitFn: clientLoader,
			reRunOnModuleChange,
		});
	});

	it("react VormaLink forwards final link handlers and strips navigation-only props", async () => {
		const reactAdapter = await import("vorma/react");
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

		const result = invokeReactMemoComponent(reactAdapter.VormaLink, {
			href: "/docs",
			id: "docs-link",
			children: "Docs",
			prefetch: "intent",
			scrollToTop: false,
			replace: true,
			state: { from: "test" },
		});
		const element = result as {
			props: Record<string, unknown>;
		};

		expect(makeFinalLinkPropsSpy).toHaveBeenCalledTimes(1);
		expect(element.props["data-external"]).toBe("external-link");
		expect(element.props.id).toBe("docs-link");
		expect(element.props.onPointerEnter).toBe(finalProps.onPointerEnter);
		expect(element.props.onFocus).toBe(finalProps.onFocus);
		expect(element.props.onPointerLeave).toBe(finalProps.onPointerLeave);
		expect(element.props.onBlur).toBe(finalProps.onBlur);
		expect(element.props.onTouchCancel).toBe(finalProps.onTouchCancel);
		expect(element.props.onClick).toBe(finalProps.onClick);
		expect(element.props.prefetch).toBeUndefined();
		expect(element.props.replace).toBeUndefined();
		expect(element.props.scrollToTop).toBeUndefined();
		expect(element.props.state).toBeUndefined();
	});

	it("react makeTypedLink builds href with search/hash and forwards state", async () => {
		const reactAdapter = await import("vorma/react");
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

		const TypedLink = reactAdapter.makeTypedLink({} as any, {
			className: "default-class",
		});
		const result = invokeReactMemoComponent(TypedLink, {
			pattern: "/typed/:id",
			params: { id: "42" },
			search: "?q=abc",
			hash: "#panel",
			state: { from: "typed-test" },
			className: "custom-class",
		});
		const element = result as {
			props: Record<string, unknown>;
		};

		expect(resolvePathSpy).toHaveBeenCalledTimes(1);
		expect(resolvePathSpy).toHaveBeenCalledWith({
			vormaAppConfig: {},
			type: "loader",
			props: {
				pattern: "/typed/:id",
				params: { id: "42" },
			},
		});
		expect(element.props.href).toBe(
			`${window.location.origin}/typed/path?q=abc#panel`,
		);
		expect(element.props.state).toEqual({ from: "typed-test" });
		expect(element.props.className).toBe("custom-class");
	});

	it("react makeTypedLink applies default props, forwards splatValues, and sets displayName", async () => {
		const reactAdapter = await import("vorma/react");
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

		const TypedLink = reactAdapter.makeTypedLink({} as any, {
			className: "default-class",
			target: "_blank",
		});
		const result = invokeReactMemoComponent(TypedLink, {
			pattern: "/typed/*",
			splatValues: ["docs/path"],
		});
		const element = result as {
			props: Record<string, unknown>;
		};

		expect((TypedLink as any).displayName).toBe(
			"TypedLink(className, target)",
		);
		expect(resolvePathSpy).toHaveBeenCalledWith({
			vormaAppConfig: {},
			type: "loader",
			props: {
				pattern: "/typed/*",
				splatValues: ["docs/path"],
			},
		});
		expect(element.props.className).toBe("default-class");
		expect(element.props.target).toBe("_blank");
	});

	it("preact VormaLink forwards final link handlers onto rendered anchors", async () => {
		const preactAdapter = await import("vorma/preact");
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

		await actPreact(async () => {
			renderPreact(
				h(preactAdapter.VormaLink as any, {
					href: "#docs",
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

		anchor.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		expect(finalProps.onClick).toHaveBeenCalledTimes(1);
	});

	it("preact makeTypedLink builds href with search/hash and forwards class", async () => {
		const preactAdapter = await import("vorma/preact");
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

		const TypedLink = preactAdapter.makeTypedLink({} as any, {
			className: "default-class",
		});
		await actPreact(async () => {
			renderPreact(
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

	it("preact makeTypedLink applies default props, forwards splatValues, and sets displayName", async () => {
		const preactAdapter = await import("vorma/preact");
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

		const TypedLink = preactAdapter.makeTypedLink({} as any, {
			className: "default-class",
			target: "_blank",
		});
		await actPreact(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/typed/*",
					splatValues: ["docs/path"],
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		if (!anchor) {
			throw new Error("Expected typed link to render an anchor");
		}
		expect((TypedLink as any).displayName).toBe(
			"TypedLink(className, target)",
		);
		expect(resolvePathSpy).toHaveBeenCalledWith({
			vormaAppConfig: {},
			type: "loader",
			props: {
				pattern: "/typed/*",
				splatValues: ["docs/path"],
			},
		});
		expect(anchor.className).toBe("default-class");
		expect(anchor.getAttribute("target")).toBe("_blank");
	});

	it("solid VormaLink forwards final link handlers onto rendered anchors", async () => {
		const solidAdapter = await import("vorma/solid");
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

		const dispose = renderSolid(() => {
			return (solidAdapter.VormaLink as any)({
				href: "#docs",
				id: "docs-link",
				children: "Docs",
				prefetch: "intent",
				scrollToTop: false,
				replace: true,
				state: { from: "test" },
			});
		}, container);

		try {
			const anchor = container.querySelector("a");
			if (!anchor) {
				throw new Error("Expected link to render an anchor");
			}
			expect(makeFinalLinkPropsSpy).toHaveBeenCalledTimes(1);
			expect(anchor.getAttribute("data-external")).toBe("external-link");
			expect(anchor.id).toBe("docs-link");

			anchor.dispatchEvent(new MouseEvent("click", { bubbles: true }));
			expect(finalProps.onClick).toHaveBeenCalledTimes(1);
		} finally {
			dispose();
		}
	});

	it("solid makeTypedLink builds href with search/hash and forwards class/state", async () => {
		const solidAdapter = await import("vorma/solid");
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

		const TypedLink = solidAdapter.makeTypedLink({} as any, {
			class: "default-class",
		});
		const dispose = renderSolid(() => {
			return (TypedLink as any)({
				pattern: "/typed/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
				state: { from: "typed-test" },
				class: "custom-class",
				children: "Docs",
			});
		}, container);

		try {
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
			expect(anchor.getAttribute("class")).toBe("custom-class");
		} finally {
			dispose();
		}
	});

	it("solid makeTypedLink applies default props and forwards splatValues", async () => {
		const solidAdapter = await import("vorma/solid");
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

		const TypedLink = solidAdapter.makeTypedLink({} as any, {
			class: "default-class",
			target: "_blank",
		});
		const dispose = renderSolid(() => {
			return (TypedLink as any)({
				pattern: "/typed/*",
				splatValues: ["docs/path"],
				children: "Docs",
			});
		}, container);

		try {
			const anchor = container.querySelector("a");
			if (!anchor) {
				throw new Error("Expected typed link to render an anchor");
			}
			expect(resolvePathSpy).toHaveBeenCalledWith({
				vormaAppConfig: {},
				type: "loader",
				props: {
					pattern: "/typed/*",
					splatValues: ["docs/path"],
				},
			});
			expect(anchor.getAttribute("class")).toBe("default-class");
			expect(anchor.getAttribute("target")).toBe("_blank");
		} finally {
			dispose();
		}
	});
});
