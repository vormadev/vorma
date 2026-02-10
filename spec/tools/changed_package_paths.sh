#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

if [ ! -f spec/PACKAGE_CATALOG.tsv ]; then
	echo "missing spec/PACKAGE_CATALOG.tsv" >&2
	exit 1
fi

package_paths_file=$(mktemp)
touched_file=$(mktemp)
trap 'rm -f "$package_paths_file" "$touched_file"' EXIT
awk -F '\t' 'NR > 1 {print $3}' spec/PACKAGE_CATALOG.tsv \
	| awk '{print length, $0}' \
	| sort -rn \
	| cut -d' ' -f2- > "$package_paths_file"

while IFS= read -r file; do
	[[ "$file" == spec/packages/* ]] || continue
	matched=""
	while IFS= read -r package_path; do
		if [[ "$file" == "$package_path" || "$file" == "$package_path/"* ]]; then
			matched="$package_path"
			break
		fi
	done < "$package_paths_file"
	if [ -z "$matched" ]; then
		echo "changed package file does not map to a catalog package: $file" >&2
		exit 1
	fi
	printf '%s\n' "$matched" >> "$touched_file"
done < <(spec/tools/changed_files.sh)

sort -u "$touched_file" | sed '/^$/d'
