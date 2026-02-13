import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { h, render as renderPreact } from "preact";
import { act as actPreact } from "preact/test-utils";
import { render as renderSolid } from "solid-js/web";
import { describe, expect, it, vi } from "vitest";
import {
	installDistTestVormaGlobal,
	type DistTestVormaInternal,
} from "./dist_test_harness.ts";

const ROUTE_CHANGE_EVENT_KEY = "vorma:route-change";

type RuntimeComponent = (props: { idx: number }) => unknown;

function applyRootOutletRuntimeState(
	globals: DistTestVormaInternal,
	component: RuntimeComponent,
	importURL: string,
): void {
	globals.matchedPatterns = ["/"];
	globals.loadersData = [{}];
	globals.clientLoadersData = [];
	globals.outermostError = undefined;
	globals.outermostErrorIdx = undefined;
	globals.activeComponents = [component];
	globals.activeErrorBoundary = undefined;
	globals.importURLs = [importURL];
	globals.exportKeys = ["default"];
}

function dispatchRouteChangeWithScrollState(): void {
	window.dispatchEvent(
		new CustomEvent(ROUTE_CHANGE_EVENT_KEY, {
			detail: {
				__scrollState: { x: 7, y: 11 },
			},
		}),
	);
}

async function waitForCondition(
	condition: () => void,
	maxTicks: number = 10,
): Promise<void> {
	let lastError: unknown;
	for (let i = 0; i < maxTicks; i++) {
		try {
			condition();
			return;
		} catch (error) {
			lastError = error;
			await Promise.resolve();
		}
	}
	throw lastError;
}

function installImmediateRAFAndScrollSpy() {
	const originalRAF = window.requestAnimationFrame;
	window.requestAnimationFrame = (callback) => {
		callback(0);
		return 0;
	};
	const scrollToSpy = vi
		.spyOn(window, "scrollTo")
		.mockImplementation(() => {});

	return {
		scrollToSpy,
		restore() {
			window.requestAnimationFrame = originalRAF;
			scrollToSpy.mockRestore();
		},
	};
}

describe("npm_dist adapter root outlets", () => {
	it("react root outlet syncs route-change updates from compiled runtime", async () => {
		const globals = installDistTestVormaGlobal();
		applyRootOutletRuntimeState(
			globals,
			(props) => React.createElement("div", {}, `first:${props.idx}`),
			"/react-first.js",
		);
		vi.resetModules();
		const reactAdapter = await import("vorma/react");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		try {
			await act(async () => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(container.textContent).toContain("first:0");

			applyRootOutletRuntimeState(
				globals,
				(props) =>
					React.createElement("div", {}, `second:${props.idx}`),
				"/react-second.js",
			);
			await act(async () => {
				dispatchRouteChangeWithScrollState();
			});

			expect(container.textContent).toContain("second:0");
			expect(scrollToSpy).toHaveBeenCalledWith(7, 11);
		} finally {
			await act(async () => {
				root.unmount();
			});
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT =
				originalActEnvironment;
			container.remove();
			restore();
		}
	});

	it("preact root outlet syncs route-change updates from compiled runtime", async () => {
		const globals = installDistTestVormaGlobal();
		applyRootOutletRuntimeState(
			globals,
			(props) => h("div", {}, `first:${props.idx}`),
			"/preact-first.js",
		);
		vi.resetModules();
		const preactAdapter = await import("vorma/preact");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		try {
			await actPreact(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					container,
				);
			});
			expect(container.textContent).toContain("first:0");

			applyRootOutletRuntimeState(
				globals,
				(props) => h("div", {}, `second:${props.idx}`),
				"/preact-second.js",
			);
			await actPreact(async () => {
				dispatchRouteChangeWithScrollState();
			});

			expect(container.textContent).toContain("second:0");
			expect(scrollToSpy).toHaveBeenCalledWith(7, 11);
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid root outlet syncs route-change updates from compiled runtime", async () => {
		const globals = installDistTestVormaGlobal();
		applyRootOutletRuntimeState(
			globals,
			(props) => `first:${props.idx}`,
			"/solid-first.js",
		);
		vi.resetModules();
		const solidAdapter = await import("vorma/solid");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const dispose = renderSolid(() => {
			return (solidAdapter.VormaRootOutlet as any)({
				idx: 0,
			});
		}, container);

		try {
			expect(container.textContent).toContain("first:0");

			applyRootOutletRuntimeState(
				globals,
				(props) => `second:${props.idx}`,
				"/solid-second.js",
			);
			dispatchRouteChangeWithScrollState();
			await waitForCondition(() => {
				expect(scrollToSpy).toHaveBeenCalledWith(7, 11);
			});
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});
});
