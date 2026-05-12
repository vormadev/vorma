// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import { createPopupRelationship } from "./popup-behavior.ts";

describe("popup behavior", () => {
	it("tracks trigger and popup targets", () => {
		const relationship = createPopupRelationship();
		const trigger = document.createElement("button");
		const popup = document.createElement("div");
		const popup_child = document.createElement("span");
		const outside = document.createElement("button");
		popup.append(popup_child);
		document.body.append(trigger, popup, outside);

		relationship.registerTrigger(trigger);
		relationship.registerPopup(popup);

		expect(relationship.getTrigger()).toBe(trigger);
		expect(relationship.getPopup()).toBe(popup);
		expect(relationship.containsTarget(trigger)).toBe(true);
		expect(relationship.containsTarget(popup_child)).toBe(true);
		expect(relationship.containsTarget(outside)).toBe(false);

		trigger.remove();
		popup.remove();
		outside.remove();
	});

	it("syncs the native popup host visibility", () => {
		const relationship = createPopupRelationship();
		const popup = document.createElement("div");
		popup.hidden = true;
		document.body.append(popup);
		relationship.registerPopup(popup);

		relationship.syncPopup(true);
		expect(popup.hidden).toBe(false);

		relationship.syncPopup(false);
		expect(popup.hidden).toBe(true);

		popup.remove();
	});

	it("can return focus to the trigger", () => {
		const relationship = createPopupRelationship();
		const trigger = document.createElement("button");
		document.body.append(trigger);
		relationship.registerTrigger(trigger);

		relationship.focusTrigger();
		expect(document.activeElement).toBe(trigger);

		trigger.remove();
	});
});
