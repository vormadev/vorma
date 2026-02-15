import { describe, expect, it } from "vitest";
import {
	buildGoGetCommandArgs,
	buildGoModInitCommandArgs,
	buildGoModReplaceCommandArgs,
	buildGoRunCommandArgs,
	isGoVersionAtLeast,
	isNodeVersionAtLeast,
	parseGoVersionOrNull,
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
		expect(
			isNodeVersionAtLeast({
				nodeVersion: "v22.10.9",
				minimumMajor: 22,
				minimumMinor: 11,
			}),
		).toBe(false);
		expect(
			isNodeVersionAtLeast({
				nodeVersion: "v22.11.0",
				minimumMajor: 22,
				minimumMinor: 11,
			}),
		).toBe(true);
		expect(
			isNodeVersionAtLeast({
				nodeVersion: "v23.0.0",
				minimumMajor: 22,
				minimumMinor: 11,
			}),
		).toBe(true);
	});

	it("parses standard and devel go version output", () => {
		expect(
			parseGoVersionOrNull("go version go1.24.7 darwin/arm64"),
		).toEqual({
			major: 1,
			minor: 24,
			patch: 7,
			normalizedVersion: "go1.24.7",
		});

		expect(
			parseGoVersionOrNull(
				"go version devel go1.25-abcdef1234 Tue Jan 1 00:00:00 2026 +0000 darwin/arm64",
			),
		).toEqual({
			major: 1,
			minor: 25,
			patch: 0,
			normalizedVersion: "go1.25.0",
		});
	});

	it("returns null for invalid go version output", () => {
		expect(parseGoVersionOrNull("go version unknown")).toBeNull();
		expect(parseGoVersionOrNull("")).toBeNull();
	});

	it("enforces the documented minimum go version", () => {
		expect(
			isGoVersionAtLeast({
				goVersionOutput: "go version go1.23.9 linux/amd64",
				minimumMajor: 1,
				minimumMinor: 24,
			}),
		).toBe(false);
		expect(
			isGoVersionAtLeast({
				goVersionOutput: "go version go1.24.0 linux/amd64",
				minimumMajor: 1,
				minimumMinor: 24,
			}),
		).toBe(true);
		expect(
			isGoVersionAtLeast({
				goVersionOutput: "go version devel go1.25-abcdef linux/amd64",
				minimumMajor: 1,
				minimumMinor: 24,
			}),
		).toBe(true);
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
