// @vitest-environment jsdom

import { Effect } from "effect";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DATA_SCRIPT_ID, VORMA_ROOT_EL_ID } from "./constants.ts";
import {
	BrowserViewTransitionFailed,
	make_browser_view_runtime,
} from "./effect_runtime/browser_view_runtime.ts";

type TestViewTransition = {
	updateCallbackDone?: PromiseLike<unknown>;
	finished?: PromiseLike<unknown>;
};

function set_start_view_transition(
	start_view_transition: (callback: () => unknown) => TestViewTransition,
): void {
	Object.defineProperty(document, "startViewTransition", {
		configurable: true,
		value: start_view_transition,
	});
}

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

beforeEach(() => {
	vi.restoreAllMocks();
	document.body.innerHTML = "";
	document.title = "";
	Reflect.deleteProperty(document, "startViewTransition");
});

describe("ccc Effect browser view runtime experiment", () => {
	it("reads and validates the boot payload at the browser boundary", async () => {
		const script = document.createElement("script");
		script.id = DATA_SCRIPT_ID;
		script.type = "application/json";
		script.textContent = JSON.stringify({ ClientBuildID: "build-1" });
		document.body.appendChild(script);
		const runtime = Effect.runSync(make_browser_view_runtime());

		await expect(run_effect(runtime.read_initial_payload)).resolves.toEqual(
			{ ClientBuildID: "build-1" },
		);

		script.textContent = JSON.stringify(["not", "an", "object"]);
		const invalid_result = await run_effect(
			Effect.either(runtime.read_initial_payload),
		);
		expect(invalid_result).toMatchObject({
			_tag: "Left",
			left: {
				_tag: "InitialPayloadReadFailed",
				reason: "Vorma data script must contain an object.",
			},
		});
	});

	it("applies decoded hash scroll and injected coordinate scroll", async () => {
		const scroll_to = vi.fn<(x: number, y: number) => void>();
		const runtime = Effect.runSync(
			make_browser_view_runtime({ scroll_to }),
		);
		const element = document.createElement("section");
		element.id = "hello world";
		const scroll_into_view = vi.fn();
		element.scrollIntoView = scroll_into_view;
		document.body.appendChild(element);

		await run_effect(runtime.apply_scroll({ hash: "#hello%20world" }));
		await run_effect(runtime.apply_scroll({ x: 12, y: 34 }));

		expect(scroll_into_view).toHaveBeenCalledTimes(1);
		expect(scroll_to).toHaveBeenCalledWith(12, 34);
	});

	it("owns root element creation and reuse", () => {
		const runtime = Effect.runSync(make_browser_view_runtime());
		const first = Effect.runSync(runtime.get_root_el);
		const second = Effect.runSync(runtime.get_root_el);

		expect(first.id).toBe(VORMA_ROOT_EL_ID);
		expect(second).toBe(first);
		expect(document.body.firstElementChild).toBe(first);
	});

	it("runs publication inside a delayed view transition callback", async () => {
		const runtime = Effect.runSync(make_browser_view_runtime());
		const callbacks: Array<() => void> = [];
		const titles_at_publish: string[] = [];
		set_start_view_transition((callback) => {
			let resolve_update!: () => void;
			let reject_update!: (error: unknown) => void;
			const updateCallbackDone = new Promise<void>((resolve, reject) => {
				resolve_update = resolve;
				reject_update = reject;
			});
			callbacks.push(() => {
				Promise.resolve(callback()).then(
					() => {
						resolve_update();
					},
					(error: unknown) => {
						reject_update(error);
					},
				);
			});
			return { updateCallbackDone };
		});
		document.title = "Old";

		const result = run_effect(
			runtime.run_view_transition({
				enabled: true,
				publish: Effect.sync(() => {
					titles_at_publish.push(document.title);
					document.title = "New";
					return "published";
				}),
			}),
		);

		expect(callbacks).toHaveLength(1);
		expect(titles_at_publish).toEqual([]);
		callbacks[0]!();

		await expect(result).resolves.toBe("published");
		expect(titles_at_publish).toEqual(["Old"]);
		expect(document.title).toBe("New");
	});

	it("surfaces missing publication as a typed browser transition failure", async () => {
		const runtime = Effect.runSync(make_browser_view_runtime());
		set_start_view_transition(() => {
			return { updateCallbackDone: Promise.resolve() };
		});

		const result = await run_effect(
			Effect.either(
				runtime.run_view_transition({
					enabled: true,
					publish: Effect.succeed("unreachable"),
				}),
			),
		);

		expect(result._tag).toBe("Left");
		if (result._tag === "Left") {
			expect(result.left).toBeInstanceOf(BrowserViewTransitionFailed);
		}
	});
});
