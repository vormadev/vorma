import { ref, type MixInput } from "remix/ui";

export type GroupLabelRelationship = {
	get_label_id: () => string;
	get_labelled_by: () => string | undefined;
	register_label: (node: HTMLElement | null) => void;
};

export type GroupLabelRelationshipInput = {
	label_id: string;
	on_change: () => void;
};

export function create_group_label_relationship(
	input: GroupLabelRelationshipInput,
): GroupLabelRelationship {
	let label_node: HTMLElement | null = null;

	const relationship: GroupLabelRelationship = {
		get_label_id: () => {
			return input.label_id;
		},
		get_labelled_by: () => {
			return label_node ? relationship.get_label_id() : undefined;
		},
		register_label: (node) => {
			const changed = label_node !== node;
			label_node = node;
			if (changed) {
				input.on_change();
			}
		},
	};

	return relationship;
}

export function create_group_label_ref_mix(
	relationship: GroupLabelRelationship,
): MixInput<HTMLElement> {
	return ref<HTMLElement>((node, signal) => {
		relationship.register_label(node);
		signal.addEventListener("abort", () => {
			relationship.register_label(null);
		});
	});
}
