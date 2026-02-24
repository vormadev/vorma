import { h, render as renderPreact } from "preact";
import { act as actPreact } from "preact/test-utils";
import { render as renderSolid } from "solid-js/web";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { installDistTestVormaGlobal } from "./dist_test_harness.ts";

const {
	makeFinalLinkPropsSpy,
	resolveTypedAdapterLinkWithDefaultsSpy,
	registerTypedAdapterClientLoaderSpy,
} = vi.hoisted(() => {
	return {
		makeFinalLinkPropsSpy: vi.fn(),
		resolveTypedAdapterLinkWithDefaultsSpy: vi.fn(),
		registerTypedAdapterClientLoaderSpy: vi.fn(),
	};
});

vi.mock("vorma/client/__internal", async (importOriginal) => {
	const actual =
		(await importOriginal()) as typeof import("vorma/client/__internal");
	return {
		...actual,
		makeFinalLinkProps: makeFinalLinkPropsSpy,
		resolveTypedAdapterLinkWithDefaults:
			resolveTypedAdapterLinkWithDefaultsSpy,
		registerTypedAdapterClientLoader: registerTypedAdapterClientLoaderSpy,
	};
});

function invokeReactMemoComponent(props: {
	component: unknown;
	componentProps: unknown;
}) {
	const { component, componentProps } = props;
	if (typeof component === "function") {
		return (component as (nextProps: unknown) => unknown)(componentProps);
	}

	const maybeMemo = component as {
		type?: (nextProps: unknown) => unknown;
	};
	if (typeof maybeMemo.type === "function") {
		return maybeMemo.type(componentProps);
	}

	throw new Error("React component is not invokable");
}

const navigationOnlyLinkPropKeys = [
	"prefetch",
	"prefetchDelayMs",
	"beforeBegin",
	"beforeRender",
	"afterRender",
	"scrollToTop",
	"replace",
	"state",
] as const;

function buildMockTypedLinkResolvedProps(input: {
	defaultProps?: Record<string, unknown>;
	linkProps: Record<string, unknown>;
}) {
	const mergedProps = {
		...input.defaultProps,
		...input.linkProps,
	};
	const {
		pattern: _pattern,
		params: _params,
		splatValues: _splatValues,
		search,
		hash,
		state,
		...linkProps
	} = mergedProps;
	const searchPart = typeof search === "string" ? search : "";
	const hashPart = typeof hash === "string" ? hash : "";
	return {
		href: `${window.location.origin}/typed/path${searchPart}${hashPart}`,
		state,
		linkProps,
	};
}

function expectNavigationOnlyPropsAreStrippedFromPropsBag(props: {
	propsBag: Record<string, unknown>;
}) {
	for (const navigationOnlyLinkPropKey of navigationOnlyLinkPropKeys) {
		expect(props.propsBag[navigationOnlyLinkPropKey]).toBeUndefined();
	}
}

function expectNavigationOnlyPropsAreStrippedFromAnchor(props: {
	anchor: HTMLAnchorElement;
}) {
	for (const navigationOnlyLinkPropKey of navigationOnlyLinkPropKeys) {
		expect(
			props.anchor.getAttribute(navigationOnlyLinkPropKey.toLowerCase()),
		).toBeNull();
	}
}

describe("npm_dist adapter mocked helper/link contracts", () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		vi.resetModules();
		vi.clearAllMocks();
		installDistTestVormaGlobal();
		resolveTypedAdapterLinkWithDefaultsSpy.mockImplementation(
			buildMockTypedLinkResolvedProps,
		);
		registerTypedAdapterClientLoaderSpy.mockImplementation((props) => {
			return {
				pattern: props.pattern,
				clientLoader: props.clientLoader,
			};
		});
		container = document.createElement("div");
		document.body.appendChild(container);
	});

	afterEach(() => {
		renderPreact(null, container);
		container.remove();
	});

	it("react makeTypedAddClientLoader delegates registration through registerTypedAdapterClientLoader", async () => {
		const reactAdapter = await import("vorma/react");
		const addClientLoader = reactAdapter.makeTypedAddClientLoader();
		const clientLoader = vi.fn(async () => "ok");
		const reRunOnModuleChange = { url: "file:///tmp/mod.ts" } as ImportMeta;

		addClientLoader({
			pattern: "/dashboard" as any,
			clientLoader: clientLoader as any,
			reRunOnModuleChange,
		});

		expect(registerTypedAdapterClientLoaderSpy).toHaveBeenCalledTimes(1);
		expect(registerTypedAdapterClientLoaderSpy).toHaveBeenCalledWith({
			pattern: "/dashboard",
			clientLoader,
			reRunOnModuleChange,
		});
	});

	it("preact makeTypedAddClientLoader delegates registration through registerTypedAdapterClientLoader", async () => {
		const preactAdapter = await import("vorma/preact");
		const addClientLoader = preactAdapter.makeTypedAddClientLoader();
		const clientLoader = vi.fn(async () => "ok");
		const reRunOnModuleChange = { url: "file:///tmp/mod.ts" } as ImportMeta;

		addClientLoader({
			pattern: "/dashboard" as any,
			clientLoader: clientLoader as any,
			reRunOnModuleChange,
		});

		expect(registerTypedAdapterClientLoaderSpy).toHaveBeenCalledTimes(1);
		expect(registerTypedAdapterClientLoaderSpy).toHaveBeenCalledWith({
			pattern: "/dashboard",
			clientLoader,
			reRunOnModuleChange,
		});
	});

	it("solid makeTypedAddClientLoader delegates registration through registerTypedAdapterClientLoader", async () => {
		const solidAdapter = await import("vorma/solid");
		const addClientLoader = solidAdapter.makeTypedAddClientLoader();
		const clientLoader = vi.fn(async () => "ok");
		const reRunOnModuleChange = { url: "file:///tmp/mod.ts" } as ImportMeta;

		addClientLoader({
			pattern: "/dashboard" as any,
			clientLoader: clientLoader as any,
			reRunOnModuleChange,
		});

		expect(registerTypedAdapterClientLoaderSpy).toHaveBeenCalledTimes(1);
		expect(registerTypedAdapterClientLoaderSpy).toHaveBeenCalledWith({
			pattern: "/dashboard",
			clientLoader,
			reRunOnModuleChange,
		});
	});

	it("makeTypedAddClientLoader re-registers duplicate pattern calls without implicit dedupe", async () => {
		const adapterImportPaths = [
			"vorma/react",
			"vorma/preact",
			"vorma/solid",
		] as const;

		for (const adapterImportPath of adapterImportPaths) {
			vi.clearAllMocks();
			const adapter = await import(adapterImportPath);
			const addClientLoader = adapter.makeTypedAddClientLoader();
			const firstClientLoader = vi.fn(async () => "first");
			const secondClientLoader = vi.fn(async () => "second");

			addClientLoader({
				pattern: "/dashboard" as any,
				clientLoader: firstClientLoader as any,
			});
			addClientLoader({
				pattern: "/dashboard" as any,
				clientLoader: secondClientLoader as any,
			});

			expect(registerTypedAdapterClientLoaderSpy).toHaveBeenCalledTimes(
				2,
			);
			expect(registerTypedAdapterClientLoaderSpy).toHaveBeenNthCalledWith(
				1,
				{
					pattern: "/dashboard",
					clientLoader: firstClientLoader,
				},
			);
			expect(registerTypedAdapterClientLoaderSpy).toHaveBeenNthCalledWith(
				2,
				{
					pattern: "/dashboard",
					clientLoader: secondClientLoader,
				},
			);
		}
	});

	it("react VormaLink forwards final link handlers and strips navigation-only props", async () => {
		const reactAdapter = await import("vorma/react");
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
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

		const result = invokeReactMemoComponent({
			component: reactAdapter.VormaLink,
			componentProps: {
				href: "/docs",
				id: "docs-link",
				children: "Docs",
				prefetch: "intent",
				prefetchDelayMs: 75,
				beforeBegin,
				beforeRender,
				afterRender,
				scrollToTop: false,
				replace: true,
				state: { from: "test" },
			},
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
		expectNavigationOnlyPropsAreStrippedFromPropsBag({
			propsBag: element.props,
		});
	});

	it("react makeTypedLink builds href with search/hash and forwards state", async () => {
		const reactAdapter = await import("vorma/react");
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
		const result = invokeReactMemoComponent({
			component: TypedLink,
			componentProps: {
				pattern: "/typed/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
				state: { from: "typed-test" },
				className: "custom-class",
			},
		});
		const element = result as {
			props: Record<string, unknown>;
		};

		expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledTimes(1);
		expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledWith(
			expect.objectContaining({
				vormaAppConfig: {},
				defaultProps: expect.objectContaining({
					className: "default-class",
				}),
				linkProps: expect.objectContaining({
					pattern: "/typed/:id",
					params: { id: "42" },
					search: "?q=abc",
					hash: "#panel",
				}),
			}),
		);
		expect(element.props.href).toBe(
			`${window.location.origin}/typed/path?q=abc#panel`,
		);
		expect(element.props.state).toEqual({ from: "typed-test" });
		expect(element.props.className).toBe("custom-class");
	});

	it("react makeTypedLink applies default props, forwards splatValues, and sets displayName", async () => {
		const reactAdapter = await import("vorma/react");
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
		const result = invokeReactMemoComponent({
			component: TypedLink,
			componentProps: {
				pattern: "/typed/*",
				splatValues: ["docs/path"],
			},
		});
		const element = result as {
			props: Record<string, unknown>;
		};

		expect((TypedLink as any).displayName).toBe(
			"TypedLink(className, target)",
		);
		expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledWith(
			expect.objectContaining({
				vormaAppConfig: {},
				defaultProps: expect.objectContaining({
					className: "default-class",
					target: "_blank",
				}),
				linkProps: expect.objectContaining({
					pattern: "/typed/*",
					splatValues: ["docs/path"],
				}),
			}),
		);
		expect(element.props.className).toBe("default-class");
		expect(element.props.target).toBe("_blank");
	});

	it("react makeTypedLink preserves default state/search/hash when not provided by caller", async () => {
		const reactAdapter = await import("vorma/react");
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
			state: { from: "default-state" },
			search: "?default=true",
			hash: "#default",
		});
		const result = invokeReactMemoComponent({
			component: TypedLink,
			componentProps: {
				pattern: "/typed/:id",
				params: { id: "42" },
			},
		});
		const element = result as {
			props: Record<string, unknown>;
		};

		expect(element.props.href).toBe(
			`${window.location.origin}/typed/path?default=true#default`,
		);
		expect(element.props.state).toEqual({ from: "default-state" });
		expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledWith(
			expect.objectContaining({
				vormaAppConfig: {},
				defaultProps: expect.objectContaining({
					state: { from: "default-state" },
					search: "?default=true",
					hash: "#default",
				}),
				linkProps: expect.objectContaining({
					pattern: "/typed/:id",
					params: { id: "42" },
				}),
			}),
		);
	});

	it("preact VormaLink forwards final link handlers onto rendered anchors", async () => {
		const preactAdapter = await import("vorma/preact");
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
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
					prefetchDelayMs: 75,
					beforeBegin,
					beforeRender,
					afterRender,
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
		expectNavigationOnlyPropsAreStrippedFromAnchor({ anchor });

		anchor.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		expect(finalProps.onClick).toHaveBeenCalledTimes(1);
	});

	it("preact makeTypedLink builds href with search/hash and forwards class", async () => {
		const preactAdapter = await import("vorma/preact");
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
		expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledTimes(1);
		expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledWith(
			expect.objectContaining({
				vormaAppConfig: {},
				defaultProps: expect.objectContaining({
					className: "default-class",
				}),
				linkProps: expect.objectContaining({
					pattern: "/typed/:id",
					params: { id: "42" },
					search: "?q=abc",
					hash: "#panel",
				}),
			}),
		);
		expect(anchor.getAttribute("href")).toBe(
			`${window.location.origin}/typed/path?q=abc#panel`,
		);
		expect(anchor.className).toBe("custom-class");
	});

	it("preact makeTypedLink applies default props, forwards splatValues, and sets displayName", async () => {
		const preactAdapter = await import("vorma/preact");
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
		expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledWith(
			expect.objectContaining({
				vormaAppConfig: {},
				defaultProps: expect.objectContaining({
					className: "default-class",
					target: "_blank",
				}),
				linkProps: expect.objectContaining({
					pattern: "/typed/*",
					splatValues: ["docs/path"],
				}),
			}),
		);
		expect(anchor.className).toBe("default-class");
		expect(anchor.getAttribute("target")).toBe("_blank");
	});

	it("preact makeTypedLink preserves default state/search/hash when not provided by caller", async () => {
		const preactAdapter = await import("vorma/preact");
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
			state: { from: "default-state" },
			search: "?default=true",
			hash: "#default",
		});
		await actPreact(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/typed/:id",
					params: { id: "42" },
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		if (!anchor) {
			throw new Error("Expected typed link to render an anchor");
		}
		expect(anchor.getAttribute("href")).toBe(
			`${window.location.origin}/typed/path?default=true#default`,
		);
		expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledWith(
			expect.objectContaining({
				vormaAppConfig: {},
				defaultProps: expect.objectContaining({
					state: { from: "default-state" },
					search: "?default=true",
					hash: "#default",
				}),
				linkProps: expect.objectContaining({
					pattern: "/typed/:id",
					params: { id: "42" },
				}),
			}),
		);
		expect(makeFinalLinkPropsSpy).toHaveBeenCalled();
		expect(makeFinalLinkPropsSpy.mock.calls.at(-1)?.[0]).toMatchObject({
			state: { from: "default-state" },
		});
	});

	it("solid VormaLink forwards final link handlers onto rendered anchors", async () => {
		const solidAdapter = await import("vorma/solid");
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
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
				prefetchDelayMs: 75,
				beforeBegin,
				beforeRender,
				afterRender,
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
			expectNavigationOnlyPropsAreStrippedFromAnchor({ anchor });

			anchor.dispatchEvent(new MouseEvent("click", { bubbles: true }));
			expect(finalProps.onClick).toHaveBeenCalledTimes(1);
		} finally {
			dispose();
		}
	});

	it("solid makeTypedLink builds href with search/hash and forwards class/state", async () => {
		const solidAdapter = await import("vorma/solid");
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
			expect(
				resolveTypedAdapterLinkWithDefaultsSpy,
			).toHaveBeenCalledTimes(1);
			expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledWith(
				expect.objectContaining({
					vormaAppConfig: {},
					defaultProps: expect.objectContaining({
						class: "default-class",
					}),
					linkProps: expect.objectContaining({
						pattern: "/typed/:id",
						params: { id: "42" },
						search: "?q=abc",
						hash: "#panel",
					}),
				}),
			);
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
			expect(resolveTypedAdapterLinkWithDefaultsSpy).toHaveBeenCalledWith(
				expect.objectContaining({
					vormaAppConfig: {},
					defaultProps: expect.objectContaining({
						class: "default-class",
						target: "_blank",
					}),
					linkProps: expect.objectContaining({
						pattern: "/typed/*",
						splatValues: ["docs/path"],
					}),
				}),
			);
			expect(anchor.getAttribute("class")).toBe("default-class");
			expect(anchor.getAttribute("target")).toBe("_blank");
		} finally {
			dispose();
		}
	});
});
