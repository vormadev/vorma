// @vitest-environment jsdom

import { vi } from "vitest";

vi.mock("./create_client_core.ts", async (import_original) => {
	const legacy =
		await import_original<typeof import("./create_client_core.ts")>();
	const effect = await import("./create_client_core_effect.ts");
	return {
		...legacy,
		create_client_core: effect.create_client_core_effect,
	};
});

await import("./router_revalidation.test.ts");
