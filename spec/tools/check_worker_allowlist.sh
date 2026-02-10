#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

violations=()
while IFS= read -r file; do
	[ -n "$file" ] || continue
	case "$file" in
		spec/packages/*)
			;;
		spec/MINING_DISPATCH.md)
			;;
		spec/DECISIONS.md)
			;;
		spec/TRACEABILITY.md)
			;;
		*)
			violations+=("$file")
			;;
	esac
done < <(spec/tools/changed_files.sh)

if [ "${#violations[@]}" -gt 0 ]; then
	echo "worker allowlist violation: these files are not editable during mining:" >&2
	printf ' - %s\n' "${violations[@]}" >&2
	echo "allowed paths: spec/packages/<claimed-path>/**, spec/MINING_DISPATCH.md, spec/DECISIONS.md, spec/TRACEABILITY.md" >&2
	exit 1
fi
