import { defineFrameworkIntegrationSuite } from "./framework.integration.shared.ts";

defineFrameworkIntegrationSuite({
	runMode: "prod",
	uiAdapter: "preact",
	suiteName: "wave-vorma-runtime-prod-preact",
});
