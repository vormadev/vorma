import { ui } from "../../route_factory.ts";
import {
	counter_href,
	counter_next,
	counter_previous,
	dynamic,
	h,
	klass,
	resource_count_pattern,
	text_state,
	view_counter_pattern,
	view_data_box,
} from "./support.ts";

export default ui.defineView({
	pattern: view_counter_pattern,
	component: (props: any) => {
		const data = view_data_box(props);
		const resource_next = text_state("");
		return h(
			"section",
			{ ...klass("panel"), "data-bmb-view": "counter" },
			h("h2", null, "Counter"),
			h(
				"div",
				{ "data-bmb-counter-value": true },
				dynamic(() => {
					return data().Value;
				}),
			),
			h(
				"div",
				klass("row"),
				h(
					ui.Link,
					{
						href: dynamic(() => {
							return counter_href(counter_previous(data().Value));
						}),
						"data-bmb-action": "counter-link-dec",
					},
					"Down",
				),
				h(
					ui.Link,
					{
						href: "/counter?n=-9",
						"data-bmb-action": "counter-link-low",
					},
					"Low",
				),
				h(
					ui.Link,
					{
						href: dynamic(() => {
							return counter_href(counter_next(data().Value));
						}),
						"data-bmb-action": "counter-link-inc",
					},
					"Up",
				),
				h(
					ui.Link,
					{
						href: "/counter?n=9",
						"data-bmb-action": "counter-link-high",
					},
					"High",
				),
				h(
					"button",
					{
						...klass("button"),
						"data-bmb-action": "counter-button-inc",
						type: "button",
						onClick: () => {
							void ui.navigate({
								href: counter_href(counter_next(data().Value)),
							});
						},
					},
					"Button up",
				),
				h(
					"button",
					{
						...klass("button"),
						"data-bmb-action": "count-resource-low",
						type: "button",
						onClick: () => {
							void ui.apiClient
								.query({
									method: "GET",
									pattern: resource_count_pattern,
									input: { delta: -9 },
								})
								.then((result: any) => {
									if (result.success) {
										resource_next.set(String(result.data.Next));
										return;
									}
									resource_next.set(result.error);
								});
						},
					},
					"Resource low",
				),
				h(
					"button",
					{
						...klass("button"),
						"data-bmb-action": "count-resource-high",
						type: "button",
						onClick: () => {
							void ui.apiClient
								.query({
									method: "GET",
									pattern: resource_count_pattern,
									input: { delta: 9 },
								})
								.then((result: any) => {
									if (result.success) {
										resource_next.set(String(result.data.Next));
										return;
									}
									resource_next.set(result.error);
								});
						},
					},
					"Resource high",
				),
			),
			h(
				"div",
				{ ...klass("status"), "data-bmb-count-resource-next": true },
				dynamic(() => {
					return resource_next.value();
				}),
			),
		);
	},
});
