export type ParsedNodeVersion = {
	major: number;
	minor: number;
	patch: number;
};

export type ParsedGoVersion = {
	major: number;
	minor: number;
	patch: number;
	normalizedVersion: string;
};

export function parseNodeVersionOrNull(
	nodeVersion: string,
): ParsedNodeVersion | null {
	const versionMatch = nodeVersion.trim().match(/^v(\d+)\.(\d+)\.(\d+)$/);
	if (!versionMatch) {
		return null;
	}

	const major = Number.parseInt(versionMatch[1] || "", 10);
	const minor = Number.parseInt(versionMatch[2] || "", 10);
	const patch = Number.parseInt(versionMatch[3] || "", 10);
	if (
		!Number.isFinite(major) ||
		!Number.isFinite(minor) ||
		!Number.isFinite(patch)
	) {
		return null;
	}

	return { major, minor, patch };
}

export function isNodeVersionAtLeast(
	nodeVersion: string,
	minimumMajor: number,
	minimumMinor: number,
): boolean {
	return isMajorMinorVersionAtLeast(
		parseNodeVersionOrNull(nodeVersion),
		minimumMajor,
		minimumMinor,
	);
}

export function parseGoVersionOrNull(
	goVersionOutput: string,
): ParsedGoVersion | null {
	const versionMatch = goVersionOutput
		.trim()
		.match(/(?:^|\s)go(\d+)\.(\d+)(?:\.(\d+))?/);
	if (!versionMatch) {
		return null;
	}

	const major = Number.parseInt(versionMatch[1] || "", 10);
	const minor = Number.parseInt(versionMatch[2] || "", 10);
	const patch = Number.parseInt(versionMatch[3] || "0", 10);
	if (
		!Number.isFinite(major) ||
		!Number.isFinite(minor) ||
		!Number.isFinite(patch)
	) {
		return null;
	}

	return {
		major,
		minor,
		patch,
		normalizedVersion: `go${major}.${minor}.${patch}`,
	};
}

export function isGoVersionAtLeast(
	goVersionOutput: string,
	minimumMajor: number,
	minimumMinor: number,
): boolean {
	return isMajorMinorVersionAtLeast(
		parseGoVersionOrNull(goVersionOutput),
		minimumMajor,
		minimumMinor,
	);
}

type MajorMinorVersion = {
	major: number;
	minor: number;
} | null;

function isMajorMinorVersionAtLeast(
	parsedVersion: MajorMinorVersion,
	minimumMajor: number,
	minimumMinor: number,
): boolean {
	if (!parsedVersion) {
		return false;
	}

	if (parsedVersion.major > minimumMajor) {
		return true;
	}

	if (parsedVersion.major < minimumMajor) {
		return false;
	}

	return parsedVersion.minor >= minimumMinor;
}

export function buildGoModInitCommandArgs(moduleName: string): Array<string> {
	return ["mod", "init", moduleName];
}

export function buildGoModReplaceCommandArgs(vormaPath: string): Array<string> {
	return [
		"mod",
		"edit",
		"-replace",
		`github.com/vormadev/vorma=${vormaPath}`,
	];
}

export function buildGoGetCommandArgs(version: string): Array<string> {
	return ["get", `github.com/vormadev/vorma@v${version}`];
}

export function buildGoRunCommandArgs(bootstrapFile: string): Array<string> {
	return ["run", bootstrapFile];
}
