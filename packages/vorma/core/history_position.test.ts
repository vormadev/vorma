// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import { HISTORY_KEY_FIELD, HISTORY_USER_STATE_FIELD } from "./constants.ts";
import { create_history_position } from "./history_position.ts";

describe("commit", () => {
	it("pushes a key-stamped entry and tracks it as the current position", () => {
		const browser = create_history_position();
		const committed = browser.commit("/next", false, { from: "test" });

		expect(committed.key).not.toBe("");
		expect(browser.position()).toBe(committed);
		expect(window.history.state[HISTORY_KEY_FIELD]).toBe(committed.key);
		expect(window.history.state[HISTORY_USER_STATE_FIELD]).toEqual({
			from: "test",
		});
	});

	it("replace preserves foreign state fields while restamping the key", () => {
		const browser = create_history_position();
		window.history.replaceState({ foreign: 1 }, "", window.location.href);

		const committed = browser.commit("/replaced", true, undefined);
		expect(window.history.state.foreign).toBe(1);
		expect(window.history.state[HISTORY_KEY_FIELD]).toBe(committed.key);
	});
});

describe("ensure_window_key", () => {
	it("stamps a key onto a keyless entry and adopts the result", () => {
		const browser = create_history_position();
		window.history.replaceState({ foreign: 2 }, "", window.location.href);

		browser.ensure_window_key();
		const position = browser.position();
		expect(position.key).not.toBe("");
		expect(window.history.state.foreign).toBe(2);
		expect(window.history.state[HISTORY_KEY_FIELD]).toBe(position.key);
	});

	it("keeps an existing key", () => {
		const browser = create_history_position();
		browser.commit("/keyed", true, "user");
		const key = browser.position().key;

		browser.ensure_window_key();
		expect(browser.position().key).toBe(key);
		expect(browser.position().state).toBe("user");
	});
});

describe("read_from_window and adopt", () => {
	it("round-trips committed user state through window.history", () => {
		const browser = create_history_position();
		browser.commit("/somewhere", true, { n: 7 });

		const read = browser.read_from_window();
		expect(read.key).toBe(browser.position().key);
		expect(read.state).toEqual({ n: 7 });

		const other = create_history_position();
		other.adopt(read);
		expect(other.position()).toBe(read);
	});
});
