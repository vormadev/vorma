import { createIsolatedClientTestRuntime } from "vorma/testing";

export function installDistTestVormaGlobal(): void {
	createIsolatedClientTestRuntime();
}
