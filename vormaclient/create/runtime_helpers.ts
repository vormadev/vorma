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

type ParsedMajorMinorPatchVersion = {
	major: number;
	minor: number;
	patch: number;
};

function parseMajorMinorPatchVersionFromRegexMatch(props: {
	match: RegExpMatchArray | null;
	allowMissingPatch: boolean;
}): ParsedMajorMinorPatchVersion | null {
	const { match, allowMissingPatch } = props;
	if (!match) {
		return null;
	}

	const majorText = match[1];
	const minorText = match[2];
	const patchText = match[3];

	if (!majorText || !minorText) {
		return null;
	}

	const major = Number.parseInt(majorText, 10);
	const minor = Number.parseInt(minorText, 10);
	const patch = Number.parseInt(
		patchText || (allowMissingPatch ? "0" : ""),
		10,
	);
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
	};
}

export function parseNodeVersionOrNull(
	nodeVersion: string,
): ParsedNodeVersion | null {
	const version = parseMajorMinorPatchVersionFromRegexMatch({
		match: nodeVersion.trim().match(/^v(\d+)\.(\d+)\.(\d+)$/),
		allowMissingPatch: false,
	});
	if (!version) {
		return null;
	}

	return version;
}

export function isNodeVersionAtLeast(props: {
	nodeVersion: string;
	minimumMajor: number;
	minimumMinor: number;
}): boolean {
	const { nodeVersion, minimumMajor, minimumMinor } = props;
	return isMajorMinorVersionAtLeast({
		parsedVersion: parseNodeVersionOrNull(nodeVersion),
		minimumMajor,
		minimumMinor,
	});
}

export function parseGoVersionOrNull(
	goVersionOutput: string,
): ParsedGoVersion | null {
	const version = parseMajorMinorPatchVersionFromRegexMatch({
		match: goVersionOutput
			.trim()
			.match(/(?:^|\s)go(\d+)\.(\d+)(?:\.(\d+))?/),
		allowMissingPatch: true,
	});
	if (!version) {
		return null;
	}

	return {
		major: version.major,
		minor: version.minor,
		patch: version.patch,
		normalizedVersion: `go${version.major}.${version.minor}.${version.patch}`,
	};
}

export function isGoVersionAtLeast(props: {
	goVersionOutput: string;
	minimumMajor: number;
	minimumMinor: number;
}): boolean {
	const { goVersionOutput, minimumMajor, minimumMinor } = props;
	return isMajorMinorVersionAtLeast({
		parsedVersion: parseGoVersionOrNull(goVersionOutput),
		minimumMajor,
		minimumMinor,
	});
}

type MajorMinorVersion = {
	major: number;
	minor: number;
} | null;

function isMajorMinorVersionAtLeast(props: {
	parsedVersion: MajorMinorVersion;
	minimumMajor: number;
	minimumMinor: number;
}): boolean {
	const { parsedVersion, minimumMajor, minimumMinor } = props;
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
