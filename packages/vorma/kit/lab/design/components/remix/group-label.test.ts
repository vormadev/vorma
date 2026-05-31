// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import { create_group_label_relationship } from "./group-label.ts";

describe("group label relationship", () => {
	it("omits labelled-by until a label registers", () => {
		const changes: string[] = [];
		const relationship = create_group_label_relationship({
			label_id: "group-label",
			on_change: () => {
				changes.push("changed");
			},
		});
		const label = document.createElement("div");

		expect(relationship.get_label_id()).toBe("group-label");
		expect(relationship.get_labelled_by()).toBeUndefined();

		relationship.register_label(label);
		expect(relationship.get_labelled_by()).toBe("group-label");

		relationship.register_label(label);
		expect(changes).toEqual(["changed"]);

		relationship.register_label(null);
		expect(relationship.get_labelled_by()).toBeUndefined();
		expect(changes).toEqual(["changed", "changed"]);
	});
});
