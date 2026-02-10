#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"
export LC_ALL=C

spec_path="${1:-}"
if [ -z "$spec_path" ]; then
	echo "usage: spec/tools/compute_package_artifact_hash.sh spec/packages/<path>" >&2
	exit 1
fi

if [ ! -d "$spec_path" ]; then
	echo "spec path not found: $spec_path" >&2
	exit 1
fi

artifact_files=(
	"00-scope.md"
	"10-capabilities.md"
	"20-requirements.md"
	"30-state-model.md"
	"40-errors.md"
	"50-external-contracts.md"
	"60-nonfunctional.md"
	"70-open-questions.md"
	"80-assertion-accounting.md"
	"evidence.yaml"
)

if command -v shasum >/dev/null 2>&1; then
	hash_cmd=(shasum -a 256)
elif command -v sha256sum >/dev/null 2>&1; then
	hash_cmd=(sha256sum)
else
	echo "missing hash command: need shasum or sha256sum" >&2
	exit 1
fi

manifest=$(mktemp)
trap 'rm -f "$manifest"' EXIT

for file in "${artifact_files[@]}"; do
	full_path="$spec_path/$file"
	if [ ! -f "$full_path" ]; then
		echo "missing artifact file: $full_path" >&2
		exit 1
	fi
	file_hash=$("${hash_cmd[@]}" "$full_path" | awk '{print $1}')
	printf '%s  %s\n' "$file_hash" "$file" >> "$manifest"
done

"${hash_cmd[@]}" "$manifest" | awk '{print $1}'
