import { beforeEach, describe, expect, it, vi } from "vitest";

const { setClientLoaderWaitFnSpy, registerClientLoaderPatternOrThrowSpy } =
	vi.hoisted(() => {
		return {
			setClientLoaderWaitFnSpy: vi.fn(),
			registerClientLoaderPatternOrThrowSpy: vi.fn(),
		};
	});

vi.mock("../../app/context.ts", async (importOriginal) => {
	const actual =
		await importOriginal<typeof import("../../app/context.ts")>();
	return {
		...actual,
		__vormaClientGlobal: {
			get: vi.fn(),
			set: vi.fn(),
		},
		setClientLoaderWaitFn: setClientLoaderWaitFnSpy,
	};
});

vi.mock("../../core/render_runtime.ts", () => {
	return {
		registerClientLoaderPatternOrThrow:
			registerClientLoaderPatternOrThrowSpy,
		setupClientLoaders: vi.fn(async () => {}),
	};
});

import { __registerClientLoaderForAdapter } from "../../core/extras.ts";

describe("client loader adapter registration", () => {
	beforeEach(() => {
		setClientLoaderWaitFnSpy.mockReset();
		registerClientLoaderPatternOrThrowSpy.mockReset();
	});

	it("registers pattern before installing wait function", () => {
		const waitFn = vi.fn(async () => "ok");
		__registerClientLoaderForAdapter({
			pattern: "/items/:id",
			waitFn: waitFn as any,
		});

		expect(registerClientLoaderPatternOrThrowSpy).toHaveBeenCalledTimes(1);
		expect(registerClientLoaderPatternOrThrowSpy).toHaveBeenCalledWith(
			"/items/:id",
		);
		expect(setClientLoaderWaitFnSpy).toHaveBeenCalledTimes(1);
		expect(setClientLoaderWaitFnSpy).toHaveBeenCalledWith(
			"/items/:id",
			waitFn,
		);
	});

	it("throws with pattern context when registration fails", () => {
		const registrationError = new Error(
			"Pattern registry has not been initialized.",
		);
		registerClientLoaderPatternOrThrowSpy.mockImplementation(() => {
			throw registrationError;
		});

		expect(() => {
			__registerClientLoaderForAdapter({
				pattern: "/broken",
				waitFn: vi.fn(async () => "ok") as any,
			});
		}).toThrow(
			'Failed to register client loader pattern "/broken": Pattern registry has not been initialized.',
		);
		expect(setClientLoaderWaitFnSpy).not.toHaveBeenCalled();
	});

	it("delegates failures to onRegistrationError without installing wait function", () => {
		const registrationError = new Error("invalid pattern");
		registerClientLoaderPatternOrThrowSpy.mockImplementation(() => {
			throw registrationError;
		});
		const onRegistrationError = vi.fn();

		expect(() => {
			__registerClientLoaderForAdapter({
				pattern: "/broken",
				waitFn: vi.fn(async () => "ok") as any,
				onRegistrationError,
			});
		}).not.toThrow();

		expect(onRegistrationError).toHaveBeenCalledTimes(1);
		expect(onRegistrationError).toHaveBeenCalledWith(registrationError);
		expect(setClientLoaderWaitFnSpy).not.toHaveBeenCalled();
	});
});
