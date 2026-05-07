import { describe, expect, it } from "vitest";
import { create_controllable_state } from "./controllable-state.ts";

describe("controllable state", () => {
	it("updates local state and emits change details when uncontrolled", () => {
		let local_value = "light";
		const changes: { detail: string; value: string }[] = [];
		const state = create_controllable_state<string, { detail: string }>({
			getControlled: () => {
				return undefined;
			},
			getLocal: () => {
				return local_value;
			},
			getOnChange: () => {
				return (value, details) => {
					changes.push({ detail: details.detail, value });
				};
			},
			setLocal: (value) => {
				local_value = value;
			},
		});

		const changed = state.set("dark", { detail: "keyboard" });

		expect(changed).toBe(true);
		expect(state.get()).toBe("dark");
		expect(local_value).toBe("dark");
		expect(changes).toEqual([{ detail: "keyboard", value: "dark" }]);
	});

	it("emits changes without mutating local state when controlled", () => {
		let local_value = "light";
		let controlled_value: string | undefined = "system";
		const changes: string[] = [];
		const state = create_controllable_state<string, undefined>({
			getControlled: () => {
				return controlled_value;
			},
			getLocal: () => {
				return local_value;
			},
			getOnChange: () => {
				return (value) => {
					changes.push(value);
				};
			},
			setLocal: (value) => {
				local_value = value;
			},
		});

		const changed = state.set("dark", undefined);

		expect(changed).toBe(true);
		expect(state.get()).toBe("system");
		expect(local_value).toBe("light");
		expect(changes).toEqual(["dark"]);

		controlled_value = "dark";
		expect(state.get()).toBe("dark");
	});

	it("suppresses no-op updates", () => {
		let local_value = "light";
		const changes: string[] = [];
		const state = create_controllable_state<string, undefined>({
			getControlled: () => {
				return undefined;
			},
			getLocal: () => {
				return local_value;
			},
			getOnChange: () => {
				return (value) => {
					changes.push(value);
				};
			},
			setLocal: (value) => {
				local_value = value;
			},
		});

		const changed = state.set("light", undefined);

		expect(changed).toBe(false);
		expect(changes).toEqual([]);
	});
});
