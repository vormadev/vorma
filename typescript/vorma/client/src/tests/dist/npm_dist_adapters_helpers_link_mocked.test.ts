import { createComponent } from "solid-js";
import { h, render as renderPreact } from "preact";
import { render as renderSolid } from "solid-js/web";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	DIST_TEST_VORMA_APP_CONFIG,
	installDistTestVormaGlobal,
	type DistTestVormaInternal,
} from "./dist_test_harness.ts";

type DistAdapterImportPath = "vorma/react" | "vorma/preact" | "vorma/solid";

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

function getPatternToWaitFnMap(
	globals: DistTestVormaInternal,
): Record<string, unknown> {
	return globals.patternToWaitFnMap as Record<string, unknown>;
}

async function importAdapter(path: DistAdapterImportPath) {
	return import(path);
}

describe("npm_dist adapter link/client-loader contracts", () => {
	let container: HTMLDivElement;
	let globals: DistTestVormaInternal;

	beforeEach(() => {
		vi.resetModules();
		globals = installDistTestVormaGlobal();
		container = document.createElement("div");
		document.body.appendChild(container);
	});

	afterEach(() => {
		renderPreact(null, container);
		container.remove();
	});

	it("makeTypedAddClientLoader registers a wait function for each adapter", async () => {
		const adapterImportPaths: ReadonlyArray<DistAdapterImportPath> = [
			"vorma/react",
			"vorma/preact",
			"vorma/solid",
		];

		for (const adapterImportPath of adapterImportPaths) {
			globals.patternToWaitFnMap = {};
			const adapter = await importAdapter(adapterImportPath);
			const addClientLoader = adapter.makeTypedAddClientLoader(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const useClientLoaderData = addClientLoader({
				pattern: "/dashboard" as any,
				clientLoader: async () => "ok",
			});

			expect(typeof useClientLoaderData).toBe("function");
			expect(typeof getPatternToWaitFnMap(globals)["/dashboard"]).toBe(
				"function",
			);
		}
	});

	it("makeTypedAddClientLoader overwrites duplicate pattern registrations with latest function", async () => {
		const adapterImportPaths: ReadonlyArray<DistAdapterImportPath> = [
			"vorma/react",
			"vorma/preact",
			"vorma/solid",
		];

		for (const adapterImportPath of adapterImportPaths) {
			globals.patternToWaitFnMap = {};
			const adapter = await importAdapter(adapterImportPath);
			const addClientLoader = adapter.makeTypedAddClientLoader(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			addClientLoader({
				pattern: "/dashboard" as any,
				clientLoader: async () => "first",
			});
			const firstRegistration =
				getPatternToWaitFnMap(globals)["/dashboard"];

			addClientLoader({
				pattern: "/dashboard" as any,
				clientLoader: async () => "second",
			});
			const secondRegistration =
				getPatternToWaitFnMap(globals)["/dashboard"];

			expect(typeof firstRegistration).toBe("function");
			expect(typeof secondRegistration).toBe("function");
			expect(secondRegistration).not.toBe(firstRegistration);
		}
	});

	it("react VormaLink strips navigation-only props from rendered anchor props", async () => {
		const reactAdapter = await import("vorma/react");
		const result = invokeReactMemoComponent({
			component: reactAdapter.VormaLink,
			componentProps: {
				href: "/docs",
				id: "docs-link",
				children: "Docs",
				prefetch: "intent",
				prefetchDelayMs: 75,
				beforeBegin: vi.fn(),
				beforeRender: vi.fn(),
				afterRender: vi.fn(),
				scrollToTop: false,
				replace: true,
				state: { from: "test" },
			},
		});
		const element = result as {
			props: Record<string, unknown>;
		};

		expect(element.props.id).toBe("docs-link");
		expect(element.props.href).toBe("/docs");
		expect(typeof element.props.onClick).toBe("function");
		expectNavigationOnlyPropsAreStrippedFromPropsBag({
			propsBag: element.props,
		});
	});

	it("preact VormaLink strips navigation-only props from rendered anchor attributes", async () => {
		const preactAdapter = await import("vorma/preact");

		renderPreact(
			h(preactAdapter.VormaLink as any, {
				href: "/docs",
				id: "docs-link",
				children: "Docs",
				prefetch: "intent",
				prefetchDelayMs: 75,
				beforeBegin: vi.fn(),
				beforeRender: vi.fn(),
				afterRender: vi.fn(),
				scrollToTop: false,
				replace: true,
				state: { from: "test" },
			}),
			container,
		);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		if (!anchor) {
			throw new Error("Expected link to render an anchor");
		}
		expect(anchor.getAttribute("id")).toBe("docs-link");
		expect(anchor.getAttribute("href")).toBe("/docs");
		expectNavigationOnlyPropsAreStrippedFromAnchor({
			anchor,
		});
	});

	it("solid VormaLink strips navigation-only props from rendered anchor attributes", async () => {
		const solidAdapter = await import("vorma/solid");
		const dispose = renderSolid(() => {
			return createComponent(solidAdapter.VormaLink as any, {
				href: "/docs",
				id: "docs-link",
				children: "Docs",
				prefetch: "intent",
				prefetchDelayMs: 75,
				beforeBegin: vi.fn(),
				beforeRender: vi.fn(),
				afterRender: vi.fn(),
				scrollToTop: false,
				replace: true,
				state: { from: "test" },
			});
		}, container);

		try {
			const anchor = container.querySelector("a");
			expect(anchor).not.toBeNull();
			if (!anchor) {
				throw new Error("Expected link to render an anchor");
			}
			expect(anchor.getAttribute("id")).toBe("docs-link");
			expect(anchor.getAttribute("href")).toBe("/docs");
			expectNavigationOnlyPropsAreStrippedFromAnchor({
				anchor,
			});
		} finally {
			dispose();
		}
	});

	it("react makeTypedLink resolves href and merges default/override props", async () => {
		const reactAdapter = await import("vorma/react");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				className: "default-class",
			},
		);
		const customResult = invokeReactMemoComponent({
			component: TypedLink,
			componentProps: {
				pattern: "/typed/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
				state: { from: "typed-test" },
				className: "custom-class",
			},
		}) as { props: Record<string, unknown> };

		expect(customResult.props.href).toBe(
			`${window.location.origin}/typed/42?q=abc#panel`,
		);
		expect(customResult.props.className).toBe("custom-class");

		const defaultResult = invokeReactMemoComponent({
			component: TypedLink,
			componentProps: {
				pattern: "/typed/:id",
				params: { id: "99" },
			},
		}) as { props: Record<string, unknown> };
		expect(defaultResult.props.className).toBe("default-class");
	});

	it("preact makeTypedLink resolves href and merges default/override props", async () => {
		const preactAdapter = await import("vorma/preact");
		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				className: "default-class",
			},
		);

		renderPreact(
			h(TypedLink as any, {
				pattern: "/typed/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
				state: { from: "typed-test" },
				className: "custom-class",
				children: "Typed",
			}),
			container,
		);
		const customAnchor = container.querySelector("a");
		expect(customAnchor).not.toBeNull();
		if (!customAnchor) {
			throw new Error("Expected typed link to render an anchor");
		}
		expect(customAnchor.getAttribute("href")).toBe(
			`${window.location.origin}/typed/42?q=abc#panel`,
		);
		expect(customAnchor.getAttribute("class")).toBe("custom-class");

		renderPreact(
			h(TypedLink as any, {
				pattern: "/typed/:id",
				params: { id: "99" },
				children: "Typed",
			}),
			container,
		);
		const defaultAnchor = container.querySelector("a");
		expect(defaultAnchor).not.toBeNull();
		if (!defaultAnchor) {
			throw new Error("Expected typed link to render an anchor");
		}
		expect(defaultAnchor.getAttribute("class")).toBe("default-class");
	});

	it("solid makeTypedLink resolves href and merges default/override props", async () => {
		const solidAdapter = await import("vorma/solid");
		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				className: "default-class",
			},
		);

		let dispose = renderSolid(() => {
			return createComponent(TypedLink as any, {
				pattern: "/typed/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
				state: { from: "typed-test" },
				className: "custom-class",
				children: "Typed",
			});
		}, container);
		try {
			const customAnchor = container.querySelector("a");
			expect(customAnchor).not.toBeNull();
			if (!customAnchor) {
				throw new Error("Expected typed link to render an anchor");
			}
			expect(customAnchor.getAttribute("href")).toBe(
				`${window.location.origin}/typed/42?q=abc#panel`,
			);
			expect(customAnchor.getAttribute("class")).toBe("custom-class");
		} finally {
			dispose();
		}

		dispose = renderSolid(() => {
			return createComponent(TypedLink as any, {
				pattern: "/typed/:id",
				params: { id: "99" },
				children: "Typed",
			});
		}, container);
		try {
			const defaultAnchor = container.querySelector("a");
			expect(defaultAnchor).not.toBeNull();
			if (!defaultAnchor) {
				throw new Error("Expected typed link to render an anchor");
			}
			expect(defaultAnchor.getAttribute("class")).toBe("default-class");
		} finally {
			dispose();
		}
	});
});
