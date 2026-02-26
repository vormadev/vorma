import { defineFrameworkIntegrationSuite } from "./framework.integration.shared.ts";

defineFrameworkIntegrationSuite({
	runMode: "dev",
	uiAdapter: "preact",
	suiteName: "wave-vorma-runtime-dev-preact",
});
