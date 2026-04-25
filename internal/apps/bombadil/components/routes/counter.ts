import { ui } from "../../route_factory.ts";
import {
	counter_href,
	counter_next,
	counter_previous,
	dynamic,
	h,
	klass,
	loader_box,
	route_counter_pattern,
	text_state,
} from "./support.ts";

export default ui.defineRoute({
	pattern: route_counter_pattern,
	component: (props: any) => {
		const data = loader_box(props);
		const action_next = text_state("");
		return h(
			"section",
			{ ...klass("panel"), "data-bmb-route": "counter" },
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
						"data-bmb-action": "count-action-low",
						type: "button",
						onClick: () => {
							void ui.apiClient
								.submit({
									method: "GET",
									pattern: "/count",
									input: { delta: -9 },
								})
								.then((result: any) => {
									if (result.success) {
										action_next.set(
											String(result.data.Next),
										);
										return;
									}
									action_next.set(result.error);
								});
						},
					},
					"Action low",
				),
				h(
					"button",
					{
						...klass("button"),
						"data-bmb-action": "count-action-high",
						type: "button",
						onClick: () => {
							void ui.apiClient
								.submit({
									method: "GET",
									pattern: "/count",
									input: { delta: 9 },
								})
								.then((result: any) => {
									if (result.success) {
										action_next.set(
											String(result.data.Next),
										);
										return;
									}
									action_next.set(result.error);
								});
						},
					},
					"Action high",
				),
			),
			h(
				"div",
				{ ...klass("status"), "data-bmb-count-action-next": true },
				dynamic(() => {
					return action_next.value();
				}),
			),
		);
	},
});
