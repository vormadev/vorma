import { useLoaderData } from "../vorma.bindings.ts";
import type { RouteProps } from "../vorma.gen/index.ts";

export function OddShapes(props: RouteProps<"/odd-shapes">) {
	const data = useLoaderData(props);
	const nestedBetaLength = () => data().nestedFlags.beta?.length ?? 0;
	const timelineSignature = () =>
		data()
			.timeline.map((row) => `${row.label}:${row.step}`)
			.join("|");

	return (
		<section id="e2e-odd-shapes-route">
			<p id="e2e-odd-empty-words-len">{data().emptyWords.length}</p>
			<p id="e2e-odd-number-matrix-shape">
				{data()
					.numberMatrix.map((row) => row.length)
					.join(",")}
			</p>
			<p id="e2e-odd-optional-note">
				{data().optionalNote === null ? "null" : data().optionalNote}
			</p>
			<p id="e2e-odd-nested-beta-len">{nestedBetaLength()}</p>
			<p id="e2e-odd-metadata-source">{data().metadata.source}</p>
			<p id="e2e-odd-mixed-negative">{data().mixedNumbers.negative}</p>
			<p id="e2e-odd-timeline-signature">{timelineSignature()}</p>
		</section>
	);
}
