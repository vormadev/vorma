import {
	createElement,
	createMixin,
	type ElementProps,
	type MixInput,
	type RemixElement,
} from "remix/ui";
import type { RecipeStyle } from "../../core/core.ts";

type StyleRule = {
	count: number;
	rule: string;
};

type StyleManager = {
	insert: (class_name: string, rule: string) => void;
	remove: (class_name: string) => void;
};

const component_style_attribute = "data-vorma-component-style";
const component_style_class_prefix = "vmxc";
const style_managers = new WeakMap<Document, StyleManager>();
const unitless_css_props = new Set([
	"aspect-ratio",
	"column-count",
	"columns",
	"flex-grow",
	"flex-order",
	"flex-shrink",
	"font-weight",
	"grid-area",
	"grid-column",
	"grid-row",
	"line-height",
	"opacity",
	"order",
	"orphans",
	"widows",
	"z-index",
	"zoom",
]);
const style_cache = new Map<string, { class_name: string; rule: string }>();

function camel_to_kebab(value: string): string {
	return value.replace(/[A-Z]/g, (letter) => {
		return `-${letter.toLowerCase()}`;
	});
}

function is_style_record(value: unknown): value is Record<string, unknown> {
	return value !== null && typeof value === "object" && !Array.isArray(value);
}

function stable_style_entries(value: unknown): unknown {
	if (Array.isArray(value)) {
		return value.map(stable_style_entries);
	}
	if (!is_style_record(value)) {
		return value;
	}
	return Object.entries(value)
		.sort(([left], [right]) => {
			return left.localeCompare(right);
		})
		.map(([key, next_value]) => {
			return [key, stable_style_entries(next_value)];
		});
}

function hash_style(style: RecipeStyle): string {
	const input = JSON.stringify(stable_style_entries(style));
	let hash = 0;
	for (let index = 0; index < input.length; index++) {
		hash = (hash << 5) - hash + input.charCodeAt(index);
		hash = hash | 0;
	}
	return Math.abs(hash).toString(36);
}

function normalize_css_value(key: string, value: unknown): string {
	if (typeof value === "number" && value !== 0) {
		const css_key = camel_to_kebab(key);
		if (!unitless_css_props.has(css_key) && !css_key.startsWith("--")) {
			return `${value}px`;
		}
	}
	return String(value);
}

function is_keyframes_at_rule(key: string): boolean {
	const lower_key = key.toLowerCase();
	return (
		lower_key.startsWith("@keyframes") ||
		lower_key.startsWith("@-webkit-keyframes") ||
		lower_key.startsWith("@-moz-keyframes") ||
		lower_key.startsWith("@-o-keyframes")
	);
}

function serialize_declarations(style: Record<string, unknown>): string {
	const declarations: string[] = [];
	for (const [key, value] of Object.entries(style)) {
		if (value === undefined || value === null || is_style_record(value)) {
			continue;
		}
		declarations.push(
			`  ${camel_to_kebab(key)}: ${normalize_css_value(key, value)};`,
		);
	}
	return declarations.join("\n");
}

function serialize_keyframes(frames: Record<string, unknown>): string {
	const blocks: string[] = [];
	for (const [selector, value] of Object.entries(frames)) {
		if (!is_style_record(value)) {
			continue;
		}

		const declarations = serialize_declarations(value);
		blocks.push(`${selector} {\n${declarations}\n}`);
	}
	return blocks.join("\n");
}

function serialize_style(style: RecipeStyle, selector: string): string {
	const blocks: string[] = [];
	const declarations = serialize_declarations(style);
	if (declarations) {
		blocks.push(`${selector} {\n${declarations}\n}`);
	}

	for (const [key, value] of Object.entries(style)) {
		if (!is_style_record(value)) {
			continue;
		}

		if (key.startsWith("@")) {
			if (is_keyframes_at_rule(key)) {
				blocks.push(`${key} {\n${serialize_keyframes(value)}\n}`);
				continue;
			}
			const nested = serialize_style(value as RecipeStyle, selector);
			if (nested) {
				blocks.push(`${key} {\n${nested}\n}`);
			}
			continue;
		}

		const nested_selector = key.includes("&")
			? key.replaceAll("&", selector)
			: `${selector} ${key}`;
		const nested = serialize_style(value as RecipeStyle, nested_selector);
		if (nested) {
			blocks.push(nested);
		}
	}

	return blocks.join("\n");
}

function process_component_style(style: RecipeStyle): {
	class_name: string;
	rule: string;
} {
	if (Object.keys(style).length === 0) {
		return {
			class_name: "",
			rule: "",
		};
	}

	const hash = hash_style(style);
	const cached = style_cache.get(hash);
	if (cached) {
		return cached;
	}

	const class_name = `${component_style_class_prefix}-${hash}`;
	const rule = serialize_style(style, `.${class_name}`);
	const result = {
		class_name,
		rule,
	};
	style_cache.set(hash, result);
	return result;
}

function get_style_element(document: Document): HTMLStyleElement {
	const existing = document.querySelector<HTMLStyleElement>(
		`style[${component_style_attribute}]`,
	);
	if (existing) {
		return existing;
	}

	const style = document.createElement("style");
	style.setAttribute(component_style_attribute, "");
	(document.head ?? document.documentElement).append(style);
	return style;
}

function create_style_manager(document: Document): StyleManager {
	const rules = new Map<string, StyleRule>();

	function sync(): void {
		get_style_element(document).textContent = Array.from(rules.values())
			.map((entry) => {
				return entry.rule;
			})
			.join("\n");
	}

	return {
		insert: (class_name, rule) => {
			const current = rules.get(class_name);
			if (current) {
				current.count++;
				return;
			}
			rules.set(class_name, {
				count: 1,
				rule,
			});
			sync();
		},
		remove: (class_name) => {
			const current = rules.get(class_name);
			if (!current) {
				return;
			}
			current.count--;
			if (current.count > 0) {
				return;
			}
			rules.delete(class_name);
			sync();
		},
	};
}

function get_style_manager(document: Document): StyleManager {
	const existing = style_managers.get(document);
	if (existing) {
		return existing;
	}

	const manager = create_style_manager(document);
	style_managers.set(document, manager);
	return manager;
}

function merge_class_name(
	current: ElementProps["className"],
	class_name: string,
): string {
	if (typeof current === "string" && current) {
		return `${current} ${class_name}`;
	}
	return class_name;
}

const component_style = createMixin<
	Element,
	[style: RecipeStyle],
	ElementProps
>((handle) => {
	let active_class_name = "";

	handle.addEventListener("remove", () => {
		if (!active_class_name || typeof document === "undefined") {
			return;
		}
		get_style_manager(document).remove(active_class_name);
		active_class_name = "";
	});

	return (style, props): RemixElement => {
		const { class_name, rule } = process_component_style(style);
		if (!class_name) {
			return createElement(handle.element, props ?? {});
		}

		if (typeof document !== "undefined") {
			const manager = get_style_manager(document);
			if (active_class_name && active_class_name !== class_name) {
				manager.remove(active_class_name);
			}
			if (active_class_name !== class_name) {
				manager.insert(class_name, rule);
				active_class_name = class_name;
			}
		}

		return createElement(handle.element, {
			...props,
			className: merge_class_name(props?.className, class_name),
		});
	};
});

export function create_component_style_mix(
	style: RecipeStyle,
): MixInput<Element> {
	return component_style(style);
}
