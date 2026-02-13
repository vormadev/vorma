import { beforeEach, describe, expect, it, vi } from "vitest";

const { registerClientLoaderForAdapterSpy } = vi.hoisted(() => {
	return {
		registerClientLoaderForAdapterSpy: vi.fn(),
	};
});

vi.mock("vorma/client", () => {
	return {
		__registerClientLoaderForAdapter: registerClientLoaderForAdapterSpy,
	};
});

vi.mock("./solid.tsx", () => {
	return {
		clientLoadersData: () => [] as unknown[],
		loadersData: () => [] as unknown[],
		routerData: () => ({ matchedPatterns: [] as string[] }),
	};
});

import { makeTypedAddClientLoader } from "./helpers.ts";

describe("makeTypedAddClientLoader", () => {
	beforeEach(() => {
		registerClientLoaderForAdapterSpy.mockReset();
	});

	it("delegates registration through __registerClientLoaderForAdapter", () => {
		const addClientLoader = makeTypedAddClientLoader<any>();
		const clientLoader = vi.fn(async () => "ok");
		const reRunOnModuleChange = { url: "file:///tmp/mod.ts" } as ImportMeta;

		addClientLoader({
			pattern: "/dashboard" as any,
			clientLoader: clientLoader as any,
			reRunOnModuleChange,
		});

		expect(registerClientLoaderForAdapterSpy).toHaveBeenCalledTimes(1);
		expect(registerClientLoaderForAdapterSpy).toHaveBeenCalledWith({
			pattern: "/dashboard",
			waitFn: clientLoader,
			reRunOnModuleChange,
		});
	});
});
