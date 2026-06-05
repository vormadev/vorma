import { h } from "preact";

type HmrProbeProps = {
	state_box: {
		value: () => string;
		set: (next: string) => void;
	};
};

export function HmrProbe(props: HmrProbeProps) {
	return h(
		"section",
		{ "data-bmb-hmr-panel": true },
		h("div", { "data-bmb-hmr-marker": true }, "client-hmr-marker-a"),
		h("div", { "data-bmb-hmr-state": true }, props.state_box.value()),
		h(
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
