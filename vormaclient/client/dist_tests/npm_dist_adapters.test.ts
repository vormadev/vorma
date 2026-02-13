import { h, render as renderPreact } from "preact";
import { act } from "preact/test-utils";
import { render as renderSolid } from "solid-js/web";
import { describe, expect, it, vi } from "vitest";
import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import {
	DIST_TEST_VORMA_APP_CONFIG,
	installDistTestVormaGlobal,
} from "./dist_test_harness.ts";

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

async function assertCompiledAdapterClientLoaderRegistration(
	adapterImportPath: "vorma/react" | "vorma/preact" | "vorma/solid",
	pattern: string,
	resolvedPath: string,
) {
	const globals = installDistTestVormaGlobal();
	vi.resetModules();
	const adapter = await import(adapterImportPath);

	const addClientLoader = adapter.makeTypedAddClientLoader();
	const waitFn = vi.fn(async () => "ok");

	const useClientLoaderData = addClientLoader({
		pattern: pattern as any,
		clientLoader: waitFn as any,
	});

	expect(typeof useClientLoaderData).toBe("function");
	expect(typeof globals.patternToWaitFnMap[pattern]).toBe("function");
	expect(
		findNestedMatches(globals.patternRegistry as any, resolvedPath),
	).not.toBeNull();
}

describe("npm_dist adapters", () => {
	it("react typed links resolve hrefs via compiled helpers", async () => {
		installDistTestVormaGlobal();
		vi.resetModules();
		const reactAdapter = await import("vorma/react");

		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				className: "default-class",
			},
		);
		const element = invokeReactMemoComponent(TypedLink, {
			pattern: "/products/:id",
			params: { id: "42" },
			search: "?q=abc",
			hash: "#panel",
			className: "custom-class",
		});

		expect((element as { props: Record<string, unknown> }).props.href).toBe(
			`${window.location.origin}/products/42?q=abc#panel`,
		);
		expect(
			(element as { props: Record<string, unknown> }).props.className,
		).toBe("custom-class");
	});

	it("preact typed links resolve hrefs via compiled helpers", async () => {
		installDistTestVormaGlobal();
		vi.resetModules();
		const preactAdapter = await import("vorma/preact");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				className: "default-class",
			},
		);
		await act(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/products/:id",
					params: { id: "42" },
					search: "?q=abc",
					hash: "#panel",
					className: "custom-class",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		expect(anchor?.getAttribute("href")).toBe(
			`${window.location.origin}/products/42?q=abc#panel`,
		);
		expect(anchor?.className).toBe("custom-class");

		renderPreact(null, container);
		container.remove();
	});

	it("solid typed links resolve hrefs via compiled helpers", async () => {
		installDistTestVormaGlobal();
		vi.resetModules();
		const solidAdapter = await import("vorma/solid");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				class: "default-class",
			},
		);
		const dispose = renderSolid(() => {
			return (TypedLink as any)({
				pattern: "/products/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
				class: "custom-class",
				children: "Products",
			});
		}, container);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		expect(anchor?.getAttribute("href")).toBe(
			`${window.location.origin}/products/42?q=abc#panel`,
		);
		expect(anchor?.getAttribute("class")).toBe("custom-class");

		dispose();
		container.remove();
	});

	it("react compiled helpers register client loaders into client runtime", async () => {
		await assertCompiledAdapterClientLoaderRegistration(
			"vorma/react",
			"/react/:id",
			"/react/42",
		);
	});

	it("preact compiled helpers register client loaders into client runtime", async () => {
		await assertCompiledAdapterClientLoaderRegistration(
			"vorma/preact",
			"/preact/:id",
			"/preact/42",
		);
	});

	it("solid compiled helpers register client loaders into client runtime", async () => {
		await assertCompiledAdapterClientLoaderRegistration(
			"vorma/solid",
			"/solid/:id",
			"/solid/42",
		);
	});
});
