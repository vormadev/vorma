import { ui } from "../../route_factory.ts";
import {
	action_count_pattern,
	action_echo_pattern,
	client_build_tag,
	dynamic,
	echo_action_fail_message,
	expected_deployment_storage_key,
	expected_operation_storage_key,
	h,
	klass,
	loader_box,
	nav_items,
	read_box,
	route_counter_one_href,
	route_root_pattern,
	switch_deployment,
	text_state,
} from "./support.ts";

export default ui.defineRoute({
	pattern: route_root_pattern,
	component: (props: any) => {
		const data = loader_box(props);
		const switch_result = text_state("");
		const expected_deployment = text_state(
			window.sessionStorage.getItem(expected_deployment_storage_key) ??
				"",
		);
		const expected_operation = text_state(
			window.sessionStorage.getItem(expected_operation_storage_key) ?? "",
		);
		const route_state = ui.useRouteState();
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
		const record_expected_deployment = (
			deployment: string,
			operation: string,
		) => {
			window.sessionStorage.setItem(
				expected_deployment_storage_key,
				deployment,
			);
			window.sessionStorage.setItem(
				expected_operation_storage_key,
				operation,
			);
			expected_deployment.set(deployment);
			expected_operation.set(operation);
		};
		const record_expected_operation = (operation: string) => {
			window.sessionStorage.setItem(
				expected_operation_storage_key,
				operation,
			);
			expected_operation.set(operation);
		};
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
			h(
				"nav",
				{ ...klass("nav"), "data-bmb-nav": true },
				nav_items.map((item) => {
					return h(
						ui.Link,
						{ href: item.href, "data-bmb-link": item.key },
						item.label,
					);
				}),
			),
			h(
				"section",
				{ ...klass("panel"), "data-bmb-deployment-panel": true },
				h(
					"div",
					{ "data-bmb-route-deployment": true },
					dynamic(() => {
						return data().Deployment;
					}),
				),
				h(
					"div",
					{ "data-bmb-client-build-tag": true },
					client_build_tag,
				),
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
				h(
					"button",
					{
						...klass("button"),
						"data-bmb-action": "switch-deployment",
						type: "button",
						onClick: () => {
							void switch_deployment(skew_target())
								.then((deployment) => {
									record_expected_deployment(
										deployment,
										"switch",
									);
									switch_result.set(deployment);
								})
								.catch((err) => {
									switch_result.set(String(err));
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
							void switch_deployment(skew_target())
								.then((deployment) => {
									record_expected_deployment(
										deployment,
										"revalidate-pending",
									);
									switch_result.set(deployment);
									return ui.revalidate();
								})
								.then((result: any) => {
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
								})
								.catch((err) => {
									switch_result.set(String(err));
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
							void switch_deployment(skew_target())
								.then((deployment) => {
									record_expected_deployment(
										deployment,
										"navigate",
									);
									switch_result.set(deployment);
									return ui.navigate({
										href: route_counter_one_href,
									});
								})
								.catch((err) => {
									switch_result.set(String(err));
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
							void switch_deployment(skew_target())
								.then((deployment) => {
									record_expected_deployment(
										deployment,
										"query-pending",
									);
									switch_result.set(deployment);
									return ui.apiClient.submit({
										method: "GET",
										pattern: action_count_pattern,
										input: { delta: 9 },
									});
								})
								.then((result: any) => {
									complete_switch_submit("query", result);
								})
								.catch((err) => {
									switch_result.set(String(err));
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
							void switch_deployment(skew_target())
								.then((deployment) => {
									record_expected_deployment(
										deployment,
										"mutation-pending",
									);
									switch_result.set(deployment);
									return ui.apiClient.submit({
										method: "POST",
										pattern: action_echo_pattern,
										input: { Message: "unsafe-ok" },
									});
								})
								.then((result: any) => {
									complete_switch_submit("mutation", result);
								})
								.catch((err) => {
									switch_result.set(String(err));
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
							void switch_deployment(skew_target())
								.then((deployment) => {
									record_expected_deployment(
										deployment,
										"mutation-error-pending",
									);
									switch_result.set(deployment);
									return ui.apiClient.submit({
										method: "POST",
										pattern: action_echo_pattern,
										input: {
											Message: echo_action_fail_message,
										},
									});
								})
								.then((result: any) => {
									complete_switch_submit(
										"mutation-error",
										result,
									);
								})
								.catch((err) => {
									switch_result.set(String(err));
								});
						},
					},
					"Switch + failed mutation",
				),
			),
			h(
				"section",
				{ ...klass("panel"), "data-bmb-route-summary": true },
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
						return work().submissions.length;
					}),
				),
				h(
					"div",
					klass("row"),
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
			),
			dynamic(() => {
				if (pathname() !== route_root_pattern) {
					return null;
				}
				return h(
					"section",
					{ ...klass("panel"), "data-bmb-route": "home" },
					h("h1", null, "Vorma Bombadil Fixture"),
					h("p", null, `${ui.variant} variant`),
				);
			}),
			props.Outlet(),
		);
	},
});
