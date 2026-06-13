// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	BUILD_ID_HEADER,
	DATA_SCRIPT_ID,
	LINK_ACTIVE_ANCESTOR_ATTR,
	LINK_ACTIVE_EXACT_ATTR,
	LINK_PENDING_ANCESTOR_ATTR,
	LINK_PENDING_EXACT_ATTR,
	set_client_matcher_factory_for_test,
	type ClientMatcher,
	type ClientMatcherNestedMatch,
	type ClientOptions,
	type RevalidationResult,
} from "vorma/__internal";

/////// Harness type

type TestRouteProps = {
	idx: number;
	Outlet: (local?: Record<string, unknown>) => unknown;
};

type TestClientLoaderProps = {
	trigger: "boot" | "navigation" | "revalidation" | "prefetch";
	href: string;
	historyState: unknown;
	pattern: string;
	params: Record<string, string>;
	splatValues: string[];
	input: unknown;
	knownMatches: Array<{
		pattern: string;
		input: unknown;
	}>;
	serverPromise: Promise<{
		clientBuildId: string;
		matches: Array<{
			pattern: string;
			input: unknown;
			viewData: unknown;
		}>;
		outermostServerError: null | {
			idx: number;
			error: unknown;
		};
		viewData: unknown;
	}>;
	signal: AbortSignal;
};

type TestRouteTarget =
	| ({ href: string } & Record<string, unknown>)
	| ({ pattern: string } & Record<string, unknown>);

type TestVormaClient = {
	boot: () => Promise<void>;

	getRouteState: () => unknown;

	getWorkState: () => {
		navigation: null | { href: string };
		prefetch: null | { href: string };
	};

	navigate: (props: TestRouteTarget) => Promise<{ didNavigate: boolean }>;

	prefetch: (target: TestRouteTarget) => void;

	cancelPrefetch: (target: TestRouteTarget) => void;

	revalidate: () => Promise<RevalidationResult>;

	RootOutlet: (props?: { idx?: number } & Record<string, unknown>) => unknown;

	Link: (props: { href: unknown } & Record<string, unknown>) => unknown;

	useViewData: (props: { idx: number } & Record<string, unknown>) => unknown;

	usePatternViewData: (pattern: string) => unknown;

	useRouteState: (...args: any[]) => unknown;

	useWorkState: (...args: any[]) => unknown;

	useRouteSync: (...args: any[]) => void;

	useClientLoaderData: (props: { idx: number } & Record<string, unknown>) => unknown;

	usePatternClientLoaderData: (pattern: string) => unknown;

	defineView: (input: {
		pattern: string;
		component: (props: TestRouteProps) => unknown;
		errorBoundary?: (props: { error: unknown }) => unknown;
		clientLoader?: (props: TestClientLoaderProps) => Promise<unknown>;
		runClientLoaderOnHmr?: boolean;
	}) => {
		pattern: string;
		component: (props: TestRouteProps) => unknown;
		error_boundary?: (props: { error: unknown }) => unknown;
		client_loader?: (props: TestClientLoaderProps) => Promise<unknown>;
	};
};

type TestViewScope = {
	viewData: (props: { idx: number } & Record<string, unknown>) => unknown;
	patternViewData: (pattern: string) => unknown;
	routeState: {
		(): unknown;
		<T>(selector: (state: any) => T): T;
	};
	workState: {
		(): unknown;
		<T>(selector: (state: any) => T): T;
	};
	routeSync: (target: TestRouteTarget) => void;
	clientLoaderData: (props: { idx: number } & Record<string, unknown>) => unknown;
	patternClientLoaderData: (pattern: string) => unknown;
};

type TestCreateClientOptions = {
	linkDefaultProps?: Record<string, unknown>;
} & Omit<Partial<ClientOptions>, "render"> & {
		render?: (args: {
			RootOutlet: unknown;
			rootEl: HTMLElement;
		}) => void | Promise<void>;
	};

type TestCreateClient = (
	config: Record<string, never>,
	options?: TestCreateClientOptions,
) => TestVormaClient;

type TestElementFactory = (
	type: unknown,
	props?: Record<string, unknown> | null,
	...children: unknown[]
) => unknown;

type TestMount = () => {
	container: HTMLElement;
	render: (element: unknown) => void;
	cleanup: () => void;
};

export type AdapterTestHarness = {
	create_client: TestCreateClient;

	h: TestElementFactory;

	create_define_view_component: <P extends object>(
		render: (props: P) => unknown,
	) => unknown;

	create_view: (input: {
		client: TestVormaClient;
		pattern: string;
		render: (args: { props: TestRouteProps; v: TestViewScope }) => unknown;
		clientLoader?: (props: TestClientLoaderProps) => Promise<unknown>;
	}) => {
		pattern: string;
		component: (props: TestRouteProps) => unknown;
		error_boundary?: (props: { error: unknown }) => unknown;
		client_loader?: (props: TestClientLoaderProps) => Promise<unknown>;
	};

	dynamic: (read_value: () => unknown) => unknown;

	mount: TestMount;

	use_ref: <T>(initial: T) => { current: T };

	create_identity_parent: () => {
		component: (props: TestRouteProps) => unknown;
		get_mount_count: () => number;
		get_unmount_count: () => number;
	};

	create_hmr_stateful_component: (input: {
		hmr_id: string;
		version: string;
		on_mount: () => void;
		on_unmount: () => void;
	}) => (props: TestRouteProps) => unknown;

	create_hmr_version_component: (input: {
		hmr_id: string;
		version: string;
	}) => (props: TestRouteProps) => unknown;

	unwrap: <T>(value: T | (() => T)) => T;
};

/////// Helpers

export const TEST_CONFIG = {} as any;

export function seed_payload(overrides: Record<string, unknown> = {}) {
	const data = {
		client_build_id: "build-1",
		deployment_id: "",
		matched_patterns: [],
		views_data: [],
		import_urls: [],
		params: {},
		splat_values: [],
		title: undefined,
		meta_head_els: [],
		rest_head_els: [],
		css_bundles: [],
		deps: [],
		...overrides,
	};
	const script = document.createElement("script");
	script.id = DATA_SCRIPT_ID;
	script.type = "application/json";
	script.textContent = JSON.stringify(data);
	document.head.appendChild(script);
}

export function route_response(
	overrides: Record<string, unknown> = {},
	build_id = "build-1",
): Response {
	const data = {
		matched_patterns: [],
		views_data: [],
		import_urls: [],
		params: {},
		splat_values: [],
		title: undefined,
		meta_head_els: [],
		rest_head_els: [],
		css_bundles: [],
		deps: [],
		...overrides,
	};
	return new Response(JSON.stringify(data), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			[BUILD_ID_HEADER]: build_id,
		},
	});
}

export async function wait_for_dom(assertion: () => void, max = 50) {
	for (let i = 0; i < max; i++) {
		try {
			assertion();
			return;
		} catch {
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}
	}
	assertion();
}

function deferred_response(): {
	promise: Promise<Response>;
	resolve: (response: Response) => void;
} {
	let resolve!: (response: Response) => void;
	const promise = new Promise<Response>((r) => {
		resolve = r;
	});
	return { promise, resolve };
}

function dispatch_link_intent_prefetch(anchor: HTMLAnchorElement): void {
	anchor.dispatchEvent(
		new Event("pointerover", {
			bubbles: true,
			cancelable: true,
		}),
	);
	anchor.dispatchEvent(
		new Event("pointerenter", {
			bubbles: true,
			cancelable: true,
		}),
	);
}

async function apply_hmr_view_update(
	url: string,
	mod: Record<string, unknown>,
): Promise<void> {
	const update = window.__vorma_hmr_view_update;
	expect(typeof update).toBe("function");
	await update?.(url, mod);
}

/////// Test definitions

export function define_adapter_tests(harness: AdapterTestHarness) {
	beforeEach(() => {
		vi.restoreAllMocks();
		vi.resetModules();
		document.head.innerHTML = "";
		document.body.innerHTML = "";
		window.history.replaceState({}, "", "/");
		vi.spyOn(window, "scrollTo").mockImplementation(() => {});
	});

	///////////////////////////////////////////////////////////////////
	/////// Render
	///////////////////////////////////////////////////////////////////

	describe("render", () => {
		it("passes RootOutlet and rootEl to render callback", async () => {
			seed_payload();
			let received_root_outlet: unknown;
			let received_root_el: HTMLElement | undefined;
			const client = harness.create_client(TEST_CONFIG, {
				render: ({ RootOutlet, rootEl }) => {
					received_root_outlet = RootOutlet;
					received_root_el = rootEl;
				},
			});

			await client.boot();

			expect(typeof received_root_outlet).toBe("function");
			expect(received_root_el).toBeInstanceOf(HTMLElement);
		});
	});

	///////////////////////////////////////////////////////////////////
	/////// RootOutlet rendering
	///////////////////////////////////////////////////////////////////

	describe("RootOutlet", () => {
		it("renders component from route entry module", async () => {
			vi.doMock("/root.js", () => {
				return {
					default: {
						pattern: "/",
						component: () => {
							return harness.h("div", {}, "root-content");
						},
					},
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{ root: true }],
				import_urls: ["/root.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toBe("root-content");
			} finally {
				cleanup();
			}
		});

		it("passes idx and Outlet to rendered component", async () => {
			vi.doMock("/props-check.js", () => {
				return {
					default: {
						pattern: "/",
						component: (props: any) => {
							return harness.h(
								"div",
								{},
								`idx:${props.idx},hasOutlet:${typeof props.Outlet === "function"}`,
							);
						},
					},
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{}],
				import_urls: ["/props-check.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toBe("idx:0,hasOutlet:true");
			} finally {
				cleanup();
			}
		});

		it("passes through when entry has no component and deeper entries exist", async () => {
			vi.doMock("/no-comp.js", () => {
				return {
					default: undefined,
				};
			});
			vi.doMock("/child.js", () => {
				return {
					default: {
						pattern: "/child",
						component: () => {
							return harness.h("div", {}, "child-content");
						},
					},
				};
			});

			seed_payload({
				matched_patterns: ["/", "/child"],
				views_data: [{}, {}],
				import_urls: ["/no-comp.js", "/child.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toBe("child-content");
			} finally {
				cleanup();
			}
		});

		it("renders empty when no entries match index", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toBe("");
			} finally {
				cleanup();
			}
		});

		it("defaults idx to 0 when omitted", async () => {
			vi.doMock("/default-idx.js", () => {
				return {
					default: {
						pattern: "/",
						component: (props: any) => {
							return harness.h("div", {}, `idx:${props.idx}`);
						},
					},
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{}],
				import_urls: ["/default-idx.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet));
				expect(container.textContent).toBe("idx:0");
			} finally {
				cleanup();
			}
		});

		it("renders error boundary from view module", async () => {
			vi.doMock("/error-route.js", () => {
				return {
					default: {
						pattern: "/error",
						component: () => {
							return harness.h("div", {}, "should-not-render");
						},
						error_boundary: (props: { error: unknown }) => {
							return harness.h("div", {}, `handled:${props.error as any}`);
						},
					},
				};
			});

			seed_payload({
				matched_patterns: ["/error"],
				views_data: [{}],
				import_urls: ["/error-route.js"],
				outermost_server_err_idx: 0,
				outermost_server_err: "boom",
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toBe("handled:boom");
				expect(container.textContent).not.toContain("should-not-render");
			} finally {
				cleanup();
			}
		});

		it("renders default error boundary when route has none", async () => {
			vi.doMock("/no-boundary.js", () => {
				return {
					default: {
						pattern: "/no-boundary",
						component: () => {
							return harness.h("div", {}, "should-not-render");
						},
					},
				};
			});

			seed_payload({
				matched_patterns: ["/no-boundary"],
				views_data: [{}],
				import_urls: ["/no-boundary.js"],
				outermost_server_err_idx: 0,
				outermost_server_err: "default-boom",
			});
			const default_boundary = (props: { error: unknown }) => {
				return harness.h("div", {}, `default:${props.error as any}`);
			};
			const client = harness.create_client(TEST_CONFIG, {
				defaultErrorBoundary: default_boundary,
			});

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toBe("default:default-boom");
			} finally {
				cleanup();
			}
		});

		it("renders child error boundary under live parent", async () => {
			vi.doMock("/parent-live.js", () => {
				return {
					default: {
						pattern: "/parent",
						component: (props: any) => {
							return harness.h(
								"section",
								{},
								harness.h(
									"div",
									{ "data-parent": "true" },
									"parent-node",
								),
								harness.h(props.Outlet, {}),
							);
						},
					},
				};
			});
			vi.doMock("/child-err.js", () => {
				return {
					default: {
						pattern: "/parent/child",
						component: () => {
							return harness.h("div", {}, "child-node");
						},
						error_boundary: (props: { error: unknown }) => {
							return harness.h(
								"div",
								{},
								`child-handled:${props.error as any}`,
							);
						},
					},
				};
			});

			seed_payload({
				matched_patterns: ["/parent", "/parent/child"],
				views_data: [{}, {}],
				import_urls: ["/parent-live.js", "/child-err.js"],
				outermost_server_err_idx: 1,
				outermost_server_err: "child-boom",
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.querySelector("[data-parent]")?.textContent).toBe(
					"parent-node",
				);
				expect(container.textContent).toContain("child-handled:child-boom");
				expect(container.textContent).not.toContain("child-node");
			} finally {
				cleanup();
			}
		});

		it("applies scroll after navigation commit", async () => {
			vi.doMock("/scroll-target.js", () => {
				return {
					default: {
						pattern: "/scrolled",
						component: () => {
							return harness.h("div", {}, "scrolled-content");
						},
					},
				};
			});

			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const scroll_spy = vi.spyOn(window, "scrollTo");
			// rAF used by some adapters (Solid) — mock to fire synchronously
			vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => {
				cb(0);
				return 0;
			});

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));

				vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
					route_response({
						matched_patterns: ["/scrolled"],
						views_data: [{}],
						import_urls: ["/scroll-target.js"],
					}),
				);
				await (client.navigate as any)({ pattern: "/scrolled" });

				await wait_for_dom(() => {
					expect(container.textContent).toBe("scrolled-content");
				});

				await wait_for_dom(() => {
					expect(scroll_spy).toHaveBeenCalledWith(0, 0);
				});
			} finally {
				cleanup();
			}
		});
	});

	///////////////////////////////////////////////////////////////////
	/////// Component identity
	///////////////////////////////////////////////////////////////////

	describe("component identity", () => {
		it("preserves parent instance when only child route changes", async () => {
			const parent = harness.create_identity_parent();

			vi.doMock("/id-parent.js", () => {
				return {
					default: {
						pattern: "/parent",
						component: parent.component,
					},
				};
			});
			vi.doMock("/id-child-a.js", () => {
				return {
					default: {
						pattern: "/parent/child",
						component: () => {
							return harness.h("div", {}, "child-a");
						},
					},
				};
			});
			vi.doMock("/id-child-b.js", () => {
				return {
					default: {
						pattern: "/parent/child",
						component: () => {
							return harness.h("div", {}, "child-b");
						},
					},
				};
			});

			seed_payload({
				matched_patterns: ["/parent", "/parent/child"],
				views_data: [{}, {}],
				import_urls: ["/id-parent.js", "/id-child-a.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toContain("child-a");
				expect(parent.get_mount_count()).toBe(1);

				// Modify parent state
				const btn = container.querySelector("[data-set]") as HTMLElement;
				btn.click();
				await wait_for_dom(() => {
					expect(container.querySelector("[data-draft]")?.textContent).toBe(
						"modified",
					);
				});

				// Navigate to change child
				vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
					route_response({
						matched_patterns: ["/parent", "/parent/child"],
						views_data: [{}, {}],
						import_urls: ["/id-parent.js", "/id-child-b.js"],
					}),
				);
				await (client.navigate as any)({
					pattern: "/parent/child",
				});

				await wait_for_dom(() => {
					expect(container.textContent).toContain("child-b");
				});

				expect(container.querySelector("[data-draft]")?.textContent).toBe(
					"modified",
				);
				expect(parent.get_mount_count()).toBe(1);
				expect(parent.get_unmount_count()).toBe(0);
			} finally {
				cleanup();
			}
		});
	});

	///////////////////////////////////////////////////////////////////
	/////// HMR
	///////////////////////////////////////////////////////////////////

	describe("HMR", () => {
		it("updates the active view implementation without remounting it", async () => {
			let client!: TestVormaClient;
			let mount_count = 0;
			let unmount_count = 0;
			const on_mount = () => {
				mount_count++;
			};
			const on_unmount = () => {
				unmount_count++;
			};
			const component_v1 = harness.create_hmr_stateful_component({
				hmr_id: "self",
				version: "one",
				on_mount,
				on_unmount,
			});

			vi.doMock("/hmr-self.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/",
						render: ({ props }) => {
							return component_v1(props);
						},
					}),
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{}],
				import_urls: ["/hmr-self.js"],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				await wait_for_dom(() => {
					expect(container.querySelector("[data-version]")?.textContent).toBe(
						"one",
					);
				});
				expect(mount_count).toBe(1);

				const button = container.querySelector("[data-set]") as HTMLElement;
				button.click();
				await wait_for_dom(() => {
					expect(container.querySelector("[data-draft]")?.textContent).toBe(
						"modified",
					);
				});

				const component_v2 = harness.create_hmr_stateful_component({
					hmr_id: "self",
					version: "two",
					on_mount,
					on_unmount,
				});
				await apply_hmr_view_update("/hmr-self.js", {
					default: harness.create_view({
						client,
						pattern: "/",
						render: ({ props }) => {
							return component_v2(props);
						},
					}),
				});

				await wait_for_dom(() => {
					expect(container.querySelector("[data-version]")?.textContent).toBe(
						"two",
					);
				});

				expect(container.querySelector("[data-draft]")?.textContent).toBe(
					"modified",
				);
				expect(mount_count).toBe(1);
				expect(unmount_count).toBe(0);
			} finally {
				cleanup();
			}
		});

		it("updates a child view implementation without remounting the parent view", async () => {
			let client!: TestVormaClient;
			let parent_mount_count = 0;
			let parent_unmount_count = 0;
			const parent = harness.create_hmr_stateful_component({
				hmr_id: "parent",
				version: "parent",
				on_mount: () => {
					parent_mount_count++;
				},
				on_unmount: () => {
					parent_unmount_count++;
				},
			});
			const child_v1 = harness.create_hmr_version_component({
				hmr_id: "child",
				version: "one",
			});

			vi.doMock("/hmr-parent.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/parent",
						render: ({ props }) => {
							return parent(props);
						},
					}),
				};
			});
			vi.doMock("/hmr-child.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/parent/child",
						render: ({ props }) => {
							return child_v1(props);
						},
					}),
				};
			});

			seed_payload({
				matched_patterns: ["/parent", "/parent/child"],
				views_data: [{}, {}],
				import_urls: ["/hmr-parent.js", "/hmr-child.js"],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				await wait_for_dom(() => {
					expect(
						container.querySelector("[data-child-version]")?.textContent,
					).toBe("one");
				});
				expect(parent_mount_count).toBe(1);

				const button = container.querySelector("[data-set]") as HTMLElement;
				button.click();
				await wait_for_dom(() => {
					expect(container.querySelector("[data-draft]")?.textContent).toBe(
						"modified",
					);
				});

				const child_v2 = harness.create_hmr_version_component({
					hmr_id: "child",
					version: "two",
				});
				await apply_hmr_view_update("/hmr-child.js", {
					default: harness.create_view({
						client,
						pattern: "/parent/child",
						render: ({ props }) => {
							return child_v2(props);
						},
					}),
				});

				await wait_for_dom(() => {
					expect(
						container.querySelector("[data-child-version]")?.textContent,
					).toBe("two");
				});
				expect(container.querySelector("[data-draft]")?.textContent).toBe(
					"modified",
				);
				expect(parent_mount_count).toBe(1);
				expect(parent_unmount_count).toBe(0);
			} finally {
				cleanup();
			}
		});
	});

	///////////////////////////////////////////////////////////////////
	/////// Data stability
	///////////////////////////////////////////////////////////////////

	describe("data stability", () => {
		it("does not remount component on data-only revalidation", async () => {
			let mount_count = 0;

			vi.doMock("/stable.js", () => {
				return {
					default: {
						pattern: "/",
						component: () => {
							const ref = harness.use_ref<number | null>(null);
							if (ref.current === null) {
								mount_count++;
								ref.current = mount_count;
							}
							return harness.h("div", {}, `mount:${ref.current}`);
						},
					},
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{ v: "initial" }],
				import_urls: ["/stable.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(mount_count).toBe(1);

				vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
					route_response({
						matched_patterns: ["/"],
						views_data: [{ v: "updated" }],
						import_urls: ["/stable.js"],
					}),
				);
				await client.revalidate();

				await wait_for_dom(() => {
					expect(container.textContent).toBe("mount:1");
				});
				expect(mount_count).toBe(1);
			} finally {
				cleanup();
			}
		});
	});

	///////////////////////////////////////////////////////////////////
	/////// Hooks
	///////////////////////////////////////////////////////////////////

	describe("hooks", () => {
		it("useViewData returns data at props.idx", async () => {
			let captured: unknown;
			let client!: TestVormaClient;

			vi.doMock("/hook-view-data.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/",
						render: ({ props, v }) => {
							captured = v.viewData(props);
							return harness.h("div", {}, "hook-test");
						},
					}),
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{ name: "Ada" }],
				import_urls: ["/hook-view-data.js"],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toEqual({ name: "Ada" });
			} finally {
				cleanup();
			}
		});

		it("usePatternViewData returns data for matching pattern", async () => {
			let captured: unknown;
			let client!: TestVormaClient;

			vi.doMock("/hook-pattern.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/",
						render: ({ v }) => {
							captured = v.patternViewData("/");
							return harness.h("div", {}, "pattern-test");
						},
					}),
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{ session: "abc" }],
				import_urls: ["/hook-pattern.js"],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toEqual({ session: "abc" });
			} finally {
				cleanup();
			}
		});

		it("usePatternViewData returns undefined for non-matching pattern", async () => {
			let captured: unknown = "sentinel";
			let client!: TestVormaClient;

			vi.doMock("/hook-no-match.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/",
						render: ({ v }) => {
							captured = v.patternViewData("/does-not-exist" as any);
							return harness.h("div", {}, "no-match-test");
						},
					}),
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{}],
				import_urls: ["/hook-no-match.js"],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toBeUndefined();
			} finally {
				cleanup();
			}
		});

		it("useRouteState returns route state", async () => {
			let captured: unknown;
			let client!: TestVormaClient;

			vi.doMock("/hook-router.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/users/:id",
						render: ({ v }) => {
							captured = v.routeState();
							return harness.h("div", {}, "router-test");
						},
					}),
				};
			});

			seed_payload({
				matched_patterns: ["/users/:id"],
				views_data: [{ user: "Ada" }],
				import_urls: ["/hook-router.js"],
				params: { id: "42" },
				splat_values: [],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				const data = harness.unwrap(captured) as any;
				expect(data.params).toEqual({ id: "42" });
				expect(data.clientBuildId).toBe("build-1");
				expect(data).toHaveProperty("historyState", undefined);
				expect(data.matches.map((m: any) => m.pattern)).toEqual(["/users/:id"]);
			} finally {
				cleanup();
			}
		});

		it("does not expose a committed route with matching pending navigation", async () => {
			const parent_pattern = "/atomic";
			const initial_child_path = "/atomic/a";
			const next_child_path = "/atomic/b";
			let client!: TestVormaClient;
			const observations: Array<{
				pending_path: string | null;
				route_path: string;
			}> = [];

			vi.doMock("/atomic-parent.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: parent_pattern,
						render: ({ props, v }) => {
							const read_observation = () => {
								const route_href = harness.unwrap(
									v.routeState((route) => {
										return route.href;
									}),
								) as string;
								const pending_href = harness.unwrap(
									v.workState((work) => {
										return work.navigation?.href ?? null;
									}),
								) as string | null;
								const route_path = new URL(
									route_href,
									window.location.href,
								).pathname;
								const pending_path = pending_href
									? new URL(pending_href, window.location.href).pathname
									: null;
								observations.push({
									pending_path,
									route_path,
								});
								return `${route_path}|${pending_path ?? ""}`;
							};
							return harness.h(
								"section",
								{
									"data-atomic-state":
										harness.dynamic(read_observation),
								},
								harness.h(props.Outlet, {}),
							);
						},
					}),
				};
			});
			vi.doMock("/atomic-child-a.js", () => {
				return {
					default: {
						pattern: initial_child_path,
						component: () => {
							return harness.h(
								"div",
								{ "data-atomic-child": "a" },
								"child-a",
							);
						},
					},
				};
			});
			vi.doMock("/atomic-child-b.js", () => {
				return {
					default: {
						pattern: next_child_path,
						component: () => {
							return harness.h(
								"div",
								{ "data-atomic-child": "b" },
								"child-b",
							);
						},
					},
				};
			});

			window.history.replaceState({}, "", initial_child_path);
			seed_payload({
				matched_patterns: [parent_pattern, initial_child_path],
				views_data: [{}, {}],
				import_urls: ["/atomic-parent.js", "/atomic-child-a.js"],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			let resolve_response!: (response: Response) => void;
			let response_resolved = false;
			const response_promise = new Promise<Response>((resolve) => {
				resolve_response = resolve;
			});
			const resolve_navigation_response = () => {
				if (response_resolved) {
					return;
				}
				response_resolved = true;
				resolve_response(
					route_response({
						matched_patterns: [parent_pattern, next_child_path],
						views_data: [{}, {}],
						import_urls: ["/atomic-parent.js", "/atomic-child-b.js"],
					}),
				);
			};
			vi.spyOn(globalThis, "fetch").mockReturnValueOnce(response_promise);

			const { container, render, cleanup } = harness.mount();
			let nav_promise: Promise<{ didNavigate: boolean }> | undefined;
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toContain("child-a");

				nav_promise = client.navigate({ href: next_child_path });
				await wait_for_dom(() => {
					expect(
						observations.some((observation) => {
							return (
								observation.route_path === initial_child_path &&
								observation.pending_path === next_child_path
							);
						}),
					).toBe(true);
				});

				resolve_navigation_response();
				await nav_promise;
				await wait_for_dom(() => {
					expect(container.textContent).toContain("child-b");
				});

				expect(
					observations.filter((observation) => {
						return (
							observation.route_path === next_child_path &&
							observation.pending_path === next_child_path
						);
					}),
				).toEqual([]);
			} finally {
				resolve_navigation_response();
				await nav_promise?.catch(() => {});
				cleanup();
			}
		});

		it("does not rerender the route tree when link intent prefetch starts", async () => {
			const root_pattern = "/";
			const prefetch_target_path = "/prefetch-granularity-target";
			const page_module_url = "/prefetch-granularity-page.js";
			let client!: TestVormaClient;
			let route_render_count = 0;

			vi.doMock(page_module_url, () => {
				return {
					default: {
						pattern: root_pattern,
						component: () => {
							route_render_count++;
							return harness.h(
								"main",
								{ "data-granularity-route": "true" },
								harness.h(client.Link as any, {
									pattern: prefetch_target_path,
									prefetch: "intent",
									prefetchDelayMs: 0,
									children: "Prefetch",
								}),
							);
						},
					},
				};
			});

			seed_payload({
				matched_patterns: [root_pattern],
				views_data: [{}],
				import_urls: [page_module_url],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const response = deferred_response();
			const fetch_mock = vi
				.spyOn(globalThis, "fetch")
				.mockReturnValueOnce(response.promise);

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(route_render_count).toBe(1);

				const anchor = container.querySelector("a")!;
				dispatch_link_intent_prefetch(anchor);

				await wait_for_dom(() => {
					expect(fetch_mock).toHaveBeenCalledTimes(1);
				});
				expect(route_render_count).toBe(1);

				await new Promise((resolve) => {
					return setTimeout(resolve, 0);
				});
				expect(route_render_count).toBe(1);
			} finally {
				response.resolve(
					route_response({
						matched_patterns: [prefetch_target_path],
						views_data: [{}],
					}),
				);
				cleanup();
			}
		});

		it("rerenders only work subscribers whose selected value changes when link intent prefetch starts", async () => {
			const root_pattern = "/";
			const prefetch_target_path = "/prefetch-selector-granularity-target";
			const page_module_url = "/prefetch-selector-granularity-page.js";
			let client!: TestVormaClient;
			let route_render_count = 0;
			let prefetch_render_count = 0;
			let navigation_render_count = 0;
			let prefetch_read_count = 0;
			let navigation_read_count = 0;

			vi.doMock(page_module_url, () => {
				const prefetch_subscriber = harness.create_view({
					client,
					pattern: root_pattern,
					render: ({ v }) => {
						prefetch_render_count++;
						const href = v.workState((work) => {
							return work.prefetch?.href ?? "none";
						});
						v.routeSync({ pattern: root_pattern });
						return harness.h(
							"output",
							{ "data-prefetch-work": "true" },
							harness.dynamic(() => {
								prefetch_read_count++;
								return String(harness.unwrap(href));
							}),
						);
					},
				});
				const navigation_subscriber = harness.create_view({
					client,
					pattern: root_pattern,
					render: ({ v }) => {
						navigation_render_count++;
						const href = v.workState((work) => {
							return work.navigation?.href ?? "none";
						});
						return harness.h(
							"output",
							{ "data-navigation-work": "true" },
							harness.dynamic(() => {
								navigation_read_count++;
								return String(harness.unwrap(href));
							}),
						);
					},
				});

				return {
					default: {
						pattern: root_pattern,
						component: () => {
							route_render_count++;
							return harness.h(
								"main",
								{},
								harness.h(prefetch_subscriber.component, {
									idx: 0,
									Outlet: () => {
										return null;
									},
								}),
								harness.h(navigation_subscriber.component, {
									idx: 0,
									Outlet: () => {
										return null;
									},
								}),
								harness.h(client.Link as any, {
									pattern: prefetch_target_path,
									prefetch: "intent",
									prefetchDelayMs: 0,
									children: "Prefetch",
								}),
							);
						},
					},
				};
			});

			seed_payload({
				matched_patterns: [root_pattern],
				views_data: [{}],
				import_urls: [page_module_url],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const response = deferred_response();
			const fetch_mock = vi
				.spyOn(globalThis, "fetch")
				.mockReturnValueOnce(response.promise);

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(route_render_count).toBe(1);
				expect(prefetch_render_count).toBe(1);
				expect(navigation_render_count).toBe(1);
				expect(prefetch_read_count).toBe(1);
				expect(navigation_read_count).toBe(1);

				const anchor = container.querySelector("a")!;
				dispatch_link_intent_prefetch(anchor);

				await wait_for_dom(() => {
					expect(fetch_mock).toHaveBeenCalledTimes(1);
				});

				await wait_for_dom(() => {
					expect(
						container.querySelector("[data-prefetch-work]")?.textContent,
					).toContain(prefetch_target_path);
				});

				expect(route_render_count).toBe(1);
				expect(prefetch_read_count).toBeGreaterThan(1);
				expect(navigation_read_count).toBe(1);
				expect(navigation_render_count).toBe(1);
			} finally {
				response.resolve(
					route_response({
						matched_patterns: [prefetch_target_path],
						views_data: [{}],
					}),
				);
				cleanup();
			}
		});

		it("useRouteSync navigates when route href differs", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
				route_response({ matched_patterns: ["/canonical"] }),
			);

			const Canonical = harness.create_view({
				client,
				pattern: "/",
				render: ({ v }) => {
					v.routeSync({
						pattern: "/canonical",
					});
					return harness.h("div", {}, "canonical");
				},
			});

			const { render, cleanup } = harness.mount();
			try {
				render(
					harness.h(Canonical.component, {
						idx: 0,
						Outlet: () => {
							return null;
						},
					}),
				);
				await wait_for_dom(() => {
					expect(window.location.pathname).toBe("/canonical");
				});
			} finally {
				cleanup();
			}
		});

		it("useRouteSync does not navigate when already synced", async () => {
			vi.doMock("/canonical.js", () => {
				return {
					default: {
						pattern: "/canonical",
						component: () => {
							return harness.h("div", {}, "canonical-route");
						},
					},
				};
			});
			window.history.replaceState({}, "", "/canonical");
			seed_payload({
				matched_patterns: ["/canonical"],
				views_data: [{}],
				import_urls: ["/canonical.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const fetch_spy = vi.spyOn(globalThis, "fetch");

			const Canonical = harness.create_view({
				client,
				pattern: "/canonical",
				render: ({ v }) => {
					v.routeSync({
						pattern: "/canonical",
					});
					return harness.h("div", {}, "canonical");
				},
			});

			const { render, cleanup } = harness.mount();
			try {
				render(
					harness.h(Canonical.component, {
						idx: 0,
						Outlet: () => {
							return null;
						},
					}),
				);
				for (let i = 0; i < 5; i++) {
					await new Promise((r) => {
						return setTimeout(r, 0);
					});
				}
				expect(fetch_spy).not.toHaveBeenCalled();
			} finally {
				cleanup();
			}
		});

		it("useRouteSync cancels a debounced navigation when the target returns to the current route", async () => {
			const amount_pattern = "/amount/:amount";
			/*
			Deterministic on REAL timers, by ordering rather than margins:
			adapters capture their effect schedulers (rAF / setTimeout) at
			module init, so fake timers cannot reach them. Instead the
			debounce is so large (60s) that no event-loop stall can fire
			it, and each flush wait (150ms) is longer than every adapter's
			scheduler delay (rAF ~16ms, preact's 100ms fallback) — real
			timers fire in due-time order, so the flush is GUARANTEED to
			land before the wait completes, no matter the load.
			*/
			const cancel_phase_delay_ms = 60_000;
			const fire_phase_delay_ms = 10;
			const effect_flush_wait_ms = 150;
			let route_sync_delay_ms = cancel_phase_delay_ms;
			let amount = "12";

			window.history.replaceState({}, "", "/amount/12");
			seed_payload({
				matched_patterns: [amount_pattern],
				views_data: [{}],
				params: { amount },
				splat_values: [],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const sync_view = harness.create_view({
				client,
				pattern: amount_pattern,
				render: ({ v }) => {
					v.routeSync({
						pattern: amount_pattern,
						params: { amount },
						debounceMs: route_sync_delay_ms,
					});
					return harness.h("output", { "data-route-sync-amount": "" }, amount);
				},
			});
			const fetch_mock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
				route_response({
					matched_patterns: [amount_pattern],
					views_data: [{}],
					params: { amount: "1" },
				}),
			);

			const { container, render, cleanup } = harness.mount();
			const render_sync = () => {
				render(
					harness.h(sync_view.component, {
						idx: 0,
						sync_amount: amount,
						Outlet: () => {
							return null;
						},
					}),
				);
			};
			const flush_effects = async () => {
				await new Promise((resolve) => {
					return setTimeout(resolve, effect_flush_wait_ms);
				});
			};
			try {
				render_sync();
				expect(container.textContent).toBe("12");

				amount = "1";
				render_sync();
				expect(container.textContent).toBe("1");
				// The debounce is now SCHEDULED (flush landed), 60s from due.
				await flush_effects();
				expect(fetch_mock).not.toHaveBeenCalled();

				amount = "12";
				render_sync();
				expect(container.textContent).toBe("12");
				// The cancelling effect's cleanup has cleared the timer.
				await flush_effects();

				expect(fetch_mock).not.toHaveBeenCalled();
				expect(window.location.pathname).toBe("/amount/12");

				/*
				Positive control: with a short debounce and no cancellation,
				the navigation MUST fire — proving the schedule/fire
				machinery runs in this harness, so the absence assertions
				above are meaningful rather than vacuous. Polled with a
				deadline: waiting FOR an event tolerates any load.
				*/
				route_sync_delay_ms = fire_phase_delay_ms;
				amount = "1";
				render_sync();
				for (let i = 0; i < 200 && fetch_mock.mock.calls.length === 0; i++) {
					await new Promise((resolve) => {
						return setTimeout(resolve, 25);
					});
				}
				expect(fetch_mock).toHaveBeenCalledTimes(1);
				const fetched = fetch_mock.mock.calls[0]?.[0];
				const fetched_url =
					fetched instanceof Request
						? fetched.url
						: String(fetched as string | URL);
				expect(fetched_url).toContain("/amount/1");
			} finally {
				cleanup();
			}
		});

		it("useClientLoaderData returns client loader data at props.idx", async () => {
			let captured: unknown = "sentinel";
			let client!: TestVormaClient;

			vi.doMock("/hook-cl.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/cl-test",
						render: ({ props, v }) => {
							captured = v.clientLoaderData(props);
							return harness.h("div", {}, "cl-test");
						},
						clientLoader: async ({ serverPromise }: any) => {
							/*
							serverPromise resolves with this view's OWN typed
							server output at `viewData` (plus the full chain in
							`matches`) — pinned because the port once dropped
							the field from the public type.
							*/
							const server = await serverPromise;
							return { enhanced: true, server_view_data: server.viewData };
						},
					}),
				};
			});

			seed_payload({
				matched_patterns: ["/cl-test"],
				views_data: [{ raw: "data" }],
				import_urls: ["/hook-cl.js"],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toEqual({
					enhanced: true,
					server_view_data: { raw: "data" },
				});
			} finally {
				cleanup();
			}
		});

		it("usePatternClientLoaderData returns undefined when no match", async () => {
			let captured: unknown = "sentinel";
			let client!: TestVormaClient;

			vi.doMock("/hook-pcl.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/",
						render: ({ v }) => {
							captured = v.patternClientLoaderData("/nonexistent" as any);
							return harness.h("div", {}, "pcl-test");
						},
					}),
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{}],
				import_urls: ["/hook-pcl.js"],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toBeUndefined();
			} finally {
				cleanup();
			}
		});

		it("hooks re-render when state changes via navigation", async () => {
			let client!: TestVormaClient;
			let read_current_value = (): unknown => {
				return undefined;
			};

			vi.doMock("/hook-rerender.js", () => {
				return {
					default: harness.create_view({
						client,
						pattern: "/",
						render: ({ v }) => {
							const data = v.viewData({
								idx: 0,
							} as any);
							read_current_value = () => {
								return harness.unwrap(data);
							};
							return harness.h(
								"div",
								{ "data-value": "true" },
								harness.dynamic(() => {
									return JSON.stringify(read_current_value());
								}),
							);
						},
					}),
				};
			});

			seed_payload({
				matched_patterns: ["/"],
				views_data: [{ v: "initial" }],
				import_urls: ["/hook-rerender.js"],
			});
			client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(read_current_value()).toEqual({ v: "initial" });

				vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
					route_response({
						matched_patterns: ["/"],
						views_data: [{ v: "updated" }],
						import_urls: ["/hook-rerender.js"],
					}),
				);
				await client.revalidate();

				await wait_for_dom(() => {
					expect(container.querySelector("[data-value]")?.textContent).toBe(
						'{"v":"updated"}',
					);
				});
				expect(read_current_value()).toEqual({ v: "updated" });
			} finally {
				cleanup();
			}
		});
	});

	///////////////////////////////////////////////////////////////////
	/////// Link
	///////////////////////////////////////////////////////////////////

	describe("Link", () => {
		it("renders anchor with href built from pattern and params", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/users/:id",
						params: { id: "42" },
						children: "User Link",
					}),
				);
				const anchor = container.querySelector("a");
				expect(anchor).not.toBeNull();
				expect(anchor!.textContent).toBe("User Link");
				expect(new URL(anchor!.href).pathname).toBe("/users/42");
			} finally {
				cleanup();
			}
		});

		it("appends search and hash to href", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/products/:id",
						params: { id: "7" },
						search: { tab: "reviews" },
						hash: "#top",
						children: "Product",
					}),
				);
				const anchor = container.querySelector("a");
				const url = new URL(anchor!.href);
				expect(url.pathname).toBe("/products/7");
				expect(url.searchParams.get("tab")).toBe("reviews");
				expect(url.hash).toBe("#top");
			} finally {
				cleanup();
			}
		});

		it("merges factory link defaults with per-link overrides", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG, {
				linkDefaultProps: { className: "default-class" },
			});

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/a",
						children: "A",
					}),
				);
				expect(container.querySelector("a")!.className).toBe("default-class");

				render(
					harness.h(client.Link as any, {
						pattern: "/b",
						className: "custom-class",
						children: "B",
					}),
				);
				expect(container.querySelector("a")!.className).toBe("custom-class");
			} finally {
				cleanup();
			}
		});

		it("adds exact and ancestor attributes for the current route", async () => {
			window.history.replaceState({}, "", "/products/7?tab=reviews#top");
			seed_payload({
				matched_patterns: ["/", "/products/:id"],
				views_data: [{}, {}],
				params: { id: "7" },
				splat_values: [],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/products/:id",
						params: { id: "7" },
						children: "Product",
					}),
				);
				let anchor = container.querySelector("a")!;
				expect(anchor.getAttribute(LINK_ACTIVE_EXACT_ATTR)).toBe("");
				expect(anchor.getAttribute("aria-current")).toBe("page");
				expect(anchor.getAttribute(LINK_ACTIVE_ANCESTOR_ATTR)).toBeNull();

				render(
					harness.h(client.Link as any, {
						pattern: "/",
						children: "Root",
					}),
				);
				anchor = container.querySelector("a")!;
				expect(anchor.getAttribute(LINK_ACTIVE_EXACT_ATTR)).toBeNull();
				expect(anchor.getAttribute(LINK_ACTIVE_ANCESTOR_ATTR)).toBe("");
				expect(anchor.getAttribute("aria-current")).toBeNull();
			} finally {
				cleanup();
			}
		});

		it("rerenders active ancestor attributes when matcher wasm becomes ready", async () => {
			window.history.replaceState({}, "", "/products/7");
			const registered_patterns = new Set<string>();
			const root_match: ClientMatcherNestedMatch = {
				params: {},
				splat_values: [],
				patterns: ["/"],
			};
			const product_match: ClientMatcherNestedMatch = {
				params: { id: "7" },
				splat_values: [],
				patterns: ["/", "/products/:id"],
			};
			const matcher: ClientMatcher = {
				register_pattern: (pattern) => {
					registered_patterns.add(pattern);
				},
				find_nested_matches: (path) => {
					if (path === "/" && registered_patterns.has("/")) {
						return root_match;
					}
					if (
						path === "/products/7" &&
						registered_patterns.has("/") &&
						registered_patterns.has("/products/:id")
					) {
						return product_match;
					}
					return null;
				},
				free: () => {},
			};
			let resolve_link_matcher!: (matcher: ClientMatcher) => void;
			let link_matcher_resolved = false;
			const link_matcher_ready = new Promise<ClientMatcher>((resolve) => {
				resolve_link_matcher = (next_matcher) => {
					link_matcher_resolved = true;
					resolve(next_matcher);
				};
			});
			// The matcher promise is controlled because this test asserts the
			// pre-ready to ready adapter rerender, not wasm matching itself.
			const restore_client_matcher_factory = set_client_matcher_factory_for_test(
				() => {
					return link_matcher_ready;
				},
			);

			let cleanup: (() => void) | null = null;
			try {
				seed_payload({
					matched_patterns: ["/", "/products/:id"],
					views_data: [{}, {}],
					params: { id: "7" },
					splat_values: [],
				});
				const client = harness.create_client(TEST_CONFIG);

				await client.boot();

				const mounted = harness.mount();
				cleanup = mounted.cleanup;
				mounted.render(
					harness.h(client.Link as any, {
						href: "/",
						children: "Products",
					}),
				);
				expect(
					mounted.container
						.querySelector("a")
						?.getAttribute(LINK_ACTIVE_ANCESTOR_ATTR),
				).toBeNull();

				resolve_link_matcher(matcher);

				await wait_for_dom(() => {
					expect(
						mounted.container
							.querySelector("a")
							?.getAttribute(LINK_ACTIVE_ANCESTOR_ATTR),
					).toBe("");
				});
				expect(
					mounted.container
						.querySelector("a")
						?.getAttribute(LINK_ACTIVE_EXACT_ATTR),
				).toBeNull();
			} finally {
				if (!link_matcher_resolved) {
					resolve_link_matcher(matcher);
					await Promise.resolve();
				}
				cleanup?.();
				restore_client_matcher_factory();
			}
		});

		it("uses attribute match rules for search and hash exactness", async () => {
			window.history.replaceState({}, "", "/products/7?tab=reviews#top");
			seed_payload({
				matched_patterns: ["/products/:id"],
				views_data: [{}],
				params: { id: "7" },
				splat_values: [],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/products/:id",
						params: { id: "7" },
						search: { tab: "details" },
						hash: "#other",
						children: "Product",
					}),
				);
				let anchor = container.querySelector("a")!;
				expect(anchor.getAttribute(LINK_ACTIVE_EXACT_ATTR)).toBe("");

				render(
					harness.h(client.Link as any, {
						pattern: "/products/:id",
						params: { id: "7" },
						search: { tab: "details" },
						hash: "#other",
						attributeMatchRules: {
							includeSearch: true,
							includeHash: true,
						},
						children: "Product",
					}),
				);
				anchor = container.querySelector("a")!;
				expect(anchor.getAttribute(LINK_ACTIVE_EXACT_ATTR)).toBeNull();

				render(
					harness.h(client.Link as any, {
						pattern: "/products/:id",
						params: { id: "7" },
						attributeMatchRules: { skip: true },
						children: "Product",
					}),
				);
				anchor = container.querySelector("a")!;
				expect(anchor.getAttribute(LINK_ACTIVE_EXACT_ATTR)).toBeNull();
				expect(anchor.getAttribute(LINK_ACTIVE_ANCESTOR_ATTR)).toBeNull();
				expect(anchor.getAttribute("aria-current")).toBeNull();
			} finally {
				cleanup();
			}
		});

		it("adds pending attributes for the pending navigation route", async () => {
			seed_payload({
				matched_patterns: ["/"],
				views_data: [{}],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			let resolve_response!: (response: Response) => void;
			const response_promise = new Promise<Response>((resolve) => {
				resolve_response = resolve;
			});
			vi.spyOn(globalThis, "fetch").mockReturnValueOnce(response_promise);

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(
						"div",
						{},
						harness.h(client.Link as any, {
							pattern: "/pending",
							children: "Pending",
						}),
						harness.h(client.Link as any, {
							pattern: "/",
							children: "Root",
						}),
					),
				);
				const anchors = container.querySelectorAll("a");
				const pending_anchor = anchors[0]!;
				const root_anchor = anchors[1]!;
				pending_anchor.dispatchEvent(
					new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					}),
				);

				await wait_for_dom(() => {
					expect(pending_anchor.getAttribute(LINK_PENDING_EXACT_ATTR)).toBe("");
					expect(root_anchor.getAttribute(LINK_PENDING_ANCESTOR_ATTR)).toBe("");
				});
			} finally {
				resolve_response(
					route_response({
						matched_patterns: ["/pending"],
						views_data: [{}],
					}),
				);
				cleanup();
			}
		});

		it("navigates on eligible internal click", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
				route_response({ matched_patterns: ["/clicked"] }),
			);

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/clicked",
						children: "Click Me",
					}),
				);
				const anchor = container.querySelector("a")!;
				const click = new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
					button: 0,
				});
				anchor.dispatchEvent(click);

				await wait_for_dom(() => {
					expect(window.location.pathname).toBe("/clicked");
				});
				expect(click.defaultPrevented).toBe(true);
			} finally {
				cleanup();
			}
		});

		it("does not navigate for modified clicks", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const fetch_spy = vi.spyOn(globalThis, "fetch");

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/no-nav",
						children: "Modified",
					}),
				);
				const anchor = container.querySelector("a")!;
				anchor.addEventListener(
					"click",
					(e) => {
						return e.preventDefault();
					},
					{
						once: true,
					},
				);
				anchor.dispatchEvent(
					new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
						metaKey: true,
					}),
				);

				await new Promise((r) => {
					return setTimeout(r, 0);
				});
				expect(fetch_spy).not.toHaveBeenCalled();
			} finally {
				cleanup();
			}
		});

		it("strips vorma-specific props from rendered anchor", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.boot();

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/stripped",
						prefetch: "intent",
						prefetchDelayMs: 100,
						replace: true,
						scrollToTop: false,
						id: "my-link",
						children: "Stripped",
					}),
				);
				const anchor = container.querySelector("a")!;
				expect(anchor.getAttribute("id")).toBe("my-link");
				expect(anchor.getAttribute("prefetch")).toBeNull();
				expect(anchor.getAttribute("prefetchdelayms")).toBeNull();
				expect(anchor.getAttribute("replace")).toBeNull();
				expect(anchor.getAttribute("scrolltotop")).toBeNull();
			} finally {
				cleanup();
			}
		});
	});

	///////////////////////////////////////////////////////////////////
	/////// defineView
	///////////////////////////////////////////////////////////////////

	describe("defineView", () => {
		it("returns ViewDefinition with correct fields", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			let component_props: TestRouteProps | undefined;
			let boundary_props: { error: unknown } | undefined;
			const comp = harness.create_define_view_component<TestRouteProps>((props) => {
				component_props = props;
				return harness.h("div", { "data-define-view": "component" }, "test");
			});
			const boundary = harness.create_define_view_component<{
				error: unknown;
			}>((props) => {
				boundary_props = props;
				return harness.h("div", {}, "test");
			});
			const client_loader_fn = async () => {
				return { data: true };
			};

			const def = client.defineView({
				pattern: "/test",
				component: comp,
				errorBoundary: boundary,
				clientLoader: client_loader_fn,
			} as any);

			expect(def.pattern).toBe("/test");
			expect(def.client_loader).toBe(client_loader_fn);

			const route_props = {
				idx: 7,
				Outlet: () => {
					return harness.h("div", {}, "outlet");
				},
			};
			const boundary_error = new Error("test");
			const { container, render, cleanup } = harness.mount();
			try {
				render(def.component(route_props));
				await wait_for_dom(() => {
					expect(
						container.querySelector("[data-define-view]")?.textContent,
					).toBe("test");
				});
				expect(component_props?.idx).toBe(route_props.idx);
				expect(component_props?.Outlet).toBe(route_props.Outlet);

				render(def.error_boundary!({ error: boundary_error }));
				await wait_for_dom(() => {
					expect(container.textContent).toBe("test");
				});
				expect(boundary_props?.error).toBe(boundary_error);
			} finally {
				cleanup();
			}
		});
	});
}
