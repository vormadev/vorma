import { setupClientLoaders } from "./client_loaders.ts";
import { ComponentLoader } from "./component_loader.ts";

export async function bootstrapInitialClientRuntime(
	importURLs: Array<string>,
): Promise<void> {
	// Load initial components
	await ComponentLoader.handleComponents(importURLs);

	// Setup client loaders
	await setupClientLoaders();

	// Handle error boundary component (must come after setupClientLoaders)
	await ComponentLoader.handleErrorBoundaryComponent(importURLs);
}
