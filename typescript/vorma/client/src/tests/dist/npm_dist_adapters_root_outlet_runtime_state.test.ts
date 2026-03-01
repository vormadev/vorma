import { h, render as renderPreact } from "preact";
import { useEffect as usePreactEffect } from "preact/hooks";
import { act as actPreact } from "preact/test-utils";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import {
	createComponent,
	createEffect,
	createMemo,
	onMount as onSolidMount,
} from "solid-js";
import { render as renderSolid } from "solid-js/web";
import { afterAll, afterEach, describe, expect, it, vi } from "vitest";
import {
	DIST_TEST_VORMA_APP_CONFIG,
	installDistTestVormaGlobal,
	patchDistRuntimeRouteSnapshot,
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
	const nextMatchedPatterns = props.matchedPatterns ?? ["/"];
	const nextLoadersData = props.loadersData ?? [{}];
	const nextClientLoadersData = props.clientLoadersData ?? [];
	patchDistRuntimeRouteSnapshot({
		globals,
		patch: {
			matchedPatterns: nextMatchedPatterns,
			loadersData: nextLoadersData,
			clientLoadersData: nextClientLoadersData,
			importURLs: props.importURLs,
			exportKeys: props.exportKeys,
			outermostError: undefined,
			outermostErrorIdx: undefined,
			activeComponents: props.activeComponents,
			activeErrorBoundary: undefined,
		},
	});
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
	addEventListenerSpy: {
		mock: { calls: Array<readonly unknown[]> };
	},
	eventType: string,
): number {
	return addEventListenerSpy.mock.calls.filter((args: readonly unknown[]) => {
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

type DistAdapterName = "react" | "preact" | "solid";
type TickPhase = "initial" | "to-root" | "back-to-probe";
type TickTraceEntry = {
	phase: TickPhase;
	value: string;
	matchedPattern: string;
};

function summarizeTickTraceByLastPhaseEntry(props: {
	trace: TickTraceEntry[];
}): TickTraceEntry[] {
	const summarizePhase = (phase: TickPhase): TickTraceEntry => {
		const phaseEntries = props.trace.filter((entry) => {
			return entry.phase === phase;
		});
		const lastEntry = phaseEntries[phaseEntries.length - 1];
		if (!lastEntry) {
			throw new Error(
				`Missing transition tick trace entry for phase "${phase}".`,
			);
		}
		return lastEntry;
	};
	return [
		summarizePhase("initial"),
		summarizePhase("to-root"),
		summarizePhase("back-to-probe"),
	];
}

async function captureRoutePropsTickTraceByAdapter(props: {
	adapterName: DistAdapterName;
}): Promise<TickTraceEntry[]> {
	const globals = installDistTestVormaGlobal();
	vi.resetModules();
	const { restore } = installImmediateRAFAndScrollSpy();
	const container = document.createElement("div");
	document.body.appendChild(container);
	let phase: TickPhase = "initial";
	const trace: TickTraceEntry[] = [];

	try {
		if (props.adapterName === "react") {
			const adapter = await import("vorma/react");
			const root = createRoot(container);
			const originalActEnvironment = (globalThis as any)
				.IS_REACT_ACT_ENVIRONMENT;
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
			const useLoaderData = adapter.makeTypedUseLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const useRouterData = adapter.makeTypedUseRouterData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
				DIST_TEST_VORMA_APP_CONFIG,
			)({
				pattern: "/probe",
				clientLoader: async () => "unused",
			});
			const RouteComponent = (routeProps: any) => {
				const loaderData = useLoaderData(routeProps) as
					| { value?: string }
					| undefined;
				const clientData = usePatternClientLoaderData(routeProps) as
					| string
					| undefined;
				const routerData = useRouterData();
				trace.push({
					phase,
					value: `${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
					matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
				});
				return React.createElement("div", {});
			};

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-a" }],
				clientLoadersData: ["probe-a"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			await act(async () => {
				root.unmount();
			});
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT =
				originalActEnvironment;
			return summarizeTickTraceByLastPhaseEntry({
				trace,
			});
		}

		if (props.adapterName === "preact") {
			const adapter = await import("vorma/preact");
			const useLoaderData = adapter.makeTypedUseLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const useRouterData = adapter.makeTypedUseRouterData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
				DIST_TEST_VORMA_APP_CONFIG,
			)({
				pattern: "/probe",
				clientLoader: async () => "unused",
			});
			const RouteComponent = (routeProps: any) => {
				const loaderData = useLoaderData(routeProps) as
					| { value?: string }
					| undefined;
				const clientData = usePatternClientLoaderData(routeProps) as
					| string
					| undefined;
				const routerData = useRouterData();
				trace.push({
					phase,
					value: `${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
					matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
				});
				return h("div", {});
			};

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-a" }],
				clientLoadersData: ["probe-a"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			renderPreact(null, container);
			return summarizeTickTraceByLastPhaseEntry({
				trace,
			});
		}

		const adapter = await import("vorma/solid");
		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps);
			const clientData = usePatternClientLoaderData(routeProps);
			const routerData = useRouterData();
			const node = document.createElement("div");
			createEffect(() => {
				const currentRouterData = routerData();
				const currentLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const currentClientData = clientData() as string | undefined;
				trace.push({
					phase,
					value: `${currentLoaderData?.value ?? "none"}|${currentClientData ?? "none"}`,
					matchedPattern:
						currentRouterData.matchedPatterns[0] ?? "<none>",
				});
				node.textContent = `${currentLoaderData?.value ?? "none"}|${currentClientData ?? "none"}`;
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 0 });
		}, container);
		await waitForCondition(() => {
			expect(trace.length).toBeGreaterThan(0);
		});

		phase = "to-root";
		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-b" }],
			clientLoadersData: ["probe-b"],
			matchedPatterns: ["/probe"],
		});
		dispatchRouteChange({ x: 0, y: 0 });
		await waitForCondition(() => {
			expect(trace.some((entry) => entry.phase === "to-root")).toBe(true);
		});

		phase = "back-to-probe";
		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-c" }],
			clientLoadersData: ["probe-c"],
			matchedPatterns: ["/probe"],
		});
		dispatchRouteChange({ x: 0, y: 0 });
		await waitForCondition(() => {
			expect(trace.some((entry) => entry.phase === "back-to-probe")).toBe(
				true,
			);
		});

		dispose();
		return summarizeTickTraceByLastPhaseEntry({
			trace,
		});
	} finally {
		container.remove();
		restore();
	}
}

async function captureGlobalSelectorTickTraceByAdapter(props: {
	adapterName: DistAdapterName;
}): Promise<TickTraceEntry[]> {
	const globals = installDistTestVormaGlobal();
	vi.resetModules();
	const { restore } = installImmediateRAFAndScrollSpy();
	const container = document.createElement("div");
	document.body.appendChild(container);
	let phase: TickPhase = "initial";
	const trace: TickTraceEntry[] = [];

	try {
		if (props.adapterName === "react") {
			const adapter = await import("vorma/react");
			const root = createRoot(container);
			const originalActEnvironment = (globalThis as any)
				.IS_REACT_ACT_ENVIRONMENT;
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
			const useRouterData = adapter.makeTypedUseRouterData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
				DIST_TEST_VORMA_APP_CONFIG,
			)({
				pattern: "/probe",
				clientLoader: async () => "unused",
			});
			const GlobalProbe = () => {
				const routerData = useRouterData();
				const matchedPattern = (routerData.matchedPatterns[0] ??
					"/probe") as any;
				const loaderData = usePatternLoaderData(matchedPattern) as
					| { value?: string }
					| undefined;
				const clientData = usePatternClientLoaderData() as
					| string
					| undefined;
				trace.push({
					phase,
					value: `${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
					matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
				});
				return React.createElement("div", {});
			};

			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-a" }],
				clientLoadersData: ["probe-a"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						React.createElement(GlobalProbe),
					),
				);
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-loader-b" }],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			await act(async () => {
				root.unmount();
			});
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT =
				originalActEnvironment;
			return summarizeTickTraceByLastPhaseEntry({
				trace,
			});
		}

		if (props.adapterName === "preact") {
			const adapter = await import("vorma/preact");
			const useRouterData = adapter.makeTypedUseRouterData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
				DIST_TEST_VORMA_APP_CONFIG,
			)({
				pattern: "/probe",
				clientLoader: async () => "unused",
			});
			const GlobalProbe = () => {
				const routerData = useRouterData();
				const matchedPattern = (routerData.matchedPatterns[0] ??
					"/probe") as any;
				const loaderData = usePatternLoaderData(matchedPattern) as
					| { value?: string }
					| undefined;
				const clientData = usePatternClientLoaderData() as
					| string
					| undefined;
				trace.push({
					phase,
					value: `${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
					matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
				});
				return h("div", {});
			};

			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-a" }],
				clientLoadersData: ["probe-a"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(adapter.VormaRootOutlet as any, { idx: 0 }),
						h(GlobalProbe, {}),
					),
					container,
				);
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-loader-b" }],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			renderPreact(null, container);
			return summarizeTickTraceByLastPhaseEntry({
				trace,
			});
		}

		const adapter = await import("vorma/solid");
		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		const GlobalProbe = () => {
			const routerData = useRouterData();
			const loaderData = createMemo(() => {
				const matchedPattern = (routerData().matchedPatterns[0] ??
					"/probe") as any;
				return usePatternLoaderData(matchedPattern)();
			});
			const clientData = usePatternClientLoaderData();
			const node = document.createElement("div");
			createEffect(() => {
				const currentRouterData = routerData();
				const currentLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const currentClientData = clientData() as string | undefined;
				trace.push({
					phase,
					value: `${currentLoaderData?.value ?? "none"}|${currentClientData ?? "none"}`,
					matchedPattern:
						currentRouterData.matchedPatterns[0] ?? "<none>",
				});
				node.textContent = `${currentLoaderData?.value ?? "none"}|${currentClientData ?? "none"}`;
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});
		const dispose = renderSolid(() => {
			return [
				createComponent(adapter.VormaRootOutlet as any, { idx: 0 }),
				GlobalProbe(),
			];
		}, container);
		await waitForCondition(() => {
			expect(trace.length).toBeGreaterThan(0);
		});

		phase = "to-root";
		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "root-loader-b" }],
			clientLoadersData: ["root-b"],
			matchedPatterns: ["/root"],
		});
		dispatchRouteChange({ x: 0, y: 0 });
		await waitForCondition(() => {
			expect(trace.some((entry) => entry.phase === "to-root")).toBe(true);
		});

		phase = "back-to-probe";
		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-c" }],
			clientLoadersData: ["probe-c"],
			matchedPatterns: ["/probe"],
		});
		dispatchRouteChange({ x: 0, y: 0 });
		await waitForCondition(() => {
			expect(trace.some((entry) => entry.phase === "back-to-probe")).toBe(
				true,
			);
		});

		dispose();
		return summarizeTickTraceByLastPhaseEntry({
			trace,
		});
	} finally {
		container.remove();
		restore();
	}
}

async function captureNestedRoutePropsRemountValuesByAdapter(props: {
	adapterName: DistAdapterName;
}): Promise<[string | null, string | null]> {
	const globals = installDistTestVormaGlobal();
	vi.resetModules();
	const { restore } = installImmediateRAFAndScrollSpy();
	const container = document.createElement("div");
	document.body.appendChild(container);

	try {
		if (props.adapterName === "react") {
			const adapter = await import("vorma/react");
			const root = createRoot(container);
			const originalActEnvironment = (globalThis as any)
				.IS_REACT_ACT_ENVIRONMENT;
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
			const useLoaderData = adapter.makeTypedUseLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const RootComponent = (routeProps: any) => {
				return routeProps.Outlet(undefined);
			};
			const SharedChildComponent = (routeProps: any) => {
				const loaderData = useLoaderData(routeProps) as
					| { value?: string }
					| undefined;
				return React.createElement(
					"div",
					{ "data-remount-probe": true },
					loaderData?.value ?? "none",
				);
			};

			applyRuntimeState(globals, {
				activeComponents: [RootComponent, SharedChildComponent],
				importURLs: ["/root.js", "/child-a.js"],
				exportKeys: ["default", "default"],
				loadersData: [{ value: "root-a" }, { value: "loader-a" }],
				clientLoadersData: ["root-client-a", "child-client-a"],
				matchedPatterns: ["/root", "/probe"],
			});
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			const initialValue =
				container.querySelector("[data-remount-probe]")?.textContent ??
				null;

			applyRuntimeState(globals, {
				activeComponents: [RootComponent, SharedChildComponent],
				importURLs: ["/root.js", "/child-b.js"],
				exportKeys: ["default", "default"],
				loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				clientLoadersData: ["root-client-b", "child-client-b"],
				matchedPatterns: ["/root", "/other"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			const remountedValue =
				container.querySelector("[data-remount-probe]")?.textContent ??
				null;

			await act(async () => {
				root.unmount();
			});
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT =
				originalActEnvironment;
			return [initialValue, remountedValue];
		}

		if (props.adapterName === "preact") {
			const adapter = await import("vorma/preact");
			const useLoaderData = adapter.makeTypedUseLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const RootComponent = (routeProps: any) => {
				return routeProps.Outlet(undefined);
			};
			const SharedChildComponent = (routeProps: any) => {
				const loaderData = useLoaderData(routeProps) as
					| { value?: string }
					| undefined;
				return h(
					"div",
					{ "data-remount-probe": true },
					loaderData?.value ?? "none",
				);
			};

			applyRuntimeState(globals, {
				activeComponents: [RootComponent, SharedChildComponent],
				importURLs: ["/root.js", "/child-a.js"],
				exportKeys: ["default", "default"],
				loadersData: [{ value: "root-a" }, { value: "loader-a" }],
				clientLoadersData: ["root-client-a", "child-client-a"],
				matchedPatterns: ["/root", "/probe"],
			});
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});
			const initialValue =
				container.querySelector("[data-remount-probe]")?.textContent ??
				null;

			applyRuntimeState(globals, {
				activeComponents: [RootComponent, SharedChildComponent],
				importURLs: ["/root.js", "/child-b.js"],
				exportKeys: ["default", "default"],
				loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				clientLoadersData: ["root-client-b", "child-client-b"],
				matchedPatterns: ["/root", "/other"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			const remountedValue =
				container.querySelector("[data-remount-probe]")?.textContent ??
				null;

			renderPreact(null, container);
			return [initialValue, remountedValue];
		}

		const adapter = await import("vorma/solid");
		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RootComponent = (routeProps: any) => {
			return routeProps.Outlet(undefined);
		};
		const SharedChildComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps);
			const node = document.createElement("div");
			node.setAttribute("data-remount-probe", "true");
			createEffect(() => {
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				node.textContent = nextLoaderData?.value ?? "none";
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [RootComponent, SharedChildComponent],
			importURLs: ["/root.js", "/child-a.js"],
			exportKeys: ["default", "default"],
			loadersData: [{ value: "root-a" }, { value: "loader-a" }],
			clientLoadersData: ["root-client-a", "child-client-a"],
			matchedPatterns: ["/root", "/probe"],
		});
		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 0 });
		}, container);
		const initialValue =
			container.querySelector("[data-remount-probe]")?.textContent ??
			null;

		applyRuntimeState(globals, {
			activeComponents: [RootComponent, SharedChildComponent],
			importURLs: ["/root.js", "/child-b.js"],
			exportKeys: ["default", "default"],
			loadersData: [{ value: "root-b" }, { value: "loader-b" }],
			clientLoadersData: ["root-client-b", "child-client-b"],
			matchedPatterns: ["/root", "/other"],
		});
		dispatchRouteChange({ x: 0, y: 0 });
		await waitForCondition(() => {
			expect(
				container.querySelector("[data-remount-probe]")?.textContent,
			).toBe("loader-b");
		});
		const remountedValue =
			container.querySelector("[data-remount-probe]")?.textContent ??
			null;
		dispose();
		return [initialValue, remountedValue];
	} finally {
		container.remove();
		restore();
	}
}

async function captureDataOnlyUpdateComponentStabilityByAdapter(props: {
	adapterName: DistAdapterName;
}): Promise<{ mountCount: number }> {
	const globals = installDistTestVormaGlobal();
	vi.resetModules();
	const { restore } = installImmediateRAFAndScrollSpy();
	const container = document.createElement("div");
	document.body.appendChild(container);
	let mountCount = 0;

	try {
		if (props.adapterName === "react") {
			const adapter = await import("vorma/react");
			const root = createRoot(container);
			const originalActEnvironment = (globalThis as any)
				.IS_REACT_ACT_ENVIRONMENT;
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
			const useLoaderData = adapter.makeTypedUseLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const RouteComponent = (routeProps: any) => {
				const loaderData = useLoaderData(routeProps) as
					| { value?: string }
					| undefined;
				React.useEffect(() => {
					mountCount += 1;
				}, []);
				return React.createElement(
					"div",
					{ "data-stability-probe": true },
					loaderData?.value ?? "none",
				);
			};

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-a" }],
				clientLoadersData: ["probe-a"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(
				container.querySelector("[data-stability-probe]")?.textContent,
			).toBe("loader-a");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-stability-probe]")?.textContent,
			).toBe("loader-b");

			await act(async () => {
				root.unmount();
			});
			(globalThis as any).IS_REACT_ACT_ENVIRONMENT =
				originalActEnvironment;
			return { mountCount };
		}

		if (props.adapterName === "preact") {
			const adapter = await import("vorma/preact");
			const useLoaderData = adapter.makeTypedUseLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
			const RouteComponent = (routeProps: any) => {
				const loaderData = useLoaderData(routeProps) as
					| { value?: string }
					| undefined;
				usePreactEffect(() => {
					mountCount += 1;
				}, []);
				return h(
					"div",
					{ "data-stability-probe": true },
					loaderData?.value ?? "none",
				);
			};

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-a" }],
				clientLoadersData: ["probe-a"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});
			expect(
				container.querySelector("[data-stability-probe]")?.textContent,
			).toBe("loader-a");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-stability-probe]")?.textContent,
			).toBe("loader-b");

			renderPreact(null, container);
			return { mountCount };
		}

		const adapter = await import("vorma/solid");
		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps);
			const node = document.createElement("div");
			node.setAttribute("data-stability-probe", "true");
			onSolidMount(() => {
				mountCount += 1;
			});
			createEffect(() => {
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				node.textContent = nextLoaderData?.value ?? "none";
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});
		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 0 });
		}, container);
		await waitForCondition(() => {
			expect(
				container.querySelector("[data-stability-probe]")?.textContent,
			).toBe("loader-a");
		});

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-b" }],
			clientLoadersData: ["probe-b"],
			matchedPatterns: ["/probe"],
		});
		dispatchRouteChange({ x: 0, y: 0 });
		await waitForCondition(() => {
			expect(
				container.querySelector("[data-stability-probe]")?.textContent,
			).toBe("loader-b");
		});

		dispose();
		return { mountCount };
	} finally {
		container.remove();
		restore();
	}
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
		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);

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
			const nextRouterData = useRouterData();
			const nextLoaderData = usePatternLoaderData(
				(nextRouterData.matchedPatterns[0] ?? "/") as any,
			) as { value?: string } | undefined;
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
				matchedPatterns: ["/"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			expect(
				container.querySelector("[data-data-probe]")?.textContent,
			).toBe("b|/");
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
		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);

		const StableRootComp = () => {
			rootRenderCount += 1;
			return h("div", { "data-root-probe": true }, "root");
		};
		const DataProbe = () => {
			dataRenderCount += 1;
			const nextRouterData = useRouterData();
			const nextLoaderData = usePatternLoaderData(
				(nextRouterData.matchedPatterns[0] ?? "/") as any,
			) as { value?: string } | undefined;
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
				matchedPatterns: ["/"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			expect(
				dataContainer.querySelector("[data-data-probe]")?.textContent,
			).toBe("b|/");
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
		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);

		const StableRootComp = () => {
			rootRunCount += 1;
			const node = document.createElement("div");
			node.setAttribute("data-root-probe", "true");
			node.textContent = "root";
			return node;
		};
		const DataProbe = () => {
			const routerData = useRouterData();
			const loaderData = createMemo(() => {
				const nextRouterData = routerData();
				return usePatternLoaderData(
					(nextRouterData.matchedPatterns[0] ?? "/") as any,
				)();
			});
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
				matchedPatterns: ["/"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-data-probe]")?.textContent,
				).toBe("b|/");
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

		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/",
			clientLoader: async () => "unused",
		});

		let dataRenderCount = 0;
		const DataProbe = () => {
			dataRenderCount += 1;
			const routerData = useRouterData();
			const loaderData = usePatternLoaderData(
				(routerData.matchedPatterns[0] ?? "/") as any,
			) as { value?: string } | undefined;
			const clientLoaderData = useClientLoaderData() as
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
				"b||/next",
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

		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/",
			clientLoader: async () => "unused",
		});

		let dataRenderCount = 0;
		const DataProbe = () => {
			dataRenderCount += 1;
			const routerData = useRouterData();
			const loaderData = usePatternLoaderData(
				(routerData.matchedPatterns[0] ?? "/") as any,
			) as { value?: string } | undefined;
			const clientLoaderData = useClientLoaderData() as
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
				"b||/next",
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

		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/",
			clientLoader: async () => "unused",
		});

		let dataRenderCount = 0;
		const DataProbe = () => {
			const routerData = useRouterData();
			const loaderData = createMemo(() => {
				const nextRouterData = routerData();
				return usePatternLoaderData(
					(nextRouterData.matchedPatterns[0] ?? "/") as any,
				)();
			});
			const clientLoaderData = useClientLoaderData();
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
				).toBe("b||/next");
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

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
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

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
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

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
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

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
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

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
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

		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
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

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
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

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
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

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
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

	it("react useClientLoaderData(routeProps) stays fresh across transition windows for a stable route scope", async () => {
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

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		const RouteComponent = (routeProps: any) => {
			const clientData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			return React.createElement(
				"div",
				{ "data-react-route-props-client-probe": true },
				clientData ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(
				container.querySelector("[data-react-route-props-client-probe]")
					?.textContent,
			).toBe("probe-a");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-react-route-props-client-probe]")
					?.textContent,
			).toBe("probe-b");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-react-route-props-client-probe]")
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

	it("react route-props ownership remains correct after a render-abort before commit", async () => {
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

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		let shouldThrowDuringRender = true;

		class RenderAbortBoundary extends React.Component<
			{ children?: React.ReactNode },
			{ didError: boolean }
		> {
			constructor(props: { children?: React.ReactNode }) {
				super(props);
				this.state = { didError: false };
			}

			static getDerivedStateFromError() {
				return { didError: true };
			}

			override componentDidCatch() {}

			override render() {
				if (this.state.didError) {
					return React.createElement(
						"div",
						{ "data-react-render-abort-fallback": true },
						"render-error",
					);
				}
				return this.props.children;
			}
		}

		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			if (shouldThrowDuringRender) {
				throw new Error("intentional render abort");
			}
			return React.createElement(
				"div",
				{ "data-react-render-abort-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(
						RenderAbortBoundary,
						{ key: "first-render" },
						React.createElement(adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
					),
				);
			});
			expect(
				container.querySelector("[data-react-render-abort-fallback]"),
			).toBeTruthy();

			shouldThrowDuringRender = false;
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				root.render(
					React.createElement(
						RenderAbortBoundary,
						{ key: "second-render" },
						React.createElement(adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
					),
				);
			});
			expect(
				container.querySelector(
					"[data-react-render-abort-loader-probe]",
				)?.textContent,
			).toBe("loader-b");
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

	it("react useLoaderData(routeProps) tracks current route scope through transition windows", async () => {
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

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			return React.createElement(
				"div",
				{ "data-react-route-props-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(
				container.querySelector("[data-react-route-props-loader-probe]")
					?.textContent,
			).toBe("loader-a");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-loader-b" }],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-react-route-props-loader-probe]")
					?.textContent,
			).toBe("root-loader-b");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector("[data-react-route-props-loader-probe]")
					?.textContent,
			).toBe("loader-c");
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

	it("preact useLoaderData(routeProps) tracks current route scope through transition windows", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			return h(
				"div",
				{ "data-preact-route-props-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});
			expect(
				container.querySelector(
					"[data-preact-route-props-loader-probe]",
				)?.textContent,
			).toBe("loader-a");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-loader-b" }],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector(
					"[data-preact-route-props-loader-probe]",
				)?.textContent,
			).toBe("root-loader-b");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector(
					"[data-preact-route-props-loader-probe]",
				)?.textContent,
			).toBe("loader-c");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("preact useClientLoaderData(routeProps) stays fresh across transition windows for a stable route scope", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		const RouteComponent = (routeProps: any) => {
			const clientData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			return h(
				"div",
				{ "data-preact-route-props-client-probe": true },
				clientData ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});
			expect(
				container.querySelector(
					"[data-preact-route-props-client-probe]",
				)?.textContent,
			).toBe("probe-a");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector(
					"[data-preact-route-props-client-probe]",
				)?.textContent,
			).toBe("probe-b");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector(
					"[data-preact-route-props-client-probe]",
				)?.textContent,
			).toBe("probe-c");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid useLoaderData(routeProps) tracks current route scope through transition windows", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps);
			const node = document.createElement("div");
			node.setAttribute("data-solid-route-props-loader-probe", "true");
			createEffect(() => {
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				node.textContent = nextLoaderData?.value ?? "none";
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 0 });
		}, container);

		try {
			await waitForCondition(() => {
				expect(
					container.querySelector(
						"[data-solid-route-props-loader-probe]",
					)?.textContent,
				).toBe("loader-a");
			});

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-loader-b" }],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector(
						"[data-solid-route-props-loader-probe]",
					)?.textContent,
				).toBe("root-loader-b");
			});

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector(
						"[data-solid-route-props-loader-probe]",
					)?.textContent,
				).toBe("loader-c");
			});
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("solid useClientLoaderData(routeProps) stays fresh across transition windows for a stable route scope", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const outletContainer = document.createElement("div");
		const probeContainer = document.createElement("div");
		document.body.appendChild(outletContainer);
		document.body.appendChild(probeContainer);

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		let capturedRouteProps: any;
		const RouteComponent = (routeProps: any) => {
			const node = document.createElement("div");
			capturedRouteProps = routeProps;
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		const disposeOutlet = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 0 });
		}, outletContainer);
		await waitForCondition(() => {
			expect(capturedRouteProps).toBeDefined();
		});
		const disposeProbe = renderSolid(() => {
			const clientData = usePatternClientLoaderData(capturedRouteProps);
			const node = document.createElement("div");
			node.setAttribute("data-solid-route-props-client-probe", "true");
			createEffect(() => {
				const nextClientData = clientData() as string | undefined;
				node.textContent = nextClientData ?? "none";
			});
			return node;
		}, probeContainer);

		try {
			expect(
				probeContainer.querySelector(
					"[data-solid-route-props-client-probe]",
				)?.textContent,
			).toBe("probe-a");

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					probeContainer.querySelector(
						"[data-solid-route-props-client-probe]",
					)?.textContent,
				).toBe("probe-b");
			});

			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					probeContainer.querySelector(
						"[data-solid-route-props-client-probe]",
					)?.textContent,
				).toBe("probe-c");
			});
		} finally {
			disposeProbe();
			disposeOutlet();
			outletContainer.remove();
			probeContainer.remove();
			restore();
		}
	});

	it("react route-props hooks keep owner-consistent values on every transition tick", async () => {
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

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		let phase: "initial" | "to-root" | "back-to-probe" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			const clientData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			const routerData = useRouterData();
			trace.push({
				phase,
				value: `${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return React.createElement(
				"div",
				{ "data-react-route-props-tick-trace": true },
				`${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			const initialTrace = trace.filter(
				(entry) => entry.phase === "initial",
			);
			const toRootTrace = trace.filter(
				(entry) => entry.phase === "to-root",
			);
			const reboundTrace = trace.filter(
				(entry) => entry.phase === "back-to-probe",
			);
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(toRootTrace.length).toBeGreaterThan(0);
			expect(reboundTrace.length).toBeGreaterThan(0);
			if (
				!initialTrace.every((entry) => {
					return (
						entry.value === "loader-a|probe-a" &&
						entry.matchedPattern === "/probe"
					);
				})
			) {
				throw new Error(JSON.stringify({ reactTrace: trace }));
			}
			expect(
				toRootTrace.every((entry) => {
					return (
						entry.value === "loader-b|probe-b" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
			expect(
				reboundTrace.every((entry) => {
					return (
						entry.value === "loader-c|probe-c" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
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

	it("preact route-props hooks keep owner-consistent values on every transition tick", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		let phase: "initial" | "to-root" | "back-to-probe" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			const clientData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			const routerData = useRouterData();
			trace.push({
				phase,
				value: `${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return h(
				"div",
				{ "data-preact-route-props-tick-trace": true },
				`${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			const initialTrace = trace.filter(
				(entry) => entry.phase === "initial",
			);
			const toRootTrace = trace.filter(
				(entry) => entry.phase === "to-root",
			);
			const reboundTrace = trace.filter(
				(entry) => entry.phase === "back-to-probe",
			);
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(toRootTrace.length).toBeGreaterThan(0);
			expect(reboundTrace.length).toBeGreaterThan(0);
			if (
				!initialTrace.every((entry) => {
					return (
						entry.value === "loader-a|probe-a" &&
						entry.matchedPattern === "/probe"
					);
				})
			) {
				throw new Error(JSON.stringify({ preactTrace: trace }));
			}
			expect(
				toRootTrace.every((entry) => {
					return (
						entry.value === "loader-b|probe-b" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
			expect(
				reboundTrace.every((entry) => {
					return (
						entry.value === "loader-c|probe-c" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid route-props hooks keep owner-consistent values on every transition tick", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		let phase: "initial" | "to-root" | "back-to-probe" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps);
			const clientData = usePatternClientLoaderData(routeProps);
			const routerData = useRouterData();
			const node = document.createElement("div");
			node.setAttribute("data-solid-route-props-tick-trace", "true");
			createEffect(() => {
				const currentRouterData = routerData();
				const currentLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const currentClientData = clientData() as string | undefined;
				trace.push({
					phase,
					value: `${currentLoaderData?.value ?? "none"}|${currentClientData ?? "none"}`,
					matchedPattern:
						currentRouterData.matchedPatterns[0] ?? "<none>",
				});
				node.textContent = `${currentLoaderData?.value ?? "none"}|${currentClientData ?? "none"}`;
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [RouteComponent],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 0 });
		}, container);

		try {
			await waitForCondition(() => {
				expect(
					container.querySelector(
						"[data-solid-route-props-tick-trace]",
					)?.textContent,
				).toBe("loader-a|probe-a");
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-b" }],
				clientLoadersData: ["probe-b"],
				matchedPatterns: ["/probe"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector(
						"[data-solid-route-props-tick-trace]",
					)?.textContent,
				).toBe("loader-b|probe-b");
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [RouteComponent],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector(
						"[data-solid-route-props-tick-trace]",
					)?.textContent,
				).toBe("loader-c|probe-c");
			});

			const initialTrace = trace.filter(
				(entry) => entry.phase === "initial",
			);
			const toRootTrace = trace.filter(
				(entry) => entry.phase === "to-root",
			);
			const reboundTrace = trace.filter(
				(entry) => entry.phase === "back-to-probe",
			);
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(toRootTrace.length).toBeGreaterThan(0);
			expect(reboundTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every((entry) => {
					return (
						entry.value === "loader-a|probe-a" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
			expect(
				toRootTrace.every((entry) => {
					return (
						entry.value === "loader-b|probe-b" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
			expect(
				reboundTrace.every((entry) => {
					return (
						entry.value === "loader-c|probe-c" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("react global selectors keep snapshot-coherent values on every transition tick", async () => {
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

		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		let phase: "initial" | "to-root" | "back-to-probe" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const GlobalProbe = () => {
			const routerData = useRouterData();
			const matchedPattern = (routerData.matchedPatterns[0] ??
				"/probe") as any;
			const loaderData = usePatternLoaderData(matchedPattern) as
				| { value?: string }
				| undefined;
			const clientData = usePatternClientLoaderData() as
				| string
				| undefined;
			trace.push({
				phase,
				value: `${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return React.createElement(
				"div",
				{ "data-react-global-tick-trace": true },
				`${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makeReactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
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
						React.createElement(GlobalProbe),
					),
				);
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-loader-b" }],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [makeReactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			const initialTrace = trace.filter(
				(entry) => entry.phase === "initial",
			);
			const toRootTrace = trace.filter(
				(entry) => entry.phase === "to-root",
			);
			const reboundTrace = trace.filter(
				(entry) => entry.phase === "back-to-probe",
			);
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(toRootTrace.length).toBeGreaterThan(0);
			expect(reboundTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every((entry) => {
					return (
						(entry.value === "none|none" &&
							entry.matchedPattern === "<none>") ||
						(entry.value === "loader-a|probe-a" &&
							entry.matchedPattern === "/probe")
					);
				}),
			).toBe(true);
			expect(
				toRootTrace.every((entry) => {
					return (
						entry.value === "root-loader-b|none" &&
						entry.matchedPattern === "/root"
					);
				}),
			).toBe(true);
			expect(
				reboundTrace.every((entry) => {
					return (
						entry.value === "loader-c|probe-c" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
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

	it("preact global selectors keep snapshot-coherent values on every transition tick", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		let phase: "initial" | "to-root" | "back-to-probe" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const GlobalProbe = () => {
			const routerData = useRouterData();
			const matchedPattern = (routerData.matchedPatterns[0] ??
				"/probe") as any;
			const loaderData = usePatternLoaderData(matchedPattern) as
				| { value?: string }
				| undefined;
			const clientData = usePatternClientLoaderData() as
				| string
				| undefined;
			trace.push({
				phase,
				value: `${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return h(
				"div",
				{ "data-preact-global-tick-trace": true },
				`${loaderData?.value ?? "none"}|${clientData ?? "none"}`,
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [makePreactOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
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
						h(GlobalProbe, {}),
					),
					container,
				);
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-loader-b" }],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [makePreactOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});

			const initialTrace = trace.filter(
				(entry) => entry.phase === "initial",
			);
			const toRootTrace = trace.filter(
				(entry) => entry.phase === "to-root",
			);
			const reboundTrace = trace.filter(
				(entry) => entry.phase === "back-to-probe",
			);
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(toRootTrace.length).toBeGreaterThan(0);
			expect(reboundTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every((entry) => {
					return (
						(entry.value === "none|none" &&
							entry.matchedPattern === "<none>") ||
						(entry.value === "loader-a|probe-a" &&
							entry.matchedPattern === "/probe")
					);
				}),
			).toBe(true);
			expect(
				toRootTrace.every((entry) => {
					return (
						entry.value === "root-loader-b|none" &&
						entry.matchedPattern === "/root"
					);
				}),
			).toBe(true);
			expect(
				reboundTrace.every((entry) => {
					return (
						entry.value === "loader-c|probe-c" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid global selectors keep snapshot-coherent values on every transition tick", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const useRouterData = adapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = adapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});
		let phase: "initial" | "to-root" | "back-to-probe" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const GlobalProbe = () => {
			const routerData = useRouterData();
			const loaderData = createMemo(() => {
				const matchedPattern = (routerData().matchedPatterns[0] ??
					"/probe") as any;
				return usePatternLoaderData(matchedPattern)();
			});
			const clientData = usePatternClientLoaderData();
			const node = document.createElement("div");
			node.setAttribute("data-solid-global-tick-trace", "true");
			createEffect(() => {
				const currentRouterData = routerData();
				const currentLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const currentClientData = clientData() as string | undefined;
				trace.push({
					phase,
					value: `${currentLoaderData?.value ?? "none"}|${currentClientData ?? "none"}`,
					matchedPattern:
						currentRouterData.matchedPatterns[0] ?? "<none>",
				});
				node.textContent = `${currentLoaderData?.value ?? "none"}|${currentClientData ?? "none"}`;
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [makeSolidOutlet("root")],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{ value: "loader-a" }],
			clientLoadersData: ["probe-a"],
			matchedPatterns: ["/probe"],
		});

		const dispose = renderSolid(() => {
			return [
				createComponent(adapter.VormaRootOutlet as any, { idx: 0 }),
				GlobalProbe(),
			];
		}, container);

		try {
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-solid-global-tick-trace]")
						?.textContent,
				).toBe("loader-a|probe-a");
			});

			phase = "to-root";
			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "root-loader-b" }],
				clientLoadersData: ["root-b"],
				matchedPatterns: ["/root"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-solid-global-tick-trace]")
						?.textContent,
				).toBe("root-loader-b|none");
			});

			phase = "back-to-probe";
			applyRuntimeState(globals, {
				activeComponents: [makeSolidOutlet("root")],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{ value: "loader-c" }],
				clientLoadersData: ["probe-c"],
				matchedPatterns: ["/probe"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-solid-global-tick-trace]")
						?.textContent,
				).toBe("loader-c|probe-c");
			});

			const initialTrace = trace.filter(
				(entry) => entry.phase === "initial",
			);
			const toRootTrace = trace.filter(
				(entry) => entry.phase === "to-root",
			);
			const reboundTrace = trace.filter(
				(entry) => entry.phase === "back-to-probe",
			);
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(toRootTrace.length).toBeGreaterThan(0);
			expect(reboundTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every((entry) => {
					return (
						entry.value === "loader-a|probe-a" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
			expect(
				toRootTrace.every((entry) => {
					return (
						entry.value === "root-loader-b|none" &&
						entry.matchedPattern === "/root"
					);
				}),
			).toBe(true);
			expect(
				reboundTrace.every((entry) => {
					return (
						entry.value === "loader-c|probe-c" &&
						entry.matchedPattern === "/probe"
					);
				}),
			).toBe(true);
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("react nested route-props loader data follows route-key remounts when component identity is reused", async () => {
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

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RootComponent = (routeProps: any) => {
			return routeProps.Outlet(undefined);
		};
		const SharedChildComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			return React.createElement(
				"div",
				{ "data-react-nested-route-props-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [RootComponent, SharedChildComponent],
			importURLs: ["/root.js", "/child-a.js"],
			exportKeys: ["default", "default"],
			loadersData: [{ value: "root-a" }, { value: "loader-a" }],
			clientLoadersData: ["root-client-a", "child-client-a"],
			matchedPatterns: ["/root", "/probe"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(adapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(
				container.querySelector(
					"[data-react-nested-route-props-loader-probe]",
				)?.textContent,
			).toBe("loader-a");

			applyRuntimeState(globals, {
				activeComponents: [RootComponent, SharedChildComponent],
				importURLs: ["/root.js", "/child-b.js"],
				exportKeys: ["default", "default"],
				loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				clientLoadersData: ["root-client-b", "child-client-b"],
				matchedPatterns: ["/root", "/other"],
			});
			await act(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector(
					"[data-react-nested-route-props-loader-probe]",
				)?.textContent,
			).toBe("loader-b");
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

	it("preact nested route-props loader data follows route-key remounts when component identity is reused", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RootComponent = (routeProps: any) => {
			return routeProps.Outlet(undefined);
		};
		const SharedChildComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			return h(
				"div",
				{ "data-preact-nested-route-props-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};

		applyRuntimeState(globals, {
			activeComponents: [RootComponent, SharedChildComponent],
			importURLs: ["/root.js", "/child-a.js"],
			exportKeys: ["default", "default"],
			loadersData: [{ value: "root-a" }, { value: "loader-a" }],
			clientLoadersData: ["root-client-a", "child-client-a"],
			matchedPatterns: ["/root", "/probe"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(adapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});
			expect(
				container.querySelector(
					"[data-preact-nested-route-props-loader-probe]",
				)?.textContent,
			).toBe("loader-a");

			applyRuntimeState(globals, {
				activeComponents: [RootComponent, SharedChildComponent],
				importURLs: ["/root.js", "/child-b.js"],
				exportKeys: ["default", "default"],
				loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				clientLoadersData: ["root-client-b", "child-client-b"],
				matchedPatterns: ["/root", "/other"],
			});
			await actPreact(async () => {
				dispatchRouteChange({ x: 0, y: 0 });
			});
			expect(
				container.querySelector(
					"[data-preact-nested-route-props-loader-probe]",
				)?.textContent,
			).toBe("loader-b");
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid nested route-props loader data follows route-key remounts when component identity is reused", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const adapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const useLoaderData = adapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RootComponent = (routeProps: any) => {
			return routeProps.Outlet(undefined);
		};
		const SharedChildComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps);
			const node = document.createElement("div");
			node.setAttribute(
				"data-solid-nested-route-props-loader-probe",
				"true",
			);
			createEffect(() => {
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				node.textContent = nextLoaderData?.value ?? "none";
			});
			return node;
		};

		applyRuntimeState(globals, {
			activeComponents: [RootComponent, SharedChildComponent],
			importURLs: ["/root.js", "/child-a.js"],
			exportKeys: ["default", "default"],
			loadersData: [{ value: "root-a" }, { value: "loader-a" }],
			clientLoadersData: ["root-client-a", "child-client-a"],
			matchedPatterns: ["/root", "/probe"],
		});

		const dispose = renderSolid(() => {
			return createComponent(adapter.VormaRootOutlet as any, { idx: 0 });
		}, container);

		try {
			expect(
				container.querySelector(
					"[data-solid-nested-route-props-loader-probe]",
				)?.textContent,
			).toBe("loader-a");

			applyRuntimeState(globals, {
				activeComponents: [RootComponent, SharedChildComponent],
				importURLs: ["/root.js", "/child-b.js"],
				exportKeys: ["default", "default"],
				loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				clientLoadersData: ["root-client-b", "child-client-b"],
				matchedPatterns: ["/root", "/other"],
			});
			dispatchRouteChange({ x: 0, y: 0 });
			await waitForCondition(() => {
				expect(
					container.querySelector(
						"[data-solid-nested-route-props-loader-probe]",
					)?.textContent,
				).toBe("loader-b");
			});
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("cross-adapter route-props transition traces are parity-consistent", async () => {
		const reactTrace = await captureRoutePropsTickTraceByAdapter({
			adapterName: "react",
		});
		const preactTrace = await captureRoutePropsTickTraceByAdapter({
			adapterName: "preact",
		});
		const solidTrace = await captureRoutePropsTickTraceByAdapter({
			adapterName: "solid",
		});

		expect(reactTrace).toEqual([
			{
				phase: "initial",
				value: "loader-a|probe-a",
				matchedPattern: "/probe",
			},
			{
				phase: "to-root",
				value: "loader-b|probe-b",
				matchedPattern: "/probe",
			},
			{
				phase: "back-to-probe",
				value: "loader-c|probe-c",
				matchedPattern: "/probe",
			},
		]);
		expect(preactTrace).toEqual(reactTrace);
		expect(solidTrace).toEqual(reactTrace);
	});

	it("cross-adapter global selector transition traces are parity-consistent", async () => {
		const reactTrace = await captureGlobalSelectorTickTraceByAdapter({
			adapterName: "react",
		});
		const preactTrace = await captureGlobalSelectorTickTraceByAdapter({
			adapterName: "preact",
		});
		const solidTrace = await captureGlobalSelectorTickTraceByAdapter({
			adapterName: "solid",
		});

		expect(reactTrace).toEqual([
			{
				phase: "initial",
				value: "loader-a|probe-a",
				matchedPattern: "/probe",
			},
			{
				phase: "to-root",
				value: "root-loader-b|none",
				matchedPattern: "/root",
			},
			{
				phase: "back-to-probe",
				value: "loader-c|probe-c",
				matchedPattern: "/probe",
			},
		]);
		expect(preactTrace).toEqual(reactTrace);
		expect(solidTrace).toEqual(reactTrace);
	});

	it("cross-adapter nested route remount behavior is parity-consistent", async () => {
		const reactValues = await captureNestedRoutePropsRemountValuesByAdapter(
			{
				adapterName: "react",
			},
		);
		const preactValues =
			await captureNestedRoutePropsRemountValuesByAdapter({
				adapterName: "preact",
			});
		const solidValues = await captureNestedRoutePropsRemountValuesByAdapter(
			{
				adapterName: "solid",
			},
		);

		expect(reactValues).toEqual(["loader-a", "loader-b"]);
		expect(preactValues).toEqual(reactValues);
		expect(solidValues).toEqual(reactValues);
	});

	it("cross-adapter data-only updates preserve stable component mounts", async () => {
		const reactStability =
			await captureDataOnlyUpdateComponentStabilityByAdapter({
				adapterName: "react",
			});
		const preactStability =
			await captureDataOnlyUpdateComponentStabilityByAdapter({
				adapterName: "preact",
			});
		const solidStability =
			await captureDataOnlyUpdateComponentStabilityByAdapter({
				adapterName: "solid",
			});

		expect(reactStability).toEqual({
			mountCount: 1,
		});
		expect(preactStability).toEqual(reactStability);
		expect(solidStability).toEqual(reactStability);
	});

	it("react useClientLoaderData(routeProps) throws when route props are not produced by a route component", async () => {
		const globals = installDistTestVormaGlobal();
		applyRuntimeState(globals, {
			activeComponents: [
				makeReactOutlet("root"),
				makeReactOutlet("child"),
			],
			importURLs: ["/root.js", "/child.js"],
			exportKeys: ["default", "default"],
			loadersData: [{}, {}],
			clientLoadersData: ["probe-a", "other-a"],
			matchedPatterns: ["/probe", "/other"],
		});
		vi.resetModules();
		const adapter = await import("vorma/react");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		const usePatternClientLoaderData = adapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/probe",
			clientLoader: async () => "unused",
		});

		class RoutePropsContractErrorBoundary extends React.Component<
			{
				children?: React.ReactNode;
			},
			{
				errorMessage: string | null;
			}
		> {
			constructor(props: { children?: React.ReactNode }) {
				super(props);
				this.state = {
					errorMessage: null,
				};
			}

			static getDerivedStateFromError(error: Error) {
				return {
					errorMessage: error.message,
				};
			}

			render() {
				if (this.state.errorMessage) {
					return React.createElement(
						"div",
						{ "data-route-props-contract-error": true },
						this.state.errorMessage,
					);
				}
				return this.props.children;
			}
		}

		const PatternClientProbe = () => {
			usePatternClientLoaderData({ idx: 1 } as any);
			return React.createElement("div", {
				"data-pattern-client-props-contract-probe": true,
			});
		};

		try {
			await act(async () => {
				root.render(
					React.createElement(
						RoutePropsContractErrorBoundary,
						{},
						React.createElement(
							React.Fragment,
							{},
							React.createElement(
								adapter.VormaRootOutlet as any,
								{
									idx: 0,
								},
							),
							React.createElement(PatternClientProbe),
						),
					),
				);
			});

			const errorText = container.querySelector(
				"[data-route-props-contract-error]",
			)?.textContent;
			expect(errorText).toContain(
				"useClientLoaderData(routeProps) contract violated",
			);
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
});
