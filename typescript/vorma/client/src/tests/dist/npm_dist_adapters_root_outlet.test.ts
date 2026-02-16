import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { h, render as renderPreact } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import { act as actPreact } from "preact/test-utils";
import { createComponent, onCleanup } from "solid-js";
import { render as renderSolid } from "solid-js/web";
import { describe, expect, it, vi } from "vitest";
import {
	installDistTestVormaGlobal,
	type DistTestVormaInternal,
} from "./dist_test_harness.ts";

const ROUTE_CHANGE_EVENT_KEY = "vorma:route-change";
const LOCATION_EVENT_KEY = "vorma:location";

type RuntimeComponent = (props: { idx: number; Outlet: any }) => unknown;

function applyRootOutletRuntimeState(
	globals: DistTestVormaInternal,
	props: {
		activeComponents: Array<RuntimeComponent> | null;
		importURLs: string[];
		exportKeys: string[];
		loadersData?: unknown[];
		clientLoadersData?: unknown[];
		matchedPatterns?: string[];
		outermostError?: unknown;
		outermostErrorIdx?: number | undefined;
		activeErrorBoundary?: unknown;
	},
): void {
	globals.matchedPatterns = props.matchedPatterns ?? ["/", "/child"];
	globals.loadersData = props.loadersData ?? [{}, {}];
	globals.clientLoadersData = props.clientLoadersData ?? [];
	globals.outermostError = props.outermostError;
	globals.outermostErrorIdx = props.outermostErrorIdx;
	globals.activeComponents = props.activeComponents;
	globals.activeErrorBoundary = props.activeErrorBoundary;
	globals.importURLs = props.importURLs;
	globals.exportKeys = props.exportKeys;
}

function dispatchRouteChangeWithScrollState(scrollState: {
	x: number;
	y: number;
}) {
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
		scrollToSpy,
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

describe("npm_dist adapter root outlets", () => {
	it("react root outlet preserves child instance until child module identity changes and ignores location events", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const reactAdapter = await import("vorma/react");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();

		let parentRenderCount = 0;
		let childRenderCount = 0;
		let childMountCount = 0;
		let childUnmountCount = 0;

		const ChildComp = () => {
			const mountIDRef = React.useRef<number | null>(null);
			if (mountIDRef.current === null) {
				childMountCount += 1;
				mountIDRef.current = childMountCount;
			}
			childRenderCount += 1;
			React.useEffect(() => {
				return () => {
					childUnmountCount += 1;
				};
			}, []);
			return React.createElement(
				"div",
				{},
				`child:${mountIDRef.current}`,
			);
		};

		const ParentComp = (props: { idx: number; Outlet: any }) => {
			parentRenderCount += 1;
			return React.createElement(
				"section",
				{},
				React.createElement("span", {}, `parent:${props.idx}`),
				React.createElement(props.Outlet, {}),
			);
		};

		const stableActiveComponents = [ParentComp, ChildComp];
		const stableImportURLs = ["/parent.js", "/child-a.js"];
		const stableExportKeys = ["default", "default"];

		applyRootOutletRuntimeState(globals, {
			activeComponents: stableActiveComponents,
			importURLs: stableImportURLs,
			exportKeys: stableExportKeys,
		});

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
			expect(container.textContent).toContain("child:1");

			const parentRendersAfterInitial = parentRenderCount;
			const childRendersAfterInitial = childRenderCount;

			await act(async () => {
				dispatchLocationChange();
			});

			expect(parentRenderCount).toBe(parentRendersAfterInitial);
			expect(childRenderCount).toBe(childRendersAfterInitial);

			await act(async () => {
				dispatchRouteChangeWithScrollState({ x: 1, y: 2 });
			});

			expect(container.textContent).toContain("child:1");
			expect(parentRenderCount).toBe(parentRendersAfterInitial);
			expect(childRenderCount).toBe(childRendersAfterInitial);
			expect(childUnmountCount).toBe(0);
			expect(scrollToSpy).toHaveBeenCalledWith(1, 2);

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildComp],
				importURLs: ["/parent.js", "/child-b.js"],
				exportKeys: ["default", "default"],
			});
			await act(async () => {
				dispatchRouteChangeWithScrollState({ x: 3, y: 4 });
			});

			expect(container.textContent).toContain("child:2");
			expect(childUnmountCount).toBe(1);
			expect(scrollToSpy).toHaveBeenCalledWith(3, 4);
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

	it("react root outlet uses the freshest child component when route updates keep the same child key", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const reactAdapter = await import("vorma/react");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		let childARenderCount = 0;
		let childBRenderCount = 0;

		const ParentComp = (props: { Outlet: any }) =>
			React.createElement(props.Outlet, {});
		const ChildA = () => {
			childARenderCount += 1;
			return React.createElement("div", {}, "child-a");
		};
		const ChildB = () => {
			childBRenderCount += 1;
			return React.createElement("div", {}, "child-b");
		};

		applyRootOutletRuntimeState(globals, {
			activeComponents: [ParentComp, ChildA],
			importURLs: ["/parent.js", "/child.js"],
			exportKeys: ["default", "default"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(container.textContent).toContain("child-a");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildB],
				importURLs: ["/parent.js", "/child.js"],
				exportKeys: ["default", "default"],
			});
			await act(async () => {
				dispatchRouteChangeWithScrollState({ x: 5, y: 6 });
			});

			expect(container.textContent).toContain("child-b");
			expect(childARenderCount).toBeGreaterThan(0);
			expect(childBRenderCount).toBeGreaterThan(0);
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

	it("react root outlet uses the freshest root component when route updates keep the same root key", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const reactAdapter = await import("vorma/react");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		let rootARenderCount = 0;
		let rootBRenderCount = 0;

		const RootA = () => {
			rootARenderCount += 1;
			return React.createElement("div", {}, "root-a");
		};
		const RootB = () => {
			rootBRenderCount += 1;
			return React.createElement("div", {}, "root-b");
		};

		applyRootOutletRuntimeState(globals, {
			activeComponents: [RootA],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{}],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(container.textContent).toContain("root-a");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [RootB],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
			});
			await act(async () => {
				dispatchRouteChangeWithScrollState({ x: 31, y: 32 });
			});

			expect(container.textContent).toContain("root-b");
			expect(rootARenderCount).toBeGreaterThan(0);
			expect(rootBRenderCount).toBeGreaterThan(0);
			expect(scrollToSpy).toHaveBeenCalledWith(31, 32);
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

	it("react keeps parent instance and local parent input state when only child route changes", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const reactAdapter = await import("vorma/react");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		let parentMountCount = 0;
		let parentUnmountCount = 0;
		let childARenderCount = 0;
		let childBRenderCount = 0;

		const ChildA = () => {
			childARenderCount += 1;
			return React.createElement("div", {}, "child-a");
		};
		const ChildB = () => {
			childBRenderCount += 1;
			return React.createElement("div", {}, "child-b");
		};

		const ParentComp = (props: { Outlet: any }) => {
			const mountIDRef = React.useRef<number | null>(null);
			if (mountIDRef.current === null) {
				parentMountCount += 1;
				mountIDRef.current = parentMountCount;
			}
			React.useEffect(() => {
				return () => {
					parentUnmountCount += 1;
				};
			}, []);

			const [draft, setDraft] = React.useState("initial");
			return React.createElement(
				"section",
				{},
				React.createElement(
					"div",
					{ "data-parent-mount-id": true },
					String(mountIDRef.current),
				),
				React.createElement(
					"div",
					{ "data-parent-draft": true },
					draft,
				),
				React.createElement("input", {
					value: draft,
					onInput: (event: React.FormEvent<HTMLInputElement>) => {
						setDraft(event.currentTarget.value);
					},
				}),
				React.createElement(props.Outlet, {}),
			);
		};

		applyRootOutletRuntimeState(globals, {
			activeComponents: [ParentComp, ChildA],
			importURLs: ["/parent.js", "/child-a.js"],
			exportKeys: ["default", "default"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(container.textContent).toContain("child-a");

			const mountNodeBefore = container.querySelector(
				"[data-parent-mount-id]",
			);
			const draftNodeBefore = container.querySelector(
				"[data-parent-draft]",
			);
			expect(mountNodeBefore?.textContent).toBe("1");
			expect(draftNodeBefore?.textContent).toBe("initial");

			const input = container.querySelector(
				"input",
			) as HTMLInputElement | null;
			if (!input) {
				throw new Error("Expected parent input");
			}
			await act(async () => {
				input.value = "typed-value";
				input.dispatchEvent(new Event("input", { bubbles: true }));
			});
			expect(
				container.querySelector("[data-parent-draft]")?.textContent,
			).toBe("typed-value");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildB],
				importURLs: ["/parent.js", "/child-b.js"],
				exportKeys: ["default", "default"],
			});
			await act(async () => {
				dispatchRouteChangeWithScrollState({ x: 21, y: 22 });
			});

			expect(container.textContent).toContain("child-b");
			expect(
				container.querySelector("[data-parent-draft]")?.textContent,
			).toBe("typed-value");
			expect(
				container.querySelector("[data-parent-mount-id]")?.textContent,
			).toBe("1");
			expect(parentMountCount).toBe(1);
			expect(parentUnmountCount).toBe(0);
			expect(childARenderCount).toBeGreaterThan(0);
			expect(childBRenderCount).toBeGreaterThan(0);
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

	it("react remounts child when child export key changes even if import URL stays the same", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const reactAdapter = await import("vorma/react");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		const originalActEnvironment = (globalThis as any)
			.IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

		let childMountCount = 0;
		let childUnmountCount = 0;

		const ChildComp = () => {
			const mountIDRef = React.useRef<number | null>(null);
			if (mountIDRef.current === null) {
				childMountCount += 1;
				mountIDRef.current = childMountCount;
			}
			React.useEffect(() => {
				return () => {
					childUnmountCount += 1;
				};
			}, []);
			return React.createElement(
				"div",
				{},
				`child:${mountIDRef.current}`,
			);
		};
		const ParentComp = (props: { Outlet: any }) =>
			React.createElement(props.Outlet, {});

		applyRootOutletRuntimeState(globals, {
			activeComponents: [ParentComp, ChildComp],
			importURLs: ["/parent.js", "/child.js"],
			exportKeys: ["default", "child-a"],
		});

		try {
			await act(async () => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			expect(container.textContent).toContain("child:1");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildComp],
				importURLs: ["/parent.js", "/child.js"],
				exportKeys: ["default", "child-b"],
			});
			await act(async () => {
				dispatchRouteChangeWithScrollState({ x: 11, y: 12 });
			});

			expect(container.textContent).toContain("child:2");
			expect(childUnmountCount).toBe(1);
			expect(scrollToSpy).toHaveBeenCalledWith(11, 12);
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

	it("preact root outlet preserves child instance until child module identity changes and ignores location events", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const preactAdapter = await import("vorma/preact");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();

		let parentRenderCount = 0;
		let childRenderCount = 0;
		let childMountCount = 0;
		let childUnmountCount = 0;

		const ChildComp = () => {
			const mountIDRef = useRef<number | null>(null);
			if (mountIDRef.current === null) {
				childMountCount += 1;
				mountIDRef.current = childMountCount;
			}
			childRenderCount += 1;
			useEffect(() => {
				return () => {
					childUnmountCount += 1;
				};
			}, []);
			return h("div", {}, `child:${mountIDRef.current}`);
		};

		const ParentComp = (props: { idx: number; Outlet: any }) => {
			parentRenderCount += 1;
			return h(
				"section",
				{},
				h("span", {}, `parent:${props.idx}`),
				h(props.Outlet, {}),
			);
		};

		const stableActiveComponents = [ParentComp, ChildComp];
		const stableImportURLs = ["/parent.js", "/child-a.js"];
		const stableExportKeys = ["default", "default"];

		applyRootOutletRuntimeState(globals, {
			activeComponents: stableActiveComponents,
			importURLs: stableImportURLs,
			exportKeys: stableExportKeys,
		});

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
			expect(container.textContent).toContain("child:1");

			const parentRendersAfterInitial = parentRenderCount;
			const childRendersAfterInitial = childRenderCount;

			await actPreact(async () => {
				dispatchLocationChange();
			});

			expect(parentRenderCount).toBe(parentRendersAfterInitial);
			expect(childRenderCount).toBe(childRendersAfterInitial);

			await actPreact(async () => {
				dispatchRouteChangeWithScrollState({ x: 1, y: 2 });
			});

			expect(container.textContent).toContain("child:1");
			expect(parentRenderCount).toBe(parentRendersAfterInitial);
			expect(childRenderCount).toBe(childRendersAfterInitial);
			expect(childUnmountCount).toBe(0);
			expect(scrollToSpy).toHaveBeenCalledWith(1, 2);

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildComp],
				importURLs: ["/parent.js", "/child-b.js"],
				exportKeys: ["default", "default"],
			});
			await actPreact(async () => {
				dispatchRouteChangeWithScrollState({ x: 3, y: 4 });
			});

			expect(container.textContent).toContain("child:2");
			expect(childUnmountCount).toBe(1);
			expect(scrollToSpy).toHaveBeenCalledWith(3, 4);
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("preact root outlet uses the freshest child component when route updates keep the same child key", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const preactAdapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		let childARenderCount = 0;
		let childBRenderCount = 0;

		const ParentComp = (props: { Outlet: any }) => h(props.Outlet, {});
		const ChildA = () => {
			childARenderCount += 1;
			return h("div", {}, "child-a");
		};
		const ChildB = () => {
			childBRenderCount += 1;
			return h("div", {}, "child-b");
		};

		applyRootOutletRuntimeState(globals, {
			activeComponents: [ParentComp, ChildA],
			importURLs: ["/parent.js", "/child.js"],
			exportKeys: ["default", "default"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					container,
				);
			});
			expect(container.textContent).toContain("child-a");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildB],
				importURLs: ["/parent.js", "/child.js"],
				exportKeys: ["default", "default"],
			});
			await actPreact(async () => {
				dispatchRouteChangeWithScrollState({ x: 5, y: 6 });
			});

			expect(container.textContent).toContain("child-b");
			expect(childARenderCount).toBeGreaterThan(0);
			expect(childBRenderCount).toBeGreaterThan(0);
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("preact root outlet uses the freshest root component when route updates keep the same root key", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const preactAdapter = await import("vorma/preact");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		let rootARenderCount = 0;
		let rootBRenderCount = 0;

		const RootA = () => {
			rootARenderCount += 1;
			return h("div", {}, "root-a");
		};
		const RootB = () => {
			rootBRenderCount += 1;
			return h("div", {}, "root-b");
		};

		applyRootOutletRuntimeState(globals, {
			activeComponents: [RootA],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{}],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					container,
				);
			});
			expect(container.textContent).toContain("root-a");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [RootB],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
			});
			await actPreact(async () => {
				dispatchRouteChangeWithScrollState({ x: 31, y: 32 });
			});

			expect(container.textContent).toContain("root-b");
			expect(rootARenderCount).toBeGreaterThan(0);
			expect(rootBRenderCount).toBeGreaterThan(0);
			expect(scrollToSpy).toHaveBeenCalledWith(31, 32);
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("preact keeps parent instance and local parent input state when only child route changes", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const preactAdapter = await import("vorma/preact");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		let parentMountCount = 0;
		let parentUnmountCount = 0;
		let childARenderCount = 0;
		let childBRenderCount = 0;

		const ChildA = () => {
			childARenderCount += 1;
			return h("div", {}, "child-a");
		};
		const ChildB = () => {
			childBRenderCount += 1;
			return h("div", {}, "child-b");
		};

		const ParentComp = (props: { Outlet: any }) => {
			const mountIDRef = useRef<number | null>(null);
			if (mountIDRef.current === null) {
				parentMountCount += 1;
				mountIDRef.current = parentMountCount;
			}
			useEffect(() => {
				return () => {
					parentUnmountCount += 1;
				};
			}, []);

			const [draft, setDraft] = useState("initial");
			return h(
				"section",
				{},
				h(
					"div",
					{ "data-parent-mount-id": true },
					String(mountIDRef.current),
				),
				h("div", { "data-parent-draft": true }, draft),
				h("input", {
					value: draft,
					onInput: (event: Event) => {
						const target = event.target as HTMLInputElement;
						setDraft(target.value);
					},
				}),
				h(props.Outlet, {}),
			);
		};

		applyRootOutletRuntimeState(globals, {
			activeComponents: [ParentComp, ChildA],
			importURLs: ["/parent.js", "/child-a.js"],
			exportKeys: ["default", "default"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					container,
				);
			});
			expect(container.textContent).toContain("child-a");
			expect(
				container.querySelector("[data-parent-mount-id]")?.textContent,
			).toBe("1");
			expect(
				container.querySelector("[data-parent-draft]")?.textContent,
			).toBe("initial");

			const input = container.querySelector(
				"input",
			) as HTMLInputElement | null;
			if (!input) {
				throw new Error("Expected parent input");
			}
			await actPreact(async () => {
				input.value = "typed-value";
				input.dispatchEvent(new Event("input", { bubbles: true }));
			});
			expect(
				container.querySelector("[data-parent-draft]")?.textContent,
			).toBe("typed-value");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildB],
				importURLs: ["/parent.js", "/child-b.js"],
				exportKeys: ["default", "default"],
			});
			await actPreact(async () => {
				dispatchRouteChangeWithScrollState({ x: 21, y: 22 });
			});

			expect(container.textContent).toContain("child-b");
			expect(
				container.querySelector("[data-parent-draft]")?.textContent,
			).toBe("typed-value");
			expect(
				container.querySelector("[data-parent-mount-id]")?.textContent,
			).toBe("1");
			expect(parentMountCount).toBe(1);
			expect(parentUnmountCount).toBe(0);
			expect(childARenderCount).toBeGreaterThan(0);
			expect(childBRenderCount).toBeGreaterThan(0);
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("preact remounts child when child export key changes even if import URL stays the same", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const preactAdapter = await import("vorma/preact");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		let childMountCount = 0;
		let childUnmountCount = 0;

		const ChildComp = () => {
			const mountIDRef = useRef<number | null>(null);
			if (mountIDRef.current === null) {
				childMountCount += 1;
				mountIDRef.current = childMountCount;
			}
			useEffect(() => {
				return () => {
					childUnmountCount += 1;
				};
			}, []);
			return h("div", {}, `child:${mountIDRef.current}`);
		};
		const ParentComp = (props: { Outlet: any }) => h(props.Outlet, {});

		applyRootOutletRuntimeState(globals, {
			activeComponents: [ParentComp, ChildComp],
			importURLs: ["/parent.js", "/child.js"],
			exportKeys: ["default", "child-a"],
		});

		try {
			await actPreact(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					container,
				);
			});
			expect(container.textContent).toContain("child:1");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildComp],
				importURLs: ["/parent.js", "/child.js"],
				exportKeys: ["default", "child-b"],
			});
			await actPreact(async () => {
				dispatchRouteChangeWithScrollState({ x: 11, y: 12 });
			});

			expect(container.textContent).toContain("child:2");
			expect(childUnmountCount).toBe(1);
			expect(scrollToSpy).toHaveBeenCalledWith(11, 12);
		} finally {
			renderPreact(null, container);
			container.remove();
			restore();
		}
	});

	it("solid root outlet preserves child instance until child module identity changes and ignores location events", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const solidAdapter = await import("vorma/solid");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();

		let parentRunCount = 0;
		let childMountCount = 0;
		let childCleanupCount = 0;

		const ChildComp = () => {
			childMountCount += 1;
			const mountID = childMountCount;
			onCleanup(() => {
				childCleanupCount += 1;
			});
			return `child:${mountID}`;
		};

		const ParentComp = (props: { Outlet: any }) => {
			parentRunCount += 1;
			return createComponent(props.Outlet as any, {});
		};

		const stableActiveComponents = [ParentComp, ChildComp];
		const stableImportURLs = ["/parent.js", "/child-a.js"];
		const stableExportKeys = ["default", "default"];

		applyRootOutletRuntimeState(globals, {
			activeComponents: stableActiveComponents,
			importURLs: stableImportURLs,
			exportKeys: stableExportKeys,
		});

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolid(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			expect(container.textContent).toContain("child:1");

			const parentRunsAfterInitial = parentRunCount;
			const childMountsAfterInitial = childMountCount;

			dispatchLocationChange();
			await waitForCondition(() => {
				expect(parentRunCount).toBe(parentRunsAfterInitial);
				expect(childMountCount).toBe(childMountsAfterInitial);
			});

			dispatchRouteChangeWithScrollState({ x: 1, y: 2 });
			await waitForCondition(() => {
				expect(container.textContent).toContain("child:1");
				expect(scrollToSpy).toHaveBeenCalledWith(1, 2);
			});
			expect(childCleanupCount).toBe(0);

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildComp],
				importURLs: ["/parent.js", "/child-b.js"],
				exportKeys: ["default", "default"],
			});
			dispatchRouteChangeWithScrollState({ x: 3, y: 4 });
			await waitForCondition(() => {
				expect(container.textContent).toContain("child:2");
				expect(scrollToSpy).toHaveBeenCalledWith(3, 4);
			});
			expect(childCleanupCount).toBe(1);
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("solid remounts child when child export key changes even if import URL stays the same", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const solidAdapter = await import("vorma/solid");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		let childMountCount = 0;
		let childCleanupCount = 0;

		const ChildComp = () => {
			childMountCount += 1;
			const mountID = childMountCount;
			onCleanup(() => {
				childCleanupCount += 1;
			});
			return `child:${mountID}`;
		};
		const ParentComp = (props: { Outlet: any }) =>
			createComponent(props.Outlet as any, {});

		applyRootOutletRuntimeState(globals, {
			activeComponents: [ParentComp, ChildComp],
			importURLs: ["/parent.js", "/child.js"],
			exportKeys: ["default", "child-a"],
		});

		const dispose = renderSolid(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			expect(container.textContent).toContain("child:1");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildComp],
				importURLs: ["/parent.js", "/child.js"],
				exportKeys: ["default", "child-b"],
			});
			dispatchRouteChangeWithScrollState({ x: 11, y: 12 });
			await waitForCondition(() => {
				expect(container.textContent).toContain("child:2");
				expect(scrollToSpy).toHaveBeenCalledWith(11, 12);
			});

			expect(childCleanupCount).toBe(1);
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("solid root outlet uses the freshest child component when route updates keep the same child key", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const solidAdapter = await import("vorma/solid");
		const { restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		let childARunCount = 0;
		let childBRunCount = 0;

		const ParentComp = (props: { Outlet: any }) =>
			createComponent(props.Outlet as any, {});
		const ChildA = () => {
			childARunCount += 1;
			return "child-a";
		};
		const ChildB = () => {
			childBRunCount += 1;
			return "child-b";
		};

		applyRootOutletRuntimeState(globals, {
			activeComponents: [ParentComp, ChildA],
			importURLs: ["/parent.js", "/child.js"],
			exportKeys: ["default", "default"],
		});

		const dispose = renderSolid(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			expect(container.textContent).toContain("child-a");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildB],
				importURLs: ["/parent.js", "/child.js"],
				exportKeys: ["default", "default"],
			});
			dispatchRouteChangeWithScrollState({ x: 5, y: 6 });
			await waitForCondition(() => {
				expect(container.textContent).toContain("child-b");
			});
			expect(childARunCount).toBeGreaterThan(0);
			expect(childBRunCount).toBeGreaterThan(0);
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("solid root outlet uses the freshest root component when route updates keep the same root key", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const solidAdapter = await import("vorma/solid");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		let rootARunCount = 0;
		let rootBRunCount = 0;

		const RootA = () => {
			rootARunCount += 1;
			return "root-a";
		};
		const RootB = () => {
			rootBRunCount += 1;
			return "root-b";
		};

		applyRootOutletRuntimeState(globals, {
			activeComponents: [RootA],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
			loadersData: [{}],
		});

		const dispose = renderSolid(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			expect(container.textContent).toContain("root-a");

			applyRootOutletRuntimeState(globals, {
				activeComponents: [RootB],
				importURLs: ["/root.js"],
				exportKeys: ["default"],
				loadersData: [{}],
			});
			dispatchRouteChangeWithScrollState({ x: 31, y: 32 });
			await waitForCondition(() => {
				expect(container.textContent).toContain("root-b");
				expect(scrollToSpy).toHaveBeenCalledWith(31, 32);
			});

			expect(rootARunCount).toBeGreaterThan(0);
			expect(rootBRunCount).toBeGreaterThan(0);
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});

	it("solid keeps parent instance and local parent input state when only child route changes", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const solidAdapter = await import("vorma/solid");
		const { scrollToSpy, restore } = installImmediateRAFAndScrollSpy();
		const container = document.createElement("div");
		document.body.appendChild(container);

		let parentMountCount = 0;
		let parentCleanupCount = 0;
		let childRunCount = 0;
		const ChildComp = () => {
			childRunCount += 1;
			return "child-route-node";
		};

		const ParentComp = (props: { Outlet: any }) => {
			parentMountCount += 1;
			onCleanup(() => {
				parentCleanupCount += 1;
			});

			let draft = "initial";
			const section = document.createElement("section");
			const mountNode = document.createElement("div");
			mountNode.setAttribute("data-parent-mount-id", "true");
			mountNode.textContent = String(parentMountCount);
			const draftNode = document.createElement("div");
			draftNode.setAttribute("data-parent-draft", "true");
			draftNode.textContent = draft;
			const inputNode = document.createElement("input");
			inputNode.value = draft;
			inputNode.addEventListener("input", () => {
				draft = inputNode.value;
				draftNode.textContent = draft;
			});
			section.appendChild(mountNode);
			section.appendChild(draftNode);
			section.appendChild(inputNode);

			return [section, createComponent(props.Outlet as any, {})];
		};

		applyRootOutletRuntimeState(globals, {
			activeComponents: [ParentComp, ChildComp],
			importURLs: ["/parent.js", "/child-a.js"],
			exportKeys: ["default", "default"],
		});

		const dispose = renderSolid(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			expect(container.textContent).toContain("child-route-node");
			expect(
				container.querySelector("[data-parent-draft]")?.textContent,
			).toBe("initial");
			expect(
				container.querySelector("[data-parent-mount-id]")?.textContent,
			).toBe("1");

			const input = container.querySelector(
				"input",
			) as HTMLInputElement | null;
			if (!input) {
				throw new Error("Expected parent input");
			}
			input.value = "typed-value";
			input.dispatchEvent(new Event("input", { bubbles: true }));
			await waitForCondition(() => {
				expect(
					container.querySelector("[data-parent-draft]")?.textContent,
				).toBe("typed-value");
			});

			applyRootOutletRuntimeState(globals, {
				activeComponents: [ParentComp, ChildComp],
				importURLs: ["/parent.js", "/child-b.js"],
				exportKeys: ["default", "default"],
			});
			dispatchRouteChangeWithScrollState({ x: 21, y: 22 });
			await waitForCondition(() => {
				expect(scrollToSpy).toHaveBeenCalledWith(21, 22);
			});

			expect(
				container.querySelector("[data-parent-draft]")?.textContent,
			).toBe("typed-value");
			expect(
				container.querySelector("[data-parent-mount-id]")?.textContent,
			).toBe("1");
			expect(parentMountCount).toBe(1);
			expect(parentCleanupCount).toBe(0);
			expect(childRunCount).toBeGreaterThan(0);
		} finally {
			dispose();
			container.remove();
			restore();
		}
	});
});
