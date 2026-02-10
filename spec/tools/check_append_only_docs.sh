#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

for file in spec/DECISIONS.md spec/TRACEABILITY.md; do
	if ! git diff --name-only HEAD -- "$file" | rg -q .; then
		continue
	fi

	if git diff --unified=0 HEAD -- "$file" | rg -q '^-[^-]'; then
		echo "$file is append-only during mining; deletions/modifications are prohibited" >&2
		exit 1
	fi
done
