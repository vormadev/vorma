import { ui } from "../../route_factory.ts";
import {
	dynamic,
	h,
	klass,
	loader_box,
	route_slow_pattern,
} from "./support.ts";

export default ui.defineView({
	pattern: route_slow_pattern,
	component: (props: any) => {
		const data = loader_box(props);
		return h(
			"section",
			{ ...klass("panel"), "data-bmb-route": "slow" },
			h("h2", null, "Slow"),
			h(
				"div",
				{ "data-bmb-slow-delay": true },
				dynamic(() => {
					return data().DelayMS;
				}),
			),
			h(
				"div",
				{ "data-bmb-slow-stamp": true },
				dynamic(() => {
					return data().Stamp;
				}),
			),
			h(
				"div",
				klass("row"),
				h(
					ui.Link,
					{
						href: "/slow?delay_ms=20",
						"data-bmb-action": "slow-fast",
					},
					"Fast",
				),
				h(
					ui.Link,
					{
						href: "/slow?delay_ms=-20",
						"data-bmb-action": "slow-negative",
					},
					"Negative",
				),
				h(
					ui.Link,
					{
						href: "/slow?delay_ms=220",
						"data-bmb-action": "slow-slower",
					},
					"Slower",
				),
				h(
					ui.Link,
					{
						href: "/slow?delay_ms=999",
						"data-bmb-action": "slow-too-slow",
					},
					"Too slow",
				),
			),
		);
	},
});
