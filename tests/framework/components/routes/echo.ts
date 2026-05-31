import { ui } from "../../route_factory.ts";
import {
	action_echo_pattern,
	dynamic,
	echo_action_fail_message,
	h,
	klass,
	loader_box,
	route_echo_pattern,
	text_state,
} from "./support.ts";

export default ui.defineView({
	pattern: route_echo_pattern,
	component: (props: any) => {
		const data = loader_box(props);
		const message = text_state("hello");
		const output = text_state(data().Message);
		const operation = text_state("");
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
							operation.set("submit-pending");
							void ui.apiClient
								.mutate({
									method: "POST",
									pattern: action_echo_pattern,
									input: { Message: message.value() },
									revalidate: true,
								})
								.then((result: any) => {
									if (result.success) {
										operation.set("submit-ok");
										output.set(result.data.Message);
										return;
									}
									operation.set("submit-error");
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
						"data-bmb-action": "echo-fail",
						type: "button",
						onClick: () => {
							operation.set("fail-pending");
							void ui.apiClient
								.mutate({
									method: "POST",
									pattern: action_echo_pattern,
									input: {
										Message: echo_action_fail_message,
									},
									revalidate: true,
								})
								.then((result: any) => {
									if (result.success) {
										operation.set("fail-ok");
										output.set(result.data.Message);
										return;
									}
									operation.set("fail-error");
									output.set(result.error);
								});
						},
					},
					"Fail",
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
			h(
				"div",
				{ "data-bmb-echo-operation": true },
				dynamic(() => {
					return operation.value();
				}),
			),
		);
	},
});
