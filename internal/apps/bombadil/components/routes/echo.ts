import { ui } from "../../route_factory.ts";
import {
	action_echo_pattern,
	dynamic,
	h,
	klass,
	loader_box,
	route_echo_pattern,
	text_state,
} from "./support.ts";

export default ui.defineRoute({
	pattern: route_echo_pattern,
	component: (props: any) => {
		const data = loader_box(props);
		const message = text_state("hello");
		const output = text_state(data().Message);
		return h(
			"section",
			{ ...klass("panel"), "data-bmb-route": "echo" },
			h("h2", null, "Echo"),
			h(
				"div",
				klass("row"),
				h("input", {
					...klass("input"),
					"data-bmb-input": "echo",
					value: dynamic(() => {
						return message.value();
					}),
					[ui.input_event]: (event: any) => {
						message.set(event.currentTarget.value);
					},
				}),
				h(
					"button",
					{
						...klass("button"),
						"data-bmb-action": "echo-submit",
						type: "button",
						onClick: () => {
							void ui.apiClient
								.submit({
									method: "POST",
									pattern: action_echo_pattern,
									input: { Message: message.value() },
									revalidate: true,
								})
								.then((result: any) => {
									if (result.success) {
										output.set(result.data.Message);
										return;
									}
									output.set(result.error);
								});
						},
					},
					"Send",
				),
				h(
					"button",
					{
						...klass("button"),
						"data-bmb-action": "revalidate",
						type: "button",
						onClick: () => {
							void ui.revalidate();
						},
					},
					"Revalidate",
				),
			),
			h(
				"div",
				{ ...klass("status"), "data-bmb-echo-output": true },
				dynamic(() => {
					return output.value();
				}),
			),
		);
	},
});
