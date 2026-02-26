import { defineFrameworkIntegrationSuite } from "./framework.integration.shared.ts";

defineFrameworkIntegrationSuite({
	runMode: "prod",
	uiAdapter: "react",
	suiteName: "wave-vorma-runtime-prod-react",
});
