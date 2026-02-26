import { defineFrameworkIntegrationSuite } from "./framework.integration.shared.ts";

defineFrameworkIntegrationSuite({
	runMode: "prod",
	uiAdapter: "solid",
	suiteName: "wave-vorma-runtime-prod-solid",
});
