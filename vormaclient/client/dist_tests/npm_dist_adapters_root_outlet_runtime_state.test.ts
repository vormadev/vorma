import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { h, render as renderPreact } from "preact";
import { act as actPreact } from "preact/test-utils";
import { createComponent, createEffect } from "solid-js";
import { render as renderSolid } from "solid-js/web";
import { afterAll, afterEach, describe, expect, it, vi } from "vitest";
import {
	installDistTestVormaGlobal,
	type DistTestVormaInternal,
} from "./dist_test_harness.ts";

const ROUTE_CHANGE_EVENT_KEY = "vorma:route-change";
const LOCATION_EVENT_KEY = "vorma:location";
const addWindowEventListenerNative = window.addEventListener.bind(window);
const removeWindowEventListenerNative = window.removeEventListener.bind(window);
const trackedRouteAndLocationListeners: Array<{
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
		trackedRouteAndLocationListeners.push({
			type,
			listener,
			options,
		});
	}
	addWindowEventListenerNative(type, listener, options);
}) as typeof window.addEventListener;

afterEach(() => {
	trackedRouteAndLocationListeners.forEach(({ type, listener, options }) => {
		removeWindowEventListenerNative(type, listener, options);
	});
	trackedRouteAndLocationListeners.length = 0;
});

afterAll(() => {
	window.addEventListener =
		addWindowEventListenerNative as typeof window.addEventListener;
});

type RuntimeComponent = (props: { idx: number; Outlet: any }) => unknown;

function applyRuntimeState(
	globals: DistTestVormaInternal,
	props: {
		activeComponents: Array<RuntimeComponent | undefined> | null;
		importURLs: string[];
		exportKeys: string[];
		loadersData?: unknown[];
		clientLoadersData?: unknown[];
		matchedPatterns?: string[];
	},
): void {
	globals.matchedPatterns = props.matchedPatterns ?? ["/"];
	globals.loadersData = props.loadersData ?? [{}];
	globals.clientLoadersData = props.clientLoadersData ?? [];
	globals.outermostError = undefined;
	globals.outermostErrorIdx = undefined;
	globals.activeErrorBoundary = undefined;
	globals.activeComponents = props.activeComponents;
	globals.importURLs = props.importURLs;
	globals.exportKeys = props.exportKeys;
}

function dispatchRouteChange(scrollState: { x: number; y: number }) {
	window.dispatchEvent(
		new CustomEvent(ROUTE_CHANGE_EVENT_KEY, {
			detail: {
				__scrollState: scrollState,
			},
		}),
	);
}

function dispatchLocationChange() {
	window.dispatchEvent(new CustomEvent(LOCATION_EVENT_KEY));
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

function countAddEventListenerCalls(
	addEventListenerSpy: ReturnType<typeof vi.spyOn>,
	eventType: string,
): number {
	return addEventListenerSpy.mock.calls.filter((args: unknown[]) => {
		return args[0] === eventType;
	}).length;
}

function makeReactOutlet(label: string) {
	return (props: { idx: number }) =>
		React.createElement("div", {}, `${label}:${props.idx}`);
}

function makePreactOutlet(label: string) {
	return (props: { idx: number }) => h("div", {}, `${label}:${props.idx}`);
}

function makeSolidOutlet(label: string) {
	return (props: { idx: number }) => `${label}:${props.idx}`;
}

describe("npm_dist root outlet runtime state coverage", () => {
	it("react root outlet initializes listeners once across remounts", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/react");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const { restore } = installImmediateRAFAndScrollSpy();
		const containerA = document.createElement("div");
		const containerB = document.createElement("div");
		document.body.appendChild(containerA);
		document.body.appendChild(containerB);
		const rootA = createRoot(containerA);
		const rootB = createRoot(containerB);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		applyRuntimeState(globals, {
			activeComponents: [makeReactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});

		try {
			await act(async () => {
				rootA.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			await act(async () => {
				rootA.unmount();
			});
			await act(async () => {
				rootB.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});

			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					ROUTE_CHANGE_EVENT_KEY,
				),
			).toBe(1);
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					LOCATION_EVENT_KEY,
				),
			).toBe(1);
		} finally {
			await act(async () => {
				rootB.unmount();
			});
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT =
				originalActEnvironment;
			restore();
			addEventListenerSpy.mockRestore();
			containerA.remove();
			containerB.remove();
		}
	});

	it("preact root outlet initializes listeners once across remounts", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const { restore } = installImmediateRAFAndScrollSpy();
		const containerA = document.createElement("div");
		const containerB = document.createElement("div");
		document.body.appendChild(containerA);
		document.body.appendChild(containerB);

		applyRuntimeState(globals, {
			activeComponents: [makePreactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					containerA,
				);
			});
			await actPreact(async () => {
				renderPreact(null, containerA);
			});
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					containerB,
				);
			});

			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					ROUTE_CHANGE_EVENT_KEY,
				),
			).toBe(1);
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					LOCATION_EVENT_KEY,
				),
			).toBe(1);
		} finally {
			renderPreact(null, containerB);
			restore();
			addEventListenerSpy.mockRestore();
			containerA.remove();
			containerB.remove();
		}
	});

	it("solid root outlet initializes listeners once across remounts", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const { restore } = installImmediateRAFAndScrollSpy();
		const containerA = document.createElement("div");
		const containerB = document.createElement("div");
		document.body.appendChild(containerA);
		document.body.appendChild(containerB);

		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});

		const disposeA = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 0 });
		}, containerA);
		disposeA();

		const disposeB = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 0 });
		}, containerB);

		try {
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					ROUTE_CHANGE_EVENT_KEY,
				),
			).toBe(1);
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					LOCATION_EVENT_KEY,
				),
			).toBe(1);
		} finally {
			disposeB();
			restore();
			addEventListenerSpy.mockRestore();
			containerA.remove();
			containerB.remove();
		}
	});

	it("react root outlet defaults idx to 0 when idx is omitted", async () => {
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

		applyRuntimeState(globals, {
			activeComponents: [makeReactOutlet("root-default-idx")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any),
				);
			});
			expect(container.textContent).toContain("root-default-idx:0");
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

	it("preact root outlet defaults idx to 0 when idx is omitted", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		applyRuntimeState(globals, {
			activeComponents: [makePreactOutlet("root-default-idx")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});

		try {
			await actPreact(async () => {
				renderPreact(h(adapter.VormaRootOutlet as any, {}), container);
			});
			expect(container.textContent).toContain("root-default-idx:0");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid root outlet defaults idx to 0 when idx is omitted", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root-default-idx")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});

		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, {});
		}, container);

		try {
			expect(container.textContent).toContain("root-default-idx:0");
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("react idx>0 outlet does not initialize listeners or apply root scroll logic", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/react");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		applyRuntimeState(globals, {
			activeComponents: [undefined, makeReactOutlet("child")],
			importURLs: ["/root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 1,
					}),
				);
			});
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					ROUTE_CHANGE_EVENT_KEY,
				),
			).toBe(0);
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					LOCATION_EVENT_KEY,
				),
			).toBe(0);
		} finally {
			await act(async () => {
				root.unmount();
			});
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT =
				originalActEnvironment;
			addEventListenerSpy.mockRestore();
			container.remove();
		}
	});

	it("preact idx>0 outlet does not initialize listeners or apply root scroll logic", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const container = document.createElement("div");
		document.body.appendChild(container);

		applyRuntimeState(globals, {
			activeComponents: [undefined, makePreactOutlet("child")],
			importURLs: ["/root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 1 }),
					container,
				);
			});
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					ROUTE_CHANGE_EVENT_KEY,
				),
			).toBe(0);
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					LOCATION_EVENT_KEY,
				),
			).toBe(0);
		} finally {
			renderPreact(null, container);
			addEventListenerSpy.mockRestore();
			container.remove();
		}
	});

	it("solid idx>0 outlet does not initialize listeners or apply root scroll logic", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const container = document.createElement("div");
		document.body.appendChild(container);

		applyRuntimeState(globals, {
			activeComponents: [undefined, makeSolidOutlet("child")],
			importURLs: ["/root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
		});

		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 1 });
		}, container);

		try {
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					ROUTE_CHANGE_EVENT_KEY,
				),
			).toBe(0);
			expect(
				countAddEventListenerCalls(
					addEventListenerSpy,
					LOCATION_EVENT_KEY,
				),
			).toBe(0);
		} finally {
			dispose();
			addEventListenerSpy.mockRestore();
			container.remove();
		}
	});

	it("react useLocation only updates on location events, not route events", async () => {
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
		window.history.replaceState(null, "", "/");

		let locationRenderCount = 0;
		const LocationProbe = () => {
			const location = adapter.useLocation();
			locationRenderCount += 1;
			return React.createElement(
				"div",
				{ "data-location-probe": true },
				location.pathname,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makeReactOutlet("root-a")],
			importURLs: ["/root-a.js"],
			exportKeys: ["default"],
			loadersData: [{}],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						React.createElement(LocationProbe),
					),
				);
			});
			expect(
				container.querySelector("[data-location-probe]")?.textContent,
			).toBe("/");
			const rendersAfterInitial = locationRenderCount;

			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root-b")],
				importURLs: ["/root-b.js"],
				exportKeys: ["default"],
				loadersData: [{}],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(locationRenderCount).toBe(rendersAfterInitial);

			window.history.pushState(null, "", "/location-next");
			await act(async () => {
				dispatchLocationChange();
			});
			expect(
				container.querySelector("[data-location-probe]")?.textContent,
			).toBe("/location-next");
			expect(locationRenderCount).toBe(rendersAfterInitial + 1);
		} finally {
			await act(async () => {
				root.unmount();
			});
			window.history.replaceState(null, "", "/");
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT =
				originalActEnvironment;
			container.remove();
			restore();
		}
	});

	it("preact location signal only updates on location events, not route events", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		window.history.replaceState(null, "", "/");

		let locationRenderCount = 0;
		const LocationProbe = () => {
			locationRenderCount += 1;
			return h(
				"div",
				{ "data-location-probe": true },
				adapter.location.value.pathname,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makePreactOutlet("root-a")],
			importURLs: ["/root-a.js"],
			exportKeys: ["default"],
			loadersData: [{}],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(adapter.VormaRootOutlet as any, { idx: 0 }),
						h(LocationProbe, {}),
					),
					container,
				);
			});
			expect(
				container.querySelector("[data-location-probe]")?.textContent,
			).toBe("/");
			const rendersAfterInitial = locationRenderCount;

			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root-b")],
				importURLs: ["/root-b.js"],
				exportKeys: ["default"],
				loadersData: [{}],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(locationRenderCount).toBe(rendersAfterInitial);

			window.history.pushState(null, "", "/location-next");
			await actPreact(async () => {
				dispatchLocationChange();
			});
			expect(
				container.querySelector("[data-location-probe]")?.textContent,
			).toBe("/location-next");
			expect(locationRenderCount).toBe(rendersAfterInitial + 1);
		} finally {
			renderPreact(null, container);
			window.history.replaceState(null, "", "/");
			container.remove();
			restore();
		}
	});

	it("solid location signal only updates on location events, not route events", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		window.history.replaceState(null, "", "/");

		let locationRenderCount = 0;
		const LocationProbe = () => {
			const node = document.createElement("div");
			node.setAttribute("data-location-probe", "true");
			createEffect(() => {
				locationRenderCount += 1;
				node.textContent = adapter.location().pathname;
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root-a")],
			importURLs: ["/root-a.js"],
			exportKeys: ["default"],
			loadersData: [{}],
		});

		const dispose = renderSolid(() => {
			return [
				createComponent(adapter.VormaRootOutlet as any, { idx: 0 }),
				LocationProbe(),
			];
		}, container);

		try {
			expect(
				container.querySelector("[data-location-probe]")?.textContent,
			).toBe("/");
			const rendersAfterInitial = locationRenderCount;

			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root-b")],
				importURLs: ["/root-b.js"],
				exportKeys: ["default"],
				loadersData: [{}],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(locationRenderCount).toBe(rendersAfterInitial);
			});

			window.history.pushState(null, "", "/location-next");
			dispatchLocationChange();
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-location-probe]")
						?.textContent,
				).toBe("/location-next");
			});
			expect(locationRenderCount).toBe(rendersAfterInitial + 1);
		} finally {
			dispose();
			window.history.replaceState(null, "", "/");
			container.remove();
			restore();
		}
	});

	it("react root outlet stays stable on data-only route updates while data hooks stay fresh", async () => {
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

		let rootRenderCount = 0;
		let dataRenderCount = 0;
		const useRouterData = adapter.makeTypedUseRouterData();
		const useLoaderData = adapter.makeTypedUseLoaderData();

		const StableRootComp = () => {
			rootRenderCount += 1;
			return React.createElement(
				"div",
				{ "data-root-probe": true },
				"root",
			);
		};
		const DataProbe = () => {
			dataRenderCount += 1;
			const nextLoaderData = useLoaderData({ idx: 0 }) as
				| { value?: string }
				| undefined;
			const nextRouterData = useRouterData();
			return React.createElement(
				"div",
				{ "data-data-probe": true },
				`${nextLoaderData?.value ?? ""}|${nextRouterData.matchedPatterns.join(",")}`,
			);
		};

		const stableActiveComponents = [StableRootComp];
		const stableImportURLs = ["/root.js"];
		const stableExportKeys = ["default"];

		applyRuntimeState(globals, {
			activeComponents: stableActiveComponents,
			importURLs: stableImportURLs,
			exportKeys: stableExportKeys,
			loadersData: [{ value: "a" }],
			clientLoadersData: ["ca"],
			matchedPatterns: ["/"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						React.createElement(DataProbe),
					),
				);
			});
			expect(
				container.querySelector("[data-root-probe]")?.textContent,
			).toBe("root");
			expect(
				container.querySelector("[data-data-probe]")?.textContent,
			).toBe("a|/");
			const rootRendersAfterInitial = rootRenderCount;
			const dataRendersAfterInitial = dataRenderCount;

			applyRuntimeState(globals, {
				activeComponents: stableActiveComponents,
				importURLs: stableImportURLs,
				exportKeys: stableExportKeys,
				loadersData: [{ value: "b" }],
				clientLoadersData: ["cb"],
				matchedPatterns: ["/next"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			expect(
				container.querySelector("[data-data-probe]")?.textContent,
			).toBe("b|/next");
			expect(rootRenderCount).toBe(rootRendersAfterInitial);
			expect(dataRenderCount).toBeGreaterThan(dataRendersAfterInitial);
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

	it("preact root outlet stays stable on data-only route updates while data signals stay fresh", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const rootContainer = document.createElement("div");
		const dataContainer = document.createElement("div");
		document.body.appendChild(rootContainer);
		document.body.appendChild(dataContainer);

		let rootRenderCount = 0;
		let dataRenderCount = 0;
		const useRouterData = adapter.makeTypedUseRouterData();
		const useLoaderData = adapter.makeTypedUseLoaderData();

		const StableRootComp = () => {
			rootRenderCount += 1;
			return h("div", { "data-root-probe": true }, "root");
		};
		const DataProbe = () => {
			dataRenderCount += 1;
			const nextLoaderData = useLoaderData({ idx: 0 }) as
				| { value?: string }
				| undefined;
			const nextRouterData = useRouterData();
			return h(
				"div",
				{ "data-data-probe": true },
				`${nextLoaderData?.value ?? ""}|${nextRouterData.matchedPatterns.join(",")}`,
			);
		};

		const stableActiveComponents = [StableRootComp];
		const stableImportURLs = ["/root.js"];
		const stableExportKeys = ["default"];

		applyRuntimeState(globals, {
			activeComponents: stableActiveComponents,
			importURLs: stableImportURLs,
			exportKeys: stableExportKeys,
			loadersData: [{ value: "a" }],
			clientLoadersData: ["ca"],
			matchedPatterns: ["/"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					rootContainer,
				);
			});
			await actPreact(async () => {
				renderPreact(h(DataProbe, {}), dataContainer);
			});
			expect(
				rootContainer.querySelector("[data-root-probe]")?.textContent,
			).toBe("root");
			expect(
				dataContainer.querySelector("[data-data-probe]")?.textContent,
			).toBe("a|/");
			const rootRendersAfterInitial = rootRenderCount;
			const dataRendersAfterInitial = dataRenderCount;

			applyRuntimeState(globals, {
				activeComponents: stableActiveComponents,
				importURLs: stableImportURLs,
				exportKeys: stableExportKeys,
				loadersData: [{ value: "b" }],
				clientLoadersData: ["cb"],
				matchedPatterns: ["/next"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			expect(
				dataContainer.querySelector("[data-data-probe]")?.textContent,
			).toBe("b|/next");
			expect(rootRenderCount).toBe(rootRendersAfterInitial);
			expect(dataRenderCount).toBeGreaterThan(dataRendersAfterInitial);
		} finally {
			renderPreact(null, rootContainer);
			renderPreact(null, dataContainer);
			rootContainer.remove();
			dataContainer.remove();
			restore();
		}
	});

	it("solid root outlet stays stable on data-only route updates while data signals stay fresh", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		let rootRunCount = 0;
		let dataRunCount = 0;
		const useRouterData = adapter.makeTypedUseRouterData();
		const useLoaderData = adapter.makeTypedUseLoaderData();

		const StableRootComp = () => {
			rootRunCount += 1;
			const node = document.createElement("div");
			node.setAttribute("data-root-probe", "true");
			node.textContent = "root";
			return node;
		};
		const DataProbe = () => {
			const loaderData = useLoaderData({ idx: 0 });
			const routerData = useRouterData();
			const node = document.createElement("div");
			node.setAttribute("data-data-probe", "true");
			createEffect(() => {
				dataRunCount += 1;
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const nextRouterData = routerData();
				node.textContent = `${nextLoaderData?.value ?? ""}|${nextRouterData.matchedPatterns.join(",")}`;
			});
			return node;
		};

		const stableActiveComponents = [StableRootComp];
		const stableImportURLs = ["/root.js"];
		const stableExportKeys = ["default"];

		applyRuntimeState(globals, {
			activeComponents: stableActiveComponents,
			importURLs: stableImportURLs,
			exportKeys: stableExportKeys,
			loadersData: [{ value: "a" }],
			clientLoadersData: ["ca"],
			matchedPatterns: ["/"],
		});

		const dispose = renderSolid(() => {
			return [
				createComponent(adapter.VormaRootOutlet as any, { idx: 0 }),
				DataProbe(),
			];
		}, container);

		try {
			expect(
				container.querySelector("[data-root-probe]")?.textContent,
			).toBe("root");
			expect(
				container.querySelector("[data-data-probe]")?.textContent,
			).toBe("a|/");
			const rootRunsAfterInitial = rootRunCount;
			const dataRunsAfterInitial = dataRunCount;

			applyRuntimeState(globals, {
				activeComponents: stableActiveComponents,
				importURLs: stableImportURLs,
				exportKeys: stableExportKeys,
				loadersData: [{ value: "b" }],
				clientLoadersData: ["cb"],
				matchedPatterns: ["/next"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-data-probe]")?.textContent,
				).toBe("b|/next");
			});

			expect(rootRunCount).toBe(rootRunsAfterInitial);
			expect(dataRunCount).toBeGreaterThan(dataRunsAfterInitial);
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("react typed loader/client-loader/router selectors update on route changes and ignore location events", async () => {
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
		window.history.replaceState(null, "", "/");

		const useRouterData = adapter.makeTypedUseRouterData();
		const useLoaderData = adapter.makeTypedUseLoaderData();
		const useClientLoaderData = adapter.makeTypedAddClientLoader()({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});

		let dataRenderCount = 0;
		const DataProbe = () => {
			dataRenderCount += 1;
			const routerData = useRouterData();
			const loaderData = useLoaderData({ idx: 0 }) as
				| { value?: string }
				| undefined;
			const clientLoaderData = useClientLoaderData({ idx: 0 }) as
				| string
				| undefined;
			return React.createElement(
				"div",
				{ "data-probe": true },
				`${loaderData?.value ?? ""}|${clientLoaderData ?? ""}|${routerData.matchedPatterns.join(",")}`,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makeReactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "a" }],
			clientLoadersData: ["ca"],
			matchedPatterns: ["/"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						React.createElement(DataProbe),
					),
				);
			});
			expect(container.querySelector("[data-probe]")?.textContent).toBe(
				"a|ca|/",
			);
			const rendersAfterInitial = dataRenderCount;

			window.history.pushState(null, "", "/location-next");
			await act(async () => {
				dispatchLocationChange();
			});
			expect(dataRenderCount).toBe(rendersAfterInitial);

			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "b" }],
				clientLoadersData: ["cb"],
				matchedPatterns: ["/next"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(container.querySelector("[data-probe]")?.textContent).toBe(
				"b|cb|/next",
			);
			expect(dataRenderCount).toBeGreaterThan(rendersAfterInitial);
		} finally {
			await act(async () => {
				root.unmount();
			});
			window.history.replaceState(null, "", "/");
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT =
				originalActEnvironment;
			container.remove();
			restore();
		}
	});

	it("preact typed loader/client-loader/router selectors update on route changes and ignore location events", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		window.history.replaceState(null, "", "/");

		const useRouterData = adapter.makeTypedUseRouterData();
		const useLoaderData = adapter.makeTypedUseLoaderData();
		const useClientLoaderData = adapter.makeTypedAddClientLoader()({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});

		let dataRenderCount = 0;
		const DataProbe = () => {
			dataRenderCount += 1;
			const routerData = useRouterData();
			const loaderData = useLoaderData({ idx: 0 }) as
				| { value?: string }
				| undefined;
			const clientLoaderData = useClientLoaderData({ idx: 0 }) as
				| string
				| undefined;
			return h(
				"div",
				{ "data-probe": true },
				`${loaderData?.value ?? ""}|${clientLoaderData ?? ""}|${routerData.matchedPatterns.join(",")}`,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makePreactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "a" }],
			clientLoadersData: ["ca"],
			matchedPatterns: ["/"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(adapter.VormaRootOutlet as any, { idx: 0 }),
						h(DataProbe, {}),
					),
					container,
				);
			});
			expect(container.querySelector("[data-probe]")?.textContent).toBe(
				"a|ca|/",
			);
			const rendersAfterInitial = dataRenderCount;

			window.history.pushState(null, "", "/location-next");
			await actPreact(async () => {
				dispatchLocationChange();
			});
			expect(dataRenderCount).toBe(rendersAfterInitial);

			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "b" }],
				clientLoadersData: ["cb"],
				matchedPatterns: ["/next"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(container.querySelector("[data-probe]")?.textContent).toBe(
				"b|cb|/next",
			);
			expect(dataRenderCount).toBeGreaterThan(rendersAfterInitial);
		} finally {
			renderPreact(null, container);
			window.history.replaceState(null, "", "/");
			container.remove();
			restore();
		}
	});

	it("solid typed loader/client-loader/router selectors update on route changes and ignore location events", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		window.history.replaceState(null, "", "/");

		const useRouterData = adapter.makeTypedUseRouterData();
		const useLoaderData = adapter.makeTypedUseLoaderData();
		const useClientLoaderData = adapter.makeTypedAddClientLoader()({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});

		let dataRenderCount = 0;
		const DataProbe = () => {
			const routerData = useRouterData();
			const loaderData = useLoaderData({ idx: 0 });
			const clientLoaderData = useClientLoaderData({ idx: 0 });
			const node = document.createElement("div");
			node.setAttribute("data-probe", "true");
			createEffect(() => {
				dataRenderCount += 1;
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const nextClientLoaderData = clientLoaderData() as
					| string
					| undefined;
				const nextRouterData = routerData();
				node.textContent = `${nextLoaderData?.value ?? ""}|${nextClientLoaderData ?? ""}|${nextRouterData.matchedPatterns.join(",")}`;
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "a" }],
			clientLoadersData: ["ca"],
			matchedPatterns: ["/"],
		});

		const dispose = renderSolid(() => {
			return [
				createComponent(adapter.VormaRootOutlet as any, { idx: 0 }),
				DataProbe(),
			];
		}, container);

		try {
			expect(container.querySelector("[data-probe]")?.textContent).toBe(
				"a|ca|/",
			);
			const rendersAfterInitial = dataRenderCount;

			window.history.pushState(null, "", "/location-next");
			dispatchLocationChange();
			await waitForCondition(() => {
				expect(dataRenderCount).toBe(rendersAfterInitial);
			});

			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "b" }],
				clientLoadersData: ["cb"],
				matchedPatterns: ["/next"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-probe]")?.textContent,
				).toBe("b|cb|/next");
			});
			expect(dataRenderCount).toBeGreaterThan(rendersAfterInitial);
		} finally {
			dispose();
			window.history.replaceState(null, "", "/");
			container.remove();
			restore();
		}
	});

	it("react typed pattern-based selectors stay fresh across route index changes", async () => {
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

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData();
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader()({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});

		const PatternProbe = () => {
			const loaderData = usePatternLoaderData("/probe") as
				| { value?: string }
				| undefined;
			const clientData = usePatternClientLoaderData() as
				| string
				| undefined;
			return React.createElement(
				"div",
				{ "data-pattern-probe": true },
				`${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [
				makeReactOutlet("root"),
				makeReactOutlet("child"),
			],
			importURLs: ["/root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{ value: "root-a" }, { value: "probe-a" }],
			clientLoadersData: ["root-ca", "probe-ca"],
			matchedPatterns: ["/root", "/probe"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						React.createElement(PatternProbe),
					),
				);
			});
			expect(
				container.querySelector("[data-pattern-probe]")?.textContent,
			).toBe("probe-a|probe-ca");

			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-b" }],
				clientLoadersData: ["root-cb"],
				matchedPatterns: ["/root"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-probe]")?.textContent,
			).toBe("none|none");

			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "probe-c" }],
				clientLoadersData: ["probe-cc"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-probe]")?.textContent,
			).toBe("probe-c|probe-cc");
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

	it("preact typed pattern-based selectors stay fresh across route index changes", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData();
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader()({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});

		const PatternProbe = () => {
			const loaderData = usePatternLoaderData("/probe") as
				| { value?: string }
				| undefined;
			const clientData = usePatternClientLoaderData() as
				| string
				| undefined;
			return h(
				"div",
				{ "data-pattern-probe": true },
				`${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [
				makePreactOutlet("root"),
				makePreactOutlet("child"),
			],
			importURLs: ["/root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{ value: "root-a" }, { value: "probe-a" }],
			clientLoadersData: ["root-ca", "probe-ca"],
			matchedPatterns: ["/root", "/probe"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(adapter.VormaRootOutlet as any, { idx: 0 }),
						h(PatternProbe, {}),
					),
					container,
				);
			});
			expect(
				container.querySelector("[data-pattern-probe]")?.textContent,
			).toBe("probe-a|probe-ca");

			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-b" }],
				clientLoadersData: ["root-cb"],
				matchedPatterns: ["/root"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-probe]")?.textContent,
			).toBe("none|none");

			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "probe-c" }],
				clientLoadersData: ["probe-cc"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-probe]")?.textContent,
			).toBe("probe-c|probe-cc");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid typed pattern-based selectors stay fresh across route index changes", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData();
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader()({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});

		const PatternProbe = () => {
			const loaderData = usePatternLoaderData("/probe");
			const clientData = usePatternClientLoaderData();
			const node = document.createElement("div");
			node.setAttribute("data-pattern-probe", "true");
			createEffect(() => {
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const nextClientData = clientData() as string | undefined;
				node.textContent = `${nextLoaderData?.value ?? "none"}|${nextClientData ?? "none"}`;
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [
				makeSolidOutlet("root"),
				makeSolidOutlet("child"),
			],
			importURLs: ["/root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{ value: "root-a" }, { value: "probe-a" }],
			clientLoadersData: ["root-ca", "probe-ca"],
			matchedPatterns: ["/root", "/probe"],
		});

		const dispose = renderSolid(() => {
			return [
				createComponent(adapter.VormaRootOutlet as any, { idx: 0 }),
				PatternProbe(),
			];
		}, container);

		try {
			expect(
				container.querySelector("[data-pattern-probe]")?.textContent,
			).toBe("probe-a|probe-ca");

			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-b" }],
				clientLoadersData: ["root-cb"],
				matchedPatterns: ["/root"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-pattern-probe]")
						?.textContent,
				).toBe("none|none");
			});

			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "probe-c" }],
				clientLoadersData: ["probe-cc"],
				matchedPatterns: ["/probe"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-pattern-probe]")
						?.textContent,
				).toBe("probe-c|probe-cc");
			});
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("react pattern loader selector stays fresh when match index changes", async () => {
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

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData();
		const PatternLoaderProbe = () => {
			const loaderData = usePatternLoaderData("/probe") as
				| { value?: string }
				| undefined;
			return React.createElement(
				"div",
				{ "data-pattern-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makeReactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "probe-a" }],
			clientLoadersData: [],
			matchedPatterns: ["/probe"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						React.createElement(PatternLoaderProbe),
					),
				);
			});
			expect(
				container.querySelector("[data-pattern-loader-probe]")
					?.textContent,
			).toBe("probe-a");

			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-b" }],
				clientLoadersData: [],
				matchedPatterns: ["/root"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-loader-probe]")
					?.textContent,
			).toBe("none");

			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "probe-c" }],
				clientLoadersData: [],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-loader-probe]")
					?.textContent,
			).toBe("probe-c");
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

	it("preact pattern loader selector stays fresh when match index changes", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData();
		const PatternLoaderProbe = () => {
			const loaderData = usePatternLoaderData("/probe") as
				| { value?: string }
				| undefined;
			return h(
				"div",
				{ "data-pattern-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makePreactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "probe-a" }],
			clientLoadersData: [],
			matchedPatterns: ["/probe"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(adapter.VormaRootOutlet as any, { idx: 0 }),
						h(PatternLoaderProbe, {}),
					),
					container,
				);
			});
			expect(
				container.querySelector("[data-pattern-loader-probe]")
					?.textContent,
			).toBe("probe-a");

			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-b" }],
				clientLoadersData: [],
				matchedPatterns: ["/root"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-loader-probe]")
					?.textContent,
			).toBe("none");

			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "probe-c" }],
				clientLoadersData: [],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-loader-probe]")
					?.textContent,
			).toBe("probe-c");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid pattern loader selector stays fresh when match index changes", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData();
		const PatternLoaderProbe = () => {
			const loaderData = usePatternLoaderData("/probe");
			const node = document.createElement("div");
			node.setAttribute("data-pattern-loader-probe", "true");
			createEffect(() => {
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				node.textContent = nextLoaderData?.value ?? "none";
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "probe-a" }],
			clientLoadersData: [],
			matchedPatterns: ["/probe"],
		});

		const dispose = renderSolid(() => {
			return [
				createComponent(adapter.VormaRootOutlet as any, { idx: 0 }),
				PatternLoaderProbe(),
			];
		}, container);

		try {
			expect(
				container.querySelector("[data-pattern-loader-probe]")
					?.textContent,
			).toBe("probe-a");

			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-b" }],
				clientLoadersData: [],
				matchedPatterns: ["/root"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-pattern-loader-probe]")
						?.textContent,
				).toBe("none");
			});

			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "probe-c" }],
				clientLoadersData: [],
				matchedPatterns: ["/probe"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-pattern-loader-probe]")
						?.textContent,
				).toBe("probe-c");
			});
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("react pattern client-loader selector stays fresh when match index changes", async () => {
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

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader()({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		const PatternClientProbe = () => {
			const clientData = usePatternClientLoaderData() as
				| string
				| undefined;
			return React.createElement(
				"div",
				{ "data-pattern-client-probe": true },
				clientData ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makeReactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{}],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						React.createElement(PatternClientProbe),
					),
				);
			});
			expect(
				container.querySelector("[data-pattern-client-probe]")
					?.textContent,
			).toBe("probe-a");

			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-client-probe]")
					?.textContent,
			).toBe("none");

			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-client-probe]")
					?.textContent,
			).toBe("probe-c");
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

	it("preact pattern client-loader selector stays fresh when match index changes", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader()({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		const PatternClientProbe = () => {
			const clientData = usePatternClientLoaderData() as
				| string
				| undefined;
			return h(
				"div",
				{ "data-pattern-client-probe": true },
				clientData ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makePreactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{}],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(adapter.VormaRootOutlet as any, { idx: 0 }),
						h(PatternClientProbe, {}),
					),
					container,
				);
			});
			expect(
				container.querySelector("[data-pattern-client-probe]")
					?.textContent,
			).toBe("probe-a");

			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-client-probe]")
					?.textContent,
			).toBe("none");

			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-pattern-client-probe]")
					?.textContent,
			).toBe("probe-c");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid pattern client-loader selector stays fresh when match index changes", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader()({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		const PatternClientProbe = () => {
			const clientData = usePatternClientLoaderData();
			const node = document.createElement("div");
			node.setAttribute("data-pattern-client-probe", "true");
			createEffect(() => {
				const nextClientData = clientData() as string | undefined;
				node.textContent = nextClientData ?? "none";
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{}],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		const dispose = renderSolid(() => {
			return [
				createComponent(adapter.VormaRootOutlet as any, { idx: 0 }),
				PatternClientProbe(),
			];
		}, container);

		try {
			expect(
				container.querySelector("[data-pattern-client-probe]")
					?.textContent,
			).toBe("probe-a");

			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-pattern-client-probe]")
						?.textContent,
				).toBe("none");
			});

			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-pattern-client-probe]")
						?.textContent,
				).toBe("probe-c");
			});
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});
});
