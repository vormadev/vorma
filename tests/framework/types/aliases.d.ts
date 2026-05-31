declare module "#variant-runtime" {
	export const variant: string;
	export const h: (...args: any[]) => any;
	export const class_prop: "class" | "className";
	export const input_event: "onChange" | "onInput";
	export const use_text_state: (initial: string) => [any, (next: string) => void];
	export function read_box(box: any): any;
	export function read_text_state(value: any): any;
	export function dynamic(read_value: () => any): any;
	export const use_loader_data: ((props: any) => any) | undefined;
	export const use_client_loader_data: ((props: any) => any) | undefined;
	export const use_route_state: (() => any) | undefined;
	export const use_work_state: (() => any) | undefined;
	export function prepare_view_definition(input: any): any;
	export function render_vorma(input: {
		RootOutlet: any;
		rootEl: HTMLElement;
	}): void | Promise<void>;
}

declare module "#vorma-client" {
	export const createVormaClient: any;
}

declare module "#vorma-gen" {
	export const vormaClientSeed: any;
}
