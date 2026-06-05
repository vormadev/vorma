import { HmrProbe } from "#hmr-probe";
import { ui } from "../../route_factory.ts";
import {
	client_build_tag,
	counter_href,
	dynamic,
	echo_resource_fail_message,
	expected_deployment_storage_key,
	expected_operation_storage_key,
	h,
	is_dev_boot,
	klass,
	nav_items,
	read_box,
	resource_count_pattern,
	resource_echo_pattern,
	resource_form_pattern,
	switch_deployment,
	text_state,
	view_counter_one_href,
	view_data_box,
	view_root_pattern,
} from "./support.ts";

let expected_storage_sync: (() => void) | undefined;
let expected_storage_sync_installed = false;

export default ui.defineView({
	pattern: view_root_pattern,
	component: (props: any) => {
		const data = view_data_box(props);
		const switch_result = text_state("");
		const form_result = text_state("");
		const hmr_state = text_state("initial");
		const expected_deployment = text_state(
			window.sessionStorage.getItem(expected_deployment_storage_key) ?? "",
		);
		const expected_operation = text_state(
			window.sessionStorage.getItem(expected_operation_storage_key) ?? "",
		);
		const route_state = ui.useRouteState();
		const is_dev = is_dev_boot();
		const route = () => {
			return read_box(route_state);
		};
		const pathname = () => {
			return new URL(route().href).pathname;
		};
		const work_state = ui.useWorkState();
		const work = () => {
			return read_box(work_state);
		};
		const skew_target = () => {
			if (data().Deployment === "from-A") {
				return "B";
			}
			return "A";
		};
		const switch_navigation_href = () => {
			const current = new URL(window.location.href);
			const target = new URL(counter_href(0), window.location.href);
			if (
				current.pathname === target.pathname &&
				current.search === target.search
			) {
				return view_counter_one_href;
			}
			return counter_href(0);
		};
		let switch_in_flight = false;
		const run_switch_action = (action: () => Promise<void>) => {
			if (switch_in_flight) {
				return;
			}
			switch_in_flight = true;
			void action()
				.catch((err) => {
					switch_result.set(String(err));
				})
				.finally(() => {
					switch_in_flight = false;
				});
		};
		const record_expected_deployment = (deployment: string, operation: string) => {
			window.sessionStorage.setItem(expected_deployment_storage_key, deployment);
			window.sessionStorage.setItem(expected_operation_storage_key, operation);
			expected_deployment.set(deployment);
			expected_operation.set(operation);
		};
		const record_expected_operation = (operation: string) => {
			window.sessionStorage.setItem(expected_operation_storage_key, operation);
			expected_operation.set(operation);
		};
		expected_storage_sync = () => {
			expected_deployment.set(
				window.sessionStorage.getItem(expected_deployment_storage_key) ?? "",
			);
			expected_operation.set(
				window.sessionStorage.getItem(expected_operation_storage_key) ?? "",
			);
		};
		if (!expected_storage_sync_installed) {
			window.addEventListener("pageshow", () => {
				expected_storage_sync?.();
			});
			expected_storage_sync_installed = true;
		}
		const complete_switch_submit = (operation: string, result: any) => {
			if (result.success) {
				record_expected_operation(`${operation}-ok`);
				switch_result.set(`${operation}:ok`);
				return;
			}
			record_expected_operation(`${operation}-error`);
			switch_result.set(`${operation}:error`);
		};
		return h(
			"main",
			{
				...klass("shell"),
				"data-bmb-shell": ui.variant,
				"data-bmb-href": dynamic(() => {
					return route().href;
				}),
				"data-bmb-pending-href": dynamic(() => {
					return work().navigation?.href ?? "";
				}),
			},
			h("div", {
				...klass("public-url-probe"),
				"data-bmb-public-url-probe": true,
			}),
			h(
				"nav",
				{ ...klass("nav"), "data-bmb-nav": true },
				nav_items.map((item) => {
					return h(
						ui.Link,
						{
							key: item.key,
							href: item.href,
							"data-bmb-link": item.key,
						},
						item.label,
					);
				}),
			),
			h(
				"section",
				{ ...klass("panel"), "data-bmb-deployment-panel": true },
				h(
					"div",
					{ "data-bmb-view-deployment": true },
					dynamic(() => {
						return data().Deployment;
					}),
				),
				h("div", { "data-bmb-client-build-tag": true }, client_build_tag),
				h(
					"div",
					{ "data-bmb-switch-result": true },
					dynamic(() => {
						return switch_result.value();
					}),
				),
				h(
					"div",
					{ "data-bmb-expected-deployment": true },
					dynamic(() => {
						return expected_deployment.value();
					}),
				),
				h(
					"div",
					{ "data-bmb-expected-operation": true },
					dynamic(() => {
						return expected_operation.value();
					}),
				),
				...(is_dev
					? []
					: [
							h(
								"button",
								{
									...klass("button"),
									"data-bmb-action": "switch-deployment",
									type: "button",
									onClick: () => {
										run_switch_action(async () => {
											const deployment =
												await switch_deployment(skew_target());
											record_expected_deployment(
												deployment,
												"switch",
											);
											switch_result.set(deployment);
										});
									},
								},
								"Switch deployment",
							),
							h(
								"button",
								{
									...klass("button"),
									"data-bmb-action": "switch-revalidate",
									type: "button",
									onClick: () => {
										run_switch_action(async () => {
											const deployment =
												await switch_deployment(skew_target());
											record_expected_deployment(
												deployment,
												"revalidate-pending",
											);
											switch_result.set(deployment);
											const result: any = await ui.revalidate();
											if (result.ok) {
												record_expected_operation(
													"revalidate-ok",
												);
												switch_result.set("revalidated:ok");
												return;
											}
											record_expected_operation(
												`revalidate-${result.reason}`,
											);
											switch_result.set(
												`revalidated:${result.reason}`,
											);
										});
									},
								},
								"Switch + revalidate",
							),
							h(
								"button",
								{
									...klass("button"),
									"data-bmb-action": "switch-navigate",
									type: "button",
									onClick: () => {
										run_switch_action(async () => {
											const deployment =
												await switch_deployment(skew_target());
											record_expected_deployment(
												deployment,
												"navigate",
											);
											switch_result.set(deployment);
											await ui.navigate({
												href: switch_navigation_href(),
											});
										});
									},
								},
								"Switch + navigate",
							),
							h(
								"button",
								{
									...klass("button"),
									"data-bmb-action": "switch-query",
									type: "button",
									onClick: () => {
										run_switch_action(async () => {
											const deployment =
												await switch_deployment(skew_target());
											record_expected_deployment(
												deployment,
												"query-pending",
											);
											switch_result.set(deployment);
											const result: any = await ui.apiClient.query({
												method: "GET",
												pattern: resource_count_pattern,
												input: { delta: 9 },
											});
											complete_switch_submit("query", result);
										});
									},
								},
								"Switch + query",
							),
							h(
								"button",
								{
									...klass("button"),
									"data-bmb-action": "switch-mutation",
									type: "button",
									onClick: () => {
										run_switch_action(async () => {
											const deployment =
												await switch_deployment(skew_target());
											record_expected_deployment(
												deployment,
												"mutation-pending",
											);
											switch_result.set(deployment);
											const result: any = await ui.apiClient.mutate(
												{
													method: "POST",
													pattern: resource_echo_pattern,
													input: {
														Message: "unsafe-ok",
													},
												},
											);
											complete_switch_submit("mutation", result);
										});
									},
								},
								"Switch + mutation",
							),
							h(
								"button",
								{
									...klass("button"),
									"data-bmb-action": "switch-mutation-fail",
									type: "button",
									onClick: () => {
										run_switch_action(async () => {
											const deployment =
												await switch_deployment(skew_target());
											record_expected_deployment(
												deployment,
												"mutation-error-pending",
											);
											switch_result.set(deployment);
											const result: any = await ui.apiClient.mutate(
												{
													method: "POST",
													pattern: resource_echo_pattern,
													input: {
														Message:
															echo_resource_fail_message,
													},
												},
											);
											complete_switch_submit(
												"mutation-error",
												result,
											);
										});
									},
								},
								"Switch + failed mutation",
							),
						]),
			),
			h(
				"section",
				{ ...klass("panel"), "data-bmb-view-summary": true },
				h(
					"div",
					{ "data-bmb-current-href": true },
					dynamic(() => {
						return route().href;
					}),
				),
				h(
					"div",
					{ "data-bmb-work-submissions": true },
					dynamic(() => {
						return work().apiRequests.length;
					}),
				),
				h(
					"div",
					klass("row"),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "form-submit",
							type: "button",
							onClick: () => {
								const form_data = new FormData();
								form_data.set("title", "hello");
								form_data.append("tag", "rust");
								form_data.append("tag", "vorma");
								form_result.set("form-pending");
								void ui.apiClient
									.mutate({
										method: "POST",
										pattern: resource_form_pattern,
										input: form_data,
									})
									.then((result: any) => {
										if (result.success) {
											form_result.set(
												result.data.Accepted
													? "accepted"
													: "rejected",
											);
											return;
										}
										form_result.set(result.error);
									});
							},
						},
						"Submit FormData",
					),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "prefetch-slow",
							type: "button",
							onClick: () => {
								ui.prefetch({ href: "/slow?delay_ms=220" });
							},
						},
						"Prefetch slow",
					),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "prefetch-client-alpha",
							type: "button",
							onClick: () => {
								ui.prefetch({ href: "/client/alpha" });
							},
						},
						"Prefetch client",
					),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "cancel-prefetch-slow",
							type: "button",
							onClick: () => {
								ui.cancelPrefetch({
									href: "/slow?delay_ms=220",
								});
							},
						},
						"Cancel prefetch",
					),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "cancel-prefetch-client-alpha",
							type: "button",
							onClick: () => {
								ui.cancelPrefetch({
									href: "/client/alpha",
								});
							},
						},
						"Cancel client",
					),
				),
				h(
					"div",
					{ "data-bmb-form-output": true },
					dynamic(() => {
						return form_result.value();
					}),
				),
			),
			h(HmrProbe, {
				state_box: hmr_state,
			}),
			dynamic(() => {
				if (pathname() !== view_root_pattern) {
					return null;
				}
				return h(
					"section",
					{ ...klass("panel"), "data-bmb-view": "home" },
					h("h1", null, "Vorma Bombadil Fixture"),
					h("p", null, `${ui.variant} variant`),
				);
			}),
			props.Outlet(),
		);
	},
});
