#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

touched_paths=$(spec/tools/changed_package_paths.sh || true)
touched_count=$(printf '%s\n' "$touched_paths" | sed '/^$/d' | wc -l | tr -d ' ')

if [ "$touched_count" -gt 1 ]; then
	echo "multiple spec package paths changed in one patch:" >&2
	printf '%s\n' "$touched_paths" | sed '/^$/d; s/^/ - /' >&2
	exit 1
fi
