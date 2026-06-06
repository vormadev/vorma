import { describe, expect, it } from "vitest";
import { create_client_matcher } from "./matcher.ts";

const test_view_patterns = ["/", "/stories/:story_id"] as const;

describe("create_client_matcher", () => {
	it("matches nested routes through the real wasm matcher", async () => {
		const matcher = await create_client_matcher();
		try {
			matcher.register_pattern("/");
			matcher.register_pattern("/stories/:story_id");
			matcher.register_pattern("/stories/:story_id/comments/:comment_id");
			matcher.register_pattern("/docs/*");

			expect(matcher.find_nested_matches("/stories/42/comments/9")).toEqual({
				params: {
					comment_id: "9",
					story_id: "42",
				},
				patterns: [
					"/",
					"/stories/:story_id",
					"/stories/:story_id/comments/:comment_id",
				],
				splat_values: [],
			});

			expect(matcher.find_nested_matches("/docs/guide/intro")).toEqual({
				params: {},
				patterns: ["/", "/docs/*"],
				splat_values: ["guide", "intro"],
			});
		} finally {
			matcher.free();
		}
	});

	it("returns null for no match and throws for invalid patterns", async () => {
		const matcher = await create_client_matcher();
		try {
			matcher.register_pattern("/");
			matcher.register_pattern("/users/:id");

			expect(matcher.find_nested_matches("/settings")).toBeNull();
			expect(() => {
				matcher.register_pattern("/:");
			}).toThrow(/pattern registration/);
			expect(matcher.find_nested_matches("/users/42")).toEqual({
				params: { id: "42" },
				patterns: ["/", "/users/:id"],
				splat_values: [],
			});
		} finally {
			matcher.free();
		}
	});

	it("matches generated view contract patterns through the real wasm matcher", async () => {
		const matcher = await create_client_matcher();
		try {
			for (const pattern of test_view_patterns) {
				matcher.register_pattern(pattern);
			}

			expect(matcher.find_nested_matches("/stories/42")).toEqual({
				params: { story_id: "42" },
				patterns: ["/", "/stories/:story_id"],
				splat_values: [],
			});
		} finally {
			matcher.free();
		}
	});

	it("rejects oversized matcher inputs before writing wasm memory", async () => {
		const matcher = await create_client_matcher();
		try {
			matcher.register_pattern("/");
			expect(() => {
				matcher.find_nested_matches(`/${"a".repeat(64 * 1024)}`);
			}).toThrow("Vorma client matcher input is too large");
		} finally {
			matcher.free();
		}
	});
});
