// @vitest-environment jsdom

import { createElement, createRoot, on, type RemixNode } from "remix/ui";
import { describe, expect, it, vi } from "vitest";
import {
	define_adapter_tests,
	route_response,
	seed_payload,
	TEST_CONFIG,
	wait_for_dom,
	type AdapterTestHarness,
} from "../../tests/adapter_test_core.ts";
import {
	createVormaClient,
	type RemixComponent,
	type RemixViewComponent,
} from "./remix.tsx";

let stable_ref: { current: unknown } | undefined;

function create_test_element(
	type: unknown,
	props?: Record<string, unknown> | null,
	...children: unknown[]
): RemixNode {
	const next_props = { ...props };
	const child_nodes =
		children.length > 0
			? children
			: props?.children !== undefined
				? [props.children]
				: [];
	delete next_props.children;

	if (typeof type !== "function") {
		return createElement(type as string, next_props, ...child_nodes);
	}

	const component = type as (props: Record<string, unknown>) => RemixNode;
	const wrapper: RemixComponent<Record<string, unknown>> = () => {
		return (render_props) => {
			return component(render_props);
		};
	};
	return createElement(wrapper, next_props, ...child_nodes);
}

define_adapter_tests({
	create_client: (config, options) => {
		return createVormaClient(config as any, options) as any;
	},

	h: create_test_element as AdapterTestHarness["h"],
	create_define_view_component: ((render: (props: object) => unknown) => {
		const component: RemixViewComponent<any, any> = () => {
			return (props: object) => {
				return render(props) as RemixNode;
			};
		};
		return component;
	}) as AdapterTestHarness["create_define_view_component"],
	create_view: (({ client, pattern, render, clientLoader }) => {
		const remix_client = client as unknown as ReturnType<
			typeof createVormaClient<any>
		>;
		return remix_client.defineView({
			pattern: pattern as any,
			component: (_handle, v) => {
				return (props) => {
					return render({
						props: props as any,
						v: v as any,
					}) as RemixNode;
				};
			},
			clientLoader: clientLoader as any,
		}) as any;
	}) as AdapterTestHarness["create_view"],
	dynamic: ((read_value: () => unknown) => {
		return read_value();
	}) as AdapterTestHarness["dynamic"],
	unwrap: ((v: unknown) => {
		return v;
	}) as AdapterTestHarness["unwrap"],

	mount: () => {
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		return {
			container,
			render: (element: unknown) => {
				root.render(element as RemixNode);
				root.flush();
			},
			cleanup: () => {
				root.dispose();
				stable_ref = undefined;
				container.remove();
			},
		};
	},

	use_ref: <T>(initial: T) => {
		if (!stable_ref) {
			stable_ref = { current: initial };
		}
		return stable_ref as { current: T };
	},

	create_identity_parent: () => {
		let mount_count = 0;
		let unmount_count = 0;

		const identity_parent: RemixComponent<any> = (handle) => {
			mount_count++;
			let draft = "initial";
			handle.signal.addEventListener(
				"abort",
				() => {
					unmount_count++;
				},
				{ once: true },
			);
			return (props) => {
				return createElement(
					"section",
					{},
					createElement("div", { "data-draft": "true" }, draft),
					createElement("button", {
						"data-set": "true",
						mix: on<HTMLButtonElement, "click">("click", () => {
							draft = "modified";
							void handle.update();
						}),
					}),
					create_test_element(props.Outlet, {}),
				);
			};
		};

		const component = (props: any) => {
			return createElement(identity_parent, props);
		};

		return {
			component,
			get_mount_count: () => {
				return mount_count;
			},
			get_unmount_count: () => {
				return unmount_count;
			},
		};
	},
});

describe("Remix adapter update subscriptions", () => {
	it("does not rerender the route tree when link intent prefetch starts", async () => {
		const root_pattern = "/";
		const prefetch_target_pattern = "/prefetch-target";
		const prefetch_page_module_url = "/remix-prefetch-page.js";
		let client: ReturnType<typeof createVormaClient<any>>;
		let route_render_count = 0;

		vi.doMock(prefetch_page_module_url, () => {
			return {
				default: {
					pattern: root_pattern,
					component: () => {
						route_render_count++;
						return createElement(
							"main",
							{},
							createElement(
								client.Link as any,
								{
									pattern: prefetch_target_pattern,
									prefetch: "intent",
									prefetchDelayMs: 0,
								},
								"Prefetch",
							),
						);
					},
				},
			};
		});

		seed_payload({
			MatchedPatterns: [root_pattern],
			LoadersData: [{}],
			ImportURLs: [prefetch_page_module_url],
		});
		client = createVormaClient(TEST_CONFIG);
		await client.boot();

		let resolve_response!: (response: Response) => void;
		const response_promise = new Promise<Response>((resolve) => {
			resolve_response = resolve;
		});
		const fetch_mock = vi
			.spyOn(globalThis, "fetch")
			.mockReturnValueOnce(response_promise);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			root.render(createElement(client.RootOutlet, {}));
			root.flush();
			expect(route_render_count).toBe(1);

			const anchor = container.querySelector("a")!;
			anchor.dispatchEvent(
				new Event("pointerenter", {
					bubbles: true,
					cancelable: true,
				}),
			);

			await wait_for_dom(() => {
				expect(fetch_mock).toHaveBeenCalledTimes(1);
			});
			expect(route_render_count).toBe(1);

			await new Promise((resolve) => {
				return setTimeout(resolve, 0);
			});
			expect(route_render_count).toBe(1);
		} finally {
			resolve_response(
				route_response({
					MatchedPatterns: [prefetch_target_pattern],
					LoadersData: [{}],
				}),
			);
			root.dispose();
			container.remove();
		}
	});

	it("rerenders explicit work-state subscribers when prefetch work changes", async () => {
		const root_pattern = "/";
		const prefetch_target_pattern = "/prefetch-subscribed-target";
		const prefetch_page_module_url = "/remix-prefetch-subscribed-page.js";
		let client: ReturnType<typeof createVormaClient<any>>;
		let route_render_count = 0;
		let subscriber_render_count = 0;

		vi.doMock(prefetch_page_module_url, () => {
			return {
				default: {
					pattern: root_pattern,
					component: () => {
						route_render_count++;
						const subscriber_view = client.defineView({
							pattern: root_pattern as any,
							component: (_handle, v) => {
								return () => {
									subscriber_render_count++;
									const href = v.workState((work) => {
										return work.prefetch?.href ?? "none";
									});
									v.routeSync({ pattern: root_pattern });
									return createElement(
										"output",
										{ "data-prefetch-state": "" },
										href,
									);
								};
							},
						});
						return createElement(
							"main",
							{},
							create_test_element(subscriber_view.component, {
								idx: 0,
								Outlet: () => {
									return null;
								},
							}),
							createElement(
								client.Link as any,
								{
									pattern: prefetch_target_pattern,
									prefetch: "intent",
									prefetchDelayMs: 0,
								},
								"Prefetch",
							),
						);
					},
				},
			};
		});

		seed_payload({
			MatchedPatterns: [root_pattern],
			LoadersData: [{}],
			ImportURLs: [prefetch_page_module_url],
		});
		client = createVormaClient(TEST_CONFIG);
		await client.boot();

		let resolve_response!: (response: Response) => void;
		const response_promise = new Promise<Response>((resolve) => {
			resolve_response = resolve;
		});
		vi.spyOn(globalThis, "fetch").mockReturnValueOnce(response_promise);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			root.render(createElement(client.RootOutlet, {}));
			root.flush();
			expect(route_render_count).toBe(1);
			expect(subscriber_render_count).toBe(1);

			const anchor = container.querySelector("a")!;
			anchor.dispatchEvent(
				new Event("pointerenter", {
					bubbles: true,
					cancelable: true,
				}),
			);

			await wait_for_dom(() => {
				const output = container.querySelector(
					"[data-prefetch-state]",
				)!;
				expect(output.textContent).toContain(prefetch_target_pattern);
			});
			expect(route_render_count).toBe(1);
			expect(subscriber_render_count).toBeGreaterThan(1);
		} finally {
			resolve_response(
				route_response({
					MatchedPatterns: [prefetch_target_pattern],
					LoadersData: [{}],
				}),
			);
			root.dispose();
			container.remove();
		}
	});
});
