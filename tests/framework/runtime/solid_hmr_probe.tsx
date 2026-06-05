type HmrProbeProps = {
	state_box: {
		value: () => string;
		set: (next: string) => void;
	};
};

export function HmrProbe(props: HmrProbeProps) {
	return (
		<section data-bmb-hmr-panel={true}>
			<div data-bmb-hmr-marker={true}>client-hmr-marker-a</div>
			<div data-bmb-hmr-state={true}>{props.state_box.value()}</div>
			<button
				data-bmb-action="hmr-state"
				data-bmb-hmr-action={true}
				type="button"
				onClick={() => {
					props.state_box.set("modified");
				}}
			>
				HMR state
			</button>
		</section>
	);
}
