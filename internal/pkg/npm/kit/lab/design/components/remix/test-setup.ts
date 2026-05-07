import { beforeEach } from "vitest";

export function setup_remix_component_test_environment(): void {
	beforeEach(() => {
		Object.defineProperty(document, "adoptedStyleSheets", {
			configurable: true,
			value: [],
			writable: true,
		});
		Object.defineProperty(HTMLDialogElement.prototype, "show", {
			configurable: true,
			value: function show(this: HTMLDialogElement): void {
				this.open = true;
			},
		});
		Object.defineProperty(HTMLDialogElement.prototype, "showModal", {
			configurable: true,
			value: function showModal(this: HTMLDialogElement): void {
				this.open = true;
			},
		});
		Object.defineProperty(HTMLDialogElement.prototype, "close", {
			configurable: true,
			value: function close(this: HTMLDialogElement): void {
				if (!this.open) {
					return;
				}
				this.open = false;
				this.dispatchEvent(new Event("close"));
			},
		});
	});
}
