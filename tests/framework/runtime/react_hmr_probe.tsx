import { createElement } from "react";

type HmrProbeProps = {
	state_box: {
		value: () => string;
		set: (next: string) => void;
	};
};

export function HmrProbe(props: HmrProbeProps) {
	return createElement(
		"section",
		{ "data-bmb-hmr-panel": true },
		createElement("div", { "data-bmb-hmr-marker": true }, "client-hmr-marker-a"),
		createElement("div", { "data-bmb-hmr-state": true }, props.state_box.value()),
		createElement(
			"button",
			{
				"data-bmb-action": "hmr-state",
				"data-bmb-hmr-action": true,
				type: "button",
				onClick: () => {
					props.state_box.set("modified");
				},
			},
			"HMR state",
		),
	);
}
