import { describe, expect, it } from "vitest";
import {
	buildGoGetCommandArgs,
	buildGoModInitCommandArgs,
	buildGoModReplaceCommandArgs,
	buildGoRunCommandArgs,
	isNodeVersionAtLeast,
	parseNodeVersionOrNull,
} from "./runtime_helpers.js";

describe("create-vorma runtime helpers", () => {
	it("parses a valid node semver string", () => {
		expect(parseNodeVersionOrNull("v22.11.0")).toEqual({
			major: 22,
			minor: 11,
			patch: 0,
		});
	});

	it("returns null for an invalid node semver string", () => {
		expect(parseNodeVersionOrNull("22.11.0")).toBeNull();
		expect(parseNodeVersionOrNull("v22")).toBeNull();
		expect(parseNodeVersionOrNull("v22.x.0")).toBeNull();
	});

	it("enforces the documented minimum node version", () => {
		expect(isNodeVersionAtLeast("v22.10.9", 22, 11)).toBe(false);
		expect(isNodeVersionAtLeast("v22.11.0", 22, 11)).toBe(true);
		expect(isNodeVersionAtLeast("v23.0.0", 22, 11)).toBe(true);
	});

	it("builds go command args without shell interpolation", () => {
		expect(buildGoModInitCommandArgs("myapp; touch hacked.txt")).toEqual([
			"mod",
			"init",
			"myapp; touch hacked.txt",
		]);
		expect(buildGoModReplaceCommandArgs("/tmp/local-vorma")).toEqual([
			"mod",
			"edit",
			"-replace",
			"github.com/vormadev/vorma=/tmp/local-vorma",
		]);
		expect(buildGoGetCommandArgs("0.85.0-pre.0")).toEqual([
			"get",
			"github.com/vormadev/vorma@v0.85.0-pre.0",
		]);
		expect(buildGoRunCommandArgs("/tmp/bootstrap/main.go")).toEqual([
			"run",
			"/tmp/bootstrap/main.go",
		]);
	});
});
