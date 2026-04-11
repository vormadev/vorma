// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	BUILD_ID_HEADER,
	DATA_SCRIPT_ID,
	type InitOptions,
	type RevalidationResult,
} from "vorma/__internal";

/////// Harness type

type TestRouteProps = {
	idx: number;
	Outlet: (local?: Record<string, unknown>) => unknown;
};

type TestClientLoaderProps = {
	params: Record<string, string>;
	splatValues: string[];
	serverDataPromise: Promise<{
		matchedPatterns: string[];
		rootData: unknown;
		loaderData: unknown;
		clientBuildID: string;
	}>;
	signal: AbortSignal;
};

type TestVormaClient = {
	init: (options: Partial<InitOptions>) => Promise<void>;

	navigate: (
		props: { pattern: string } & Record<string, unknown>,
	) => Promise<{ didNavigate: boolean }>;

	revalidate: () => Promise<RevalidationResult>;

	RootOutlet: (props?: { idx?: number } & Record<string, unknown>) => unknown;

	Link: (props: { pattern: string } & Record<string, unknown>) => unknown;

	useLoaderData: (
		props: { idx: number } & Record<string, unknown>,
	) => unknown;

	usePatternLoaderData: (pattern: string) => unknown;

	useRouterData: () => unknown;

	useClientLoaderData: (
		props: { idx: number } & Record<string, unknown>,
	) => unknown;

	usePatternClientLoaderData: (pattern: string) => unknown;

	defineRoute: (input: {
		pattern: string;
		component: (props: TestRouteProps) => unknown;
		errorBoundary?: (props: { error: unknown }) => unknown;
		clientLoader?: (props: TestClientLoaderProps) => Promise<unknown>;
		runClientLoaderOnHMR?: boolean;
	}) => {
		pattern: string;
		component: (props: TestRouteProps) => unknown;
		error_boundary?: (props: { error: unknown }) => unknown;
		client_loader?: (props: TestClientLoaderProps) => Promise<unknown>;
	};
};

type TestCreateClientOptions = {
	linkDefaultProps?: Record<string, unknown>;
};

export type AdapterTestHarness = {
	create_client: (
		config: { actionsMountRoot: string },
		options?: TestCreateClientOptions,
	) => TestVormaClient;

	h: (
		type: unknown,
		props?: Record<string, unknown> | null,
		...children: unknown[]
	) => unknown;

	mount: () => {
		container: HTMLElement;
		render: (element: unknown) => void;
		cleanup: () => void;
	};

	use_ref: <T>(initial: T) => { current: T };

	create_identity_parent: () => {
		component: (props: TestRouteProps) => unknown;
		get_mount_count: () => number;
		get_unmount_count: () => number;
	};

	unwrap: <T>(value: T | (() => T)) => T;

	skip_rerender_test?: boolean;
};

/////// Helpers

const TEST_CONFIG = { actionsMountRoot: "/api/" } as any;

function seed_payload(overrides: Record<string, unknown> = {}) {
	const data = {
		ClientBuildID: "build-1",
		DeploymentID: "",
		MatchedPatterns: [],
		LoadersData: [],
		ImportURLs: [],
		Params: {},
		SplatValues: [],
		Title: undefined,
		MetaHeadEls: [],
		RestHeadEls: [],
		CSSBundles: [],
		Deps: [],
		...overrides,
	};
	const script = document.createElement("script");
	script.id = DATA_SCRIPT_ID;
	script.type = "application/json";
	script.textContent = JSON.stringify(data);
	document.head.appendChild(script);
}

function route_response(
	overrides: Record<string, unknown> = {},
	build_id = "build-1",
): Response {
	const data = {
		MatchedPatterns: [],
		LoadersData: [],
		ImportURLs: [],
		Params: {},
		SplatValues: [],
		Title: undefined,
		MetaHeadEls: [],
		RestHeadEls: [],
		CSSBundles: [],
		Deps: [],
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

async function wait_for_dom(assertion: () => void, max = 50) {
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
				MatchedPatterns: ["/"],
				LoadersData: [{ root: true }],
				ImportURLs: ["/root.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

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
				MatchedPatterns: ["/"],
				LoadersData: [{}],
				ImportURLs: ["/props-check.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

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
				MatchedPatterns: ["/", "/child"],
				LoadersData: [{}, {}],
				ImportURLs: ["/no-comp.js", "/child.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

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

			await client.init({});

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
				MatchedPatterns: ["/"],
				LoadersData: [{}],
				ImportURLs: ["/default-idx.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet));
				expect(container.textContent).toBe("idx:0");
			} finally {
				cleanup();
			}
		});

		it("renders error boundary from route module", async () => {
			vi.doMock("/error-route.js", () => {
				return {
					default: {
						pattern: "/error",
						component: () => {
							return harness.h("div", {}, "should-not-render");
						},
						error_boundary: (props: { error: unknown }) => {
							return harness.h(
								"div",
								{},
								`handled:${props.error as any}`,
							);
						},
					},
				};
			});

			seed_payload({
				MatchedPatterns: ["/error"],
				LoadersData: [{}],
				ImportURLs: ["/error-route.js"],
				OutermostServerErrIdx: 0,
				OutermostServerErr: "boom",
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toBe("handled:boom");
				expect(container.textContent).not.toContain(
					"should-not-render",
				);
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
				MatchedPatterns: ["/no-boundary"],
				LoadersData: [{}],
				ImportURLs: ["/no-boundary.js"],
				OutermostServerErrIdx: 0,
				OutermostServerErr: "default-boom",
			});
			const default_boundary = (props: { error: unknown }) => {
				return harness.h("div", {}, `default:${props.error as any}`);
			};
			const client = harness.create_client(TEST_CONFIG);

			await client.init({ defaultErrorBoundary: default_boundary });

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
				MatchedPatterns: ["/parent", "/parent/child"],
				LoadersData: [{}, {}],
				ImportURLs: ["/parent-live.js", "/child-err.js"],
				OutermostServerErrIdx: 1,
				OutermostServerErr: "child-boom",
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(
					container.querySelector("[data-parent]")?.textContent,
				).toBe("parent-node");
				expect(container.textContent).toContain(
					"child-handled:child-boom",
				);
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

			await client.init({});

			const scroll_spy = vi.spyOn(window, "scrollTo");
			// rAF used by some adapters (Solid) — mock to fire synchronously
			vi.spyOn(window, "requestAnimationFrame").mockImplementation(
				(cb) => {
					cb(0);
					return 0;
				},
			);

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));

				vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
					route_response({
						MatchedPatterns: ["/scrolled"],
						LoadersData: [{}],
						ImportURLs: ["/scroll-target.js"],
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
				MatchedPatterns: ["/parent", "/parent/child"],
				LoadersData: [{}, {}],
				ImportURLs: ["/id-parent.js", "/id-child-a.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(container.textContent).toContain("child-a");
				expect(parent.get_mount_count()).toBe(1);

				// Modify parent state
				const btn = container.querySelector(
					"[data-set]",
				) as HTMLElement;
				btn.click();
				await wait_for_dom(() => {
					expect(
						container.querySelector("[data-draft]")?.textContent,
					).toBe("modified");
				});

				// Navigate to change child
				vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
					route_response({
						MatchedPatterns: ["/parent", "/parent/child"],
						LoadersData: [{}, {}],
						ImportURLs: ["/id-parent.js", "/id-child-b.js"],
					}),
				);
				await (client.navigate as any)({
					pattern: "/parent/child",
				});

				await wait_for_dom(() => {
					expect(container.textContent).toContain("child-b");
				});

				expect(
					container.querySelector("[data-draft]")?.textContent,
				).toBe("modified");
				expect(parent.get_mount_count()).toBe(1);
				expect(parent.get_unmount_count()).toBe(0);
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
				MatchedPatterns: ["/"],
				LoadersData: [{ v: "initial" }],
				ImportURLs: ["/stable.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { container, render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(mount_count).toBe(1);

				vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
					route_response({
						MatchedPatterns: ["/"],
						LoadersData: [{ v: "updated" }],
						ImportURLs: ["/stable.js"],
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
		it("useLoaderData returns data at props.idx", async () => {
			let captured: unknown;

			vi.doMock("/hook-loader.js", () => {
				return {
					default: {
						pattern: "/",
						component: (props: any) => {
							captured = client.useLoaderData(props);
							return harness.h("div", {}, "hook-test");
						},
					},
				};
			});

			seed_payload({
				MatchedPatterns: ["/"],
				LoadersData: [{ name: "Ada" }],
				ImportURLs: ["/hook-loader.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toEqual({ name: "Ada" });
			} finally {
				cleanup();
			}
		});

		it("usePatternLoaderData returns data for matching pattern", async () => {
			let captured: unknown;

			vi.doMock("/hook-pattern.js", () => {
				return {
					default: {
						pattern: "/",
						component: () => {
							captured = client.usePatternLoaderData("/");
							return harness.h("div", {}, "pattern-test");
						},
					},
				};
			});

			seed_payload({
				MatchedPatterns: ["/"],
				LoadersData: [{ session: "abc" }],
				ImportURLs: ["/hook-pattern.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toEqual({ session: "abc" });
			} finally {
				cleanup();
			}
		});

		it("usePatternLoaderData returns undefined for non-matching pattern", async () => {
			let captured: unknown = "sentinel";

			vi.doMock("/hook-no-match.js", () => {
				return {
					default: {
						pattern: "/",
						component: () => {
							captured = client.usePatternLoaderData(
								"/does-not-exist" as any,
							);
							return harness.h("div", {}, "no-match-test");
						},
					},
				};
			});

			seed_payload({
				MatchedPatterns: ["/"],
				LoadersData: [{}],
				ImportURLs: ["/hook-no-match.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toBeUndefined();
			} finally {
				cleanup();
			}
		});

		it("useRouterData returns unscoped router data", async () => {
			let captured: unknown;

			vi.doMock("/hook-router.js", () => {
				return {
					default: {
						pattern: "/users/:id",
						component: () => {
							captured = client.useRouterData();
							return harness.h("div", {}, "router-test");
						},
					},
				};
			});

			seed_payload({
				MatchedPatterns: ["/users/:id"],
				LoadersData: [{ user: "Ada" }],
				ImportURLs: ["/hook-router.js"],
				Params: { id: "42" },
				SplatValues: [],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				const data = harness.unwrap(captured) as any;
				expect(data.params).toEqual({ id: "42" });
				expect(data.matchedPatterns).toEqual(["/users/:id"]);
				expect(data.clientBuildID).toBe("build-1");
				expect(data).toHaveProperty("historyState", undefined);
			} finally {
				cleanup();
			}
		});

		it("useClientLoaderData returns client_data at props.idx", async () => {
			let captured: unknown = "sentinel";

			vi.doMock("/hook-cl.js", () => {
				return {
					default: {
						pattern: "/cl-test",
						component: (props: any) => {
							captured = client.useClientLoaderData(props);
							return harness.h("div", {}, "cl-test");
						},
						client_loader: async ({ serverDataPromise }: any) => {
							await serverDataPromise;
							return { enhanced: true };
						},
					},
				};
			});

			seed_payload({
				MatchedPatterns: ["/cl-test"],
				LoadersData: [{ raw: "data" }],
				ImportURLs: ["/hook-cl.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toEqual({ enhanced: true });
			} finally {
				cleanup();
			}
		});

		it("usePatternClientLoaderData returns undefined when no match", async () => {
			let captured: unknown = "sentinel";

			vi.doMock("/hook-pcl.js", () => {
				return {
					default: {
						pattern: "/",
						component: () => {
							captured = client.usePatternClientLoaderData(
								"/nonexistent" as any,
							);
							return harness.h("div", {}, "pcl-test");
						},
					},
				};
			});

			seed_payload({
				MatchedPatterns: ["/"],
				LoadersData: [{}],
				ImportURLs: ["/hook-pcl.js"],
			});
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			const { render, cleanup } = harness.mount();
			try {
				render(harness.h(client.RootOutlet, { idx: 0 }));
				expect(harness.unwrap(captured)).toBeUndefined();
			} finally {
				cleanup();
			}
		});

		const rerender_it = harness.skip_rerender_test ? it.skip : it;
		rerender_it(
			"hooks re-render when state changes via navigation",
			async () => {
				const rendered_values: unknown[] = [];

				vi.doMock("/hook-rerender.js", () => {
					return {
						default: {
							pattern: "/",
							component: () => {
								const data = client.useLoaderData({
									idx: 0,
								} as any);
								rendered_values.push(harness.unwrap(data));
								return harness.h(
									"div",
									{ "data-value": "true" },
									JSON.stringify(harness.unwrap(data)),
								);
							},
						},
					};
				});

				seed_payload({
					MatchedPatterns: ["/"],
					LoadersData: [{ v: "initial" }],
					ImportURLs: ["/hook-rerender.js"],
				});
				const client = harness.create_client(TEST_CONFIG);

				await client.init({});

				const { container, render, cleanup } = harness.mount();
				try {
					render(harness.h(client.RootOutlet, { idx: 0 }));
					expect(rendered_values[0]).toEqual({ v: "initial" });

					vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
						route_response({
							MatchedPatterns: ["/"],
							LoadersData: [{ v: "updated" }],
							ImportURLs: ["/hook-rerender.js"],
						}),
					);
					await client.revalidate();

					await wait_for_dom(() => {
						expect(
							container.querySelector("[data-value]")
								?.textContent,
						).toBe('{"v":"updated"}');
					});
				} finally {
					cleanup();
				}
			},
		);
	});

	///////////////////////////////////////////////////////////////////
	/////// Link
	///////////////////////////////////////////////////////////////////

	describe("Link", () => {
		it("renders anchor with href built from pattern and params", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

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

			await client.init({});

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/products/:id",
						params: { id: "7" },
						search: "?tab=reviews",
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

			await client.init({});

			const { container, render, cleanup } = harness.mount();
			try {
				render(
					harness.h(client.Link as any, {
						pattern: "/a",
						children: "A",
					}),
				);
				expect(container.querySelector("a")!.className).toBe(
					"default-class",
				);

				render(
					harness.h(client.Link as any, {
						pattern: "/b",
						className: "custom-class",
						children: "B",
					}),
				);
				expect(container.querySelector("a")!.className).toBe(
					"custom-class",
				);
			} finally {
				cleanup();
			}
		});

		it("navigates on eligible internal click", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			await client.init({});

			vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
				route_response({ MatchedPatterns: ["/clicked"] }),
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

			await client.init({});

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

			await client.init({});

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
	/////// defineRoute
	///////////////////////////////////////////////////////////////////

	describe("defineRoute", () => {
		it("returns RouteDefinition with correct fields", async () => {
			seed_payload();
			const client = harness.create_client(TEST_CONFIG);

			const comp = () => {
				return harness.h("div", {}, "test");
			};
			const boundary = (props: { error: unknown }) => {
				return harness.h("div", {}, String(props.error));
			};
			const loader = async () => {
				return { data: true };
			};

			const def = client.defineRoute({
				pattern: "/test",
				component: comp,
				errorBoundary: boundary,
				clientLoader: loader,
			} as any);

			expect(def.pattern).toBe("/test");
			expect(def.component).toBe(comp);
			expect(def.error_boundary).toBe(boundary);
			expect(def.client_loader).toBe(loader);
		});
	});
}
