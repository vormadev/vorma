import { describe, expect, it } from "vitest";
import { parseSegments } from "./parse_segments.ts";

describe("ParseSegments", () => {
	const tests = [
		{ name: "empty path", path: "", expected: [] },
		{ name: "root path", path: "/", expected: [""] },
		{ name: "simple path", path: "/users", expected: ["users"] },
		{
			name: "multi-segment path",
			path: "/api/v1/users",
			expected: ["api", "v1", "users"],
		},
		{ name: "trailing slash", path: "/users/", expected: ["users", ""] },
		{
			name: "path with parameters",
			path: "/users/:id/posts",
			expected: ["users", ":id", "posts"],
		},
		{
			name: "path with parameters, implicit index segment",
			path: "/users/:id/posts/",
			expected: ["users", ":id", "posts", ""],
		},
		{
			name: "path with parameters, explicit index segment",
			path: "/users/:id/posts/_index",
			expected: ["users", ":id", "posts", "_index"],
		},
		{ name: "path with splat", path: "/files/*", expected: ["files", "*"] },
		{
			name: "multiple slashes",
			path: "//api///users",
			expected: ["api", "users"],
		},
		{
			name: "complex path",
			path: "/api/v1/users/:user_id/posts/:post_id/comments",
			expected: [
				"api",
				"v1",
				"users",
				":user_id",
				"posts",
				":post_id",
				"comments",
			],
		},
		{
			name: "unicode path",
			path: "/café/über/resumé",
			expected: ["café", "über", "resumé"],
		},
	];

	for (const tt of tests) {
		it(tt.name, () => {
			const result = parseSegments(tt.path);
			expect(result).toEqual(tt.expected);
		});
	}
});
