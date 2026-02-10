#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

phase1_status=$(awk -F ': ' '/^phase1_status:/{print $2; exit}' spec/PHASE_STATUS.md)

if [ "$phase1_status" = "COMPLETE" ]; then
	exit 0
fi

violations=()
while IFS= read -r file; do
	[[ -z "$file" ]] && continue
	if [[ "$file" != spec/* ]]; then
		violations+=("$file")
	fi
done < <(spec/tools/changed_files.sh)

if [ "${#violations[@]}" -gt 0 ]; then
	echo "phase1_status=$phase1_status, edits outside spec/** are prohibited:" >&2
	printf ' - %s\n' "${violations[@]}" >&2
	exit 1
fi
