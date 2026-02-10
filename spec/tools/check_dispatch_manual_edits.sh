#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

if ! git diff --name-only HEAD -- spec/MINING_DISPATCH.md | rg -q .; then
	exit 0
fi

if ! git diff --unified=0 HEAD -- spec/MINING_DISPATCH.md | awk '
	/^diff --git / { next }
	/^index / { next }
	/^--- / { next }
	/^\+\+\+ / { next }
	/^@@ / { next }
	/^[-+]/ {
		if ($0 ~ /^[-+]SLOT-[0-9][0-9][0-9]\t(OPEN|CLAIMED|DONE)\t/) {
			next
		}
		print
		bad = 1
	}
	END { exit bad ? 1 : 0 }
'; then
	echo "spec/MINING_DISPATCH.md must only change via slot row transitions (use claim/mark scripts)." >&2
	exit 1
fi
