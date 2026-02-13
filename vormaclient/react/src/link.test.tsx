import { describe, expect, it, vi } from "vitest";

const { makeFinalLinkPropsSpy, resolvePathSpy } = vi.hoisted(() => {
	return {
		makeFinalLinkPropsSpy: vi.fn(),
		resolvePathSpy: vi.fn(),
	};
});

vi.mock("vorma/client", () => {
	return {
		__makeFinalLinkProps: makeFinalLinkPropsSpy,
		__resolvePath: resolvePathSpy,
	};
});

import { VormaLink, makeTypedLink } from "./link.tsx";

function invokeMemoComponent(component: unknown, props: unknown) {
	if (typeof component === "function") {
		return (component as (p: unknown) => unknown)(props);
	}
	const maybeMemo = component as { type?: (p: unknown) => unknown };
	if (typeof maybeMemo.type === "function") {
		return maybeMemo.type(props);
	}
	throw new Error("Component is not invokable");
}

describe("react link adapter", () => {
	it("forwards final link handlers onto rendered anchor props", () => {
		const finalProps = {
			dataExternal: "external-link",
			onPointerEnter: vi.fn(),
			onFocus: vi.fn(),
			onPointerLeave: vi.fn(),
			onBlur: vi.fn(),
			onTouchCancel: vi.fn(),
			onClick: vi.fn(),
		};
		makeFinalLinkPropsSpy.mockReturnValue(finalProps);

		const result = invokeMemoComponent(VormaLink, {
			href: "/docs",
			id: "docs-link",
			children: "Docs",
			prefetch: "intent",
			scrollToTop: false,
			replace: true,
			state: { from: "test" },
		});
		const element = result as {
			props: Record<string, unknown>;
		};

		expect(makeFinalLinkPropsSpy).toHaveBeenCalledTimes(1);
		expect(element.props["data-external"]).toBe("external-link");
		expect(element.props.id).toBe("docs-link");
		expect(element.props.onPointerEnter).toBe(finalProps.onPointerEnter);
		expect(element.props.onFocus).toBe(finalProps.onFocus);
		expect(element.props.onPointerLeave).toBe(finalProps.onPointerLeave);
		expect(element.props.onBlur).toBe(finalProps.onBlur);
		expect(element.props.onTouchCancel).toBe(finalProps.onTouchCancel);
		expect(element.props.onClick).toBe(finalProps.onClick);
		expect(element.props.prefetch).toBeUndefined();
		expect(element.props.replace).toBeUndefined();
		expect(element.props.scrollToTop).toBeUndefined();
		expect(element.props.state).toBeUndefined();
	});

	it("builds typed hrefs with search and hash", () => {
		resolvePathSpy.mockReturnValue("/typed/path");
		makeFinalLinkPropsSpy.mockReturnValue({
			dataExternal: undefined,
			onPointerEnter: vi.fn(),
			onFocus: vi.fn(),
			onPointerLeave: vi.fn(),
			onBlur: vi.fn(),
			onTouchCancel: vi.fn(),
			onClick: vi.fn(),
		});

		const TypedLink = makeTypedLink({} as any, {
			className: "default-class",
		});
		const result = invokeMemoComponent(TypedLink, {
			pattern: "/typed/:id",
			params: { id: "42" },
			search: "?q=abc",
			hash: "#panel",
			state: { from: "typed-test" },
			className: "custom-class",
		});
		const element = result as {
			props: Record<string, unknown>;
		};

		expect(resolvePathSpy).toHaveBeenCalledTimes(1);
		expect(resolvePathSpy).toHaveBeenCalledWith({
			vormaAppConfig: {},
			type: "loader",
			props: {
				pattern: "/typed/:id",
				params: { id: "42" },
			},
		});
		expect(element.props.href).toBe(
			`${window.location.origin}/typed/path?q=abc#panel`,
		);
		expect(element.props.state).toEqual({ from: "typed-test" });
		expect(element.props.className).toBe("custom-class");
	});
});
