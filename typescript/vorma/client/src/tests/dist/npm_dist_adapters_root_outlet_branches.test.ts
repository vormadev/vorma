import { h, render as renderPreact } from "preact";
import { act as actPreact } from "preact/test-utils";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { createComponent } from "solid-js";
import { render as renderSolid } from "solid-js/web";
import { describe, expect, it, vi } from "vitest";
import {
	installDistTestVormaGlobal,
	patchDistRuntimeRouteSnapshot,
	type DistTestVormaInternal,
} from "./dist_test_harness.ts";

const ROUTE_CHANGE_EVENT_KEY = "vorma:route-change";
const LOCATION_EVENT_KEY = "vorma:location";

function dispatchRouteChange() {
	window.dispatchEvent(
		new CustomEvent(ROUTE_CHANGE_EVENT_KEY, {
			detail: {
				__scrollState: { x: 0, y: 0 },
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
	const originalAddEventListener = window.addEventListener.bind(window);
	const trackedListeners: Array<{
		type: string;
		listener: EventListenerOrEventListenerObject;
		options?: boolean | AddEventListenerOptions;
	}> = [];

	window.addEventListener = ((
		type: string,
		listener: EventListenerOrEventListenerObject,
		options?: boolean | AddEventListenerOptions,
	) => {
		if (type === ROUTE_CHANGE_EVENT_KEY || type === LOCATION_EVENT_KEY) {
			trackedListeners.push({
				type,
				listener,
				options,
			});
		}
		originalAddEventListener(type, listener, options);
	}) as typeof window.addEventListener;

	window.requestAnimationFrame = (callback) => {
		callback(0);
		return 0;
	};
	const scrollToSpy = vi
		.spyOn(window, "scrollTo")
		.mockImplementation(() => {});

	return {
		restore() {
			window.requestAnimationFrame = originalRAF;
			window.addEventListener =
				originalAddEventListener as typeof window.addEventListener;
			trackedListeners.forEach(({ type, listener, options }) => {
				window.removeEventListener(type, listener, options);
			});
			scrollToSpy.mockRestore();
		},
	};
}

function applyRuntimeState(
	globals: DistTestVormaInternal,
	props: {
		activeComponents: unknown[] | null;
		importURLs: string[];
		exportKeys: string[];
		loadersData: unknown[];
		outermostError?: unknown;
		outermostErrorIdx?: number | undefined;
		activeErrorBoundary?: unknown;
	},
) {
	const nextMatchedPatterns = ["/", "/child"];
	const nextClientLoadersData: Array<unknown> = [];
	patchDistRuntimeRouteSnapshot({
		globals,
		patch: {
			matchedPatterns: nextMatchedPatterns,
			clientLoadersData: nextClientLoadersData,
			activeComponents: props.activeComponents,
			importURLs: props.importURLs,
			exportKeys: props.exportKeys,
			loadersData: props.loadersData,
			outermostError: props.outermostError,
			outermostErrorIdx: props.outermostErrorIdx,
			activeErrorBoundary: props.activeErrorBoundary,
		},
	});
}

describe("npm_dist root outlet branch coverage", () => {
	it("react covers fallback, empty, custom error boundary, and default error branches", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/react");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		const FallbackChild = (props: { idx: number }) =>
			React.createElement("div", {}, `fallback-child:${props.idx}`);
		const ErrorBoundary = (props: { error: unknown }) =>
			React.createElement("div", {}, `handled:${String(props.error)}`);

		applyRuntimeState(globals, {
			activeComponents: [undefined, FallbackChild],
			importURLs: ["/missing-root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(container.textContent).toContain("fallback-child:1");

			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/missing-root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
			});
			await act(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toBe("");

			applyRuntimeState(globals, {
				activeComponents: [FallbackChild],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom",
				outermostErrorIdx: 0,
				activeErrorBoundary: ErrorBoundary,
			});
			await act(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("handled:boom");
			expect(container.textContent).not.toContain("fallback-child");

			applyRuntimeState(globals, {
				activeComponents: [FallbackChild],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: undefined,
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			await act(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("Error: unknown");
			expect(container.textContent).not.toContain("fallback-child");

			applyRuntimeState(globals, {
				activeComponents: [FallbackChild],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom-explicit",
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			await act(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("Error: boom-explicit");
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

	it("preact covers fallback, empty, custom error boundary, and default error branches", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const FallbackChild = (props: { idx: number }) =>
			h("div", {}, `fallback-child:${props.idx}`);
		const ErrorBoundary = (props: { error: unknown }) =>
			h("div", {}, `handled:${String(props.error)}`);

		applyRuntimeState(globals, {
			activeComponents: [undefined, FallbackChild],
			importURLs: ["/missing-root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					container,
				);
			});
			expect(container.textContent).toContain("fallback-child:1");

			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/missing-root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
			});
			await actPreact(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toBe("");

			applyRuntimeState(globals, {
				activeComponents: [FallbackChild],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom",
				outermostErrorIdx: 0,
				activeErrorBoundary: ErrorBoundary,
			});
			await actPreact(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("handled:boom");
			expect(container.textContent).not.toContain("fallback-child");

			applyRuntimeState(globals, {
				activeComponents: [FallbackChild],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: undefined,
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			await actPreact(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("Error: unknown");
			expect(container.textContent).not.toContain("fallback-child");

			applyRuntimeState(globals, {
				activeComponents: [FallbackChild],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom-explicit",
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			await actPreact(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("Error: boom-explicit");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid covers fallback, empty, custom error boundary, and default error branches", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const FallbackChild = (props: { idx: number }) =>
			`fallback-child:${props.idx}`;
		const ErrorBoundary = (props: { error: unknown }) =>
			`handled:${String(props.error)}`;

		applyRuntimeState(globals, {
			activeComponents: [undefined, FallbackChild],
			importURLs: ["/missing-root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
		});

		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			expect(container.textContent).toContain("fallback-child:1");

			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/missing-root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
			});
			dispatchRouteChange();
			await waitForCondition(() => {
				expect(container.textContent).toBe("");
			});

			applyRuntimeState(globals, {
				activeComponents: [FallbackChild],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom",
				outermostErrorIdx: 0,
				activeErrorBoundary: ErrorBoundary,
			});
			dispatchRouteChange();
			await waitForCondition(() => {
				expect(container.textContent).toContain("handled:boom");
			});
			expect(container.textContent).not.toContain("fallback-child");

			applyRuntimeState(globals, {
				activeComponents: [FallbackChild],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: undefined,
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			dispatchRouteChange();
			await waitForCondition(() => {
				expect(container.textContent).toContain("Error: unknown");
			});
			expect(container.textContent).not.toContain("fallback-child");

			applyRuntimeState(globals, {
				activeComponents: [FallbackChild],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom-explicit",
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			dispatchRouteChange();
			await waitForCondition(() => {
				expect(container.textContent).toContain("Error: boom-explicit");
			});
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("react renders isolated custom and default error branches", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/react");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		const ErrorBoundary = (props: { error: unknown }) =>
			React.createElement("div", {}, `handled:${String(props.error)}`);

		try {
			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom",
				outermostErrorIdx: 0,
				activeErrorBoundary: ErrorBoundary,
			});
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(container.textContent).toContain("handled:boom");

			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: undefined,
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			await act(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("Error: unknown");

			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom-explicit",
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			await act(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("Error: boom-explicit");
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

	it("preact renders isolated custom and default error branches", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const ErrorBoundary = (props: { error: unknown }) =>
			h("div", {}, `handled:${String(props.error)}`);

		try {
			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom",
				outermostErrorIdx: 0,
				activeErrorBoundary: ErrorBoundary,
			});
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					container,
				);
			});
			expect(container.textContent).toContain("handled:boom");

			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: undefined,
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			await actPreact(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("Error: unknown");

			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom-explicit",
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			await actPreact(async () => {
				dispatchRouteChange();
			});
			expect(container.textContent).toContain("Error: boom-explicit");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid renders isolated custom and default error branches", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const ErrorBoundary = (props: { error: unknown }) =>
			`handled:${String(props.error)}`;

		applyRuntimeState(globals, {
			activeComponents: [undefined],
			importURLs: ["/error.js"],
			exportKeys: ["default"],
			loadersData: [{}],
			outermostError: "boom",
			outermostErrorIdx: 0,
			activeErrorBoundary: ErrorBoundary,
		});

		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			expect(container.textContent).toContain("handled:boom");

			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: undefined,
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			dispatchRouteChange();
			await waitForCondition(() => {
				expect(container.textContent).toContain("Error: unknown");
			});

			applyRuntimeState(globals, {
				activeComponents: [undefined],
				importURLs: ["/error.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				outermostError: "boom-explicit",
				outermostErrorIdx: 0,
				activeErrorBoundary: undefined,
			});
			dispatchRouteChange();
			await waitForCondition(() => {
				expect(container.textContent).toContain("Error: boom-explicit");
			});
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("react renders child-level error branch at idx=1 while keeping parent output", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/react");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		const ParentComp = (props: { Outlet: any }) =>
			React.createElement(
				"section",
				{},
				React.createElement(
					"div",
					{ "data-parent": true },
					"parent-node",
				),
				React.createElement(props.Outlet, {}),
			);
		const ChildComp = () => React.createElement("div", {}, "child-node");
		const ErrorBoundary = (props: { error: unknown }) =>
			React.createElement("div", {}, `handled:${String(props.error)}`);

		applyRuntimeState(globals, {
			activeComponents: [ParentComp, ChildComp],
			importURLs: ["/parent.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
			outermostError: "child-boom",
			outermostErrorIdx: 1,
			activeErrorBoundary: ErrorBoundary,
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(container.querySelector("[data-parent]")?.textContent).toBe(
				"parent-node",
			);
			expect(container.textContent).toContain("handled:child-boom");
			expect(container.textContent).not.toContain("child-node");
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

	it("preact renders child-level error branch at idx=1 while keeping parent output", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const ParentComp = (props: { Outlet: any }) =>
			h(
				"section",
				{},
				h("div", { "data-parent": true }, "parent-node"),
				h(props.Outlet, {}),
			);
		const ChildComp = () => h("div", {}, "child-node");
		const ErrorBoundary = (props: { error: unknown }) =>
			h("div", {}, `handled:${String(props.error)}`);

		applyRuntimeState(globals, {
			activeComponents: [ParentComp, ChildComp],
			importURLs: ["/parent.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
			outermostError: "child-boom",
			outermostErrorIdx: 1,
			activeErrorBoundary: ErrorBoundary,
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					container,
				);
			});
			expect(container.querySelector("[data-parent]")?.textContent).toBe(
				"parent-node",
			);
			expect(container.textContent).toContain("handled:child-boom");
			expect(container.textContent).not.toContain("child-node");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid renders child-level error branch at idx=1 while keeping parent output", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const ParentComp = (props: { Outlet: any }) => {
			const section = document.createElement("section");
			const parent = document.createElement("div");
			parent.setAttribute("data-parent", "true");
			parent.textContent = "parent-node";
			section.appendChild(parent);
			return [section, createComponent(props.Outlet as any, {})];
		};
		const ChildComp = () => "child-node";
		const ErrorBoundary = (props: { error: unknown }) =>
			`handled:${String(props.error)}`;

		applyRuntimeState(globals, {
			activeComponents: [ParentComp, ChildComp],
			importURLs: ["/parent.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
			outermostError: "child-boom",
			outermostErrorIdx: 1,
			activeErrorBoundary: ErrorBoundary,
		});

		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			expect(container.querySelector("[data-parent]")?.textContent).toBe(
				"parent-node",
			);
			expect(container.textContent).toContain("handled:child-boom");
			expect(container.textContent).not.toContain("child-node");
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});
});
