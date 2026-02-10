#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

if [ ! -f spec/PACKAGE_CATALOG.tsv ]; then
	echo "missing spec/PACKAGE_CATALOG.tsv" >&2
	exit 1
fi

render_template() {
	local template_path="$1"
	local output_path="$2"
	local package_name="$3"
	local spec_path="$4"
	local source_roots="$5"
	local req_prefix="$6"
	awk \
		-v package_name="$package_name" \
		-v spec_path="$spec_path" \
		-v source_roots="$source_roots" \
		-v req_prefix="$req_prefix" '
		{
			gsub(/__PACKAGE_NAME__/, package_name)
			gsub(/__SPEC_PATH__/, spec_path)
			gsub(/__SOURCE_ROOTS__/, source_roots)
			gsub(/__REQ_PREFIX__/, req_prefix)
			print
		}
	' "$template_path" > "$output_path"
}

while IFS=$'\t' read -r slot_id priority_group spec_path source_roots; do
	[ "$slot_id" = "slot_id" ] && continue

	mkdir -p "$spec_path"

	package_rel=${spec_path#spec/packages/}
	package_name=$package_rel
	req_prefix=$(printf '%s' "$package_rel" | tr '[:lower:]' '[:upper:]' | sed -E 's/[^A-Z0-9]+/-/g; s/^-+//; s/-+$//')

	render_template spec/_templates/package/00-scope.md "$spec_path/00-scope.md" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
	render_template spec/_templates/package/10-capabilities.md "$spec_path/10-capabilities.md" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
	render_template spec/_templates/package/20-requirements.md "$spec_path/20-requirements.md" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
	render_template spec/_templates/package/30-state-model.md "$spec_path/30-state-model.md" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
	render_template spec/_templates/package/40-errors.md "$spec_path/40-errors.md" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
	render_template spec/_templates/package/50-external-contracts.md "$spec_path/50-external-contracts.md" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
	render_template spec/_templates/package/60-nonfunctional.md "$spec_path/60-nonfunctional.md" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
	render_template spec/_templates/package/70-open-questions.md "$spec_path/70-open-questions.md" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
	render_template spec/_templates/package/80-assertion-accounting.md "$spec_path/80-assertion-accounting.md" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
	render_template spec/_templates/package/evidence.yaml "$spec_path/evidence.yaml" "$package_name" "$spec_path" "$source_roots" "$req_prefix"
done < spec/PACKAGE_CATALOG.tsv

echo "scaffolded package specs from spec/PACKAGE_CATALOG.tsv"
