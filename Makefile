#####################################################################
####### GLOBAL
#####################################################################

e2e:
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-prod
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev -variant react
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev -variant preact
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev -variant solid
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev -variant remix
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev-changes -variant react
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev-changes -variant preact
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev-changes -variant solid
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev-changes -variant remix

#####################################################################
####### RUST
#####################################################################

rust-build:
	cargo build --workspace --all-targets

rust-build-client-wasm:
	cargo build -p vorma-client-wasm \
		--target wasm32-unknown-unknown \
		--profile wasm-release
	wasm-opt --enable-bulk-memory -Oz target/wasm32-unknown-unknown/wasm-release/vorma_client_wasm.wasm \
		-o packages/vorma/core/client_wasm/vorma_client_wasm_bg.wasm

rust-fmt:
	cargo fmt --all

rust-fmt-check:
	cargo fmt --all --check

rust-lint:
	cargo clippy --workspace --all-targets -- -D warnings

rust-lint-fix:
	cargo clippy --fix --workspace --all-targets --allow-dirty --allow-staged -- -D warnings

rust-package:
	cargo package --workspace --exclude vorma-xtask --allow-dirty

rust-test:
	cargo test --workspace --all-targets
	cargo test --workspace --doc

rust-gate: rust-fmt-check rust-lint rust-test rust-build rust-build-client-wasm rust-package

#####################################################################
####### TYPESCRIPT
#####################################################################

ts-install:
	pnpm install --config.confirmModulesPurge=false

ts-fmt:
	pnpm exec oxfmt --config=oxfmt.config.ts --write .

ts-fmt-check:
	pnpm exec oxfmt --config=oxfmt.config.ts --check .

ts-lint:
	pnpm exec oxlint --config=oxlint.config.ts .

ts-lint-fix:
	pnpm exec oxlint --config=oxlint.config.ts --fix .

ts-typecheck: ts-build
	pnpm exec tsgo -p packages/vorma/kit --pretty false
	pnpm exec tsgo -p packages/vorma/core --pretty false
	pnpm exec tsgo -p packages/vorma/ui/preact --pretty false
	pnpm exec tsgo -p packages/vorma/ui/react --pretty false
	pnpm exec tsgo -p packages/vorma/ui/remix --pretty false
	pnpm exec tsgo -p packages/vorma/ui/solid --pretty false
	pnpm exec tsgo -p packages/vorma/vite --pretty false
	pnpm exec tsgo -p packages/vorma/tests --pretty false
	pnpm exec tsgo -p packages/create-vorma --pretty false

ts-test:
	pnpm exec vitest run --reporter=dot

ts-build: ts-install rust-build-client-wasm
	pnpm exec tsdown
	mkdir -p packages/vorma/.dist/core
	cp packages/vorma/core/client_wasm/vorma_client_wasm_bg.wasm \
		packages/vorma/.dist/core/vorma_client_wasm_bg.wasm

ts-gate: ts-fmt-check ts-lint ts-typecheck ts-test rust-build-client-wasm

#####################################################################
####### RELEASES
#####################################################################

gate:
	@cargo run --manifest-path xtask/Cargo.toml --quiet

ts-publish-pre:
	test -n "$(version)" || { echo "version= is required"; exit 1; }
	test -n "$(pre)" || { echo "pre= is required"; exit 1; }
	pnpm version $(version)-pre.$(pre) --recursive --no-git-checks --no-git-tag-version --allow-same-version
	git add . && git commit -m 'v$(version)-pre.$(pre)' --no-verify && git tag v$(version)-pre.$(pre)
	pnpm publish --access public --recursive --tag pre

ts-publish:
	test -n "$(version)" || { echo "version= is required"; exit 1; }
	pnpm version $(version) --recursive --no-git-checks --no-git-tag-version --allow-same-version
	git add . && git commit -m 'v$(version)' --no-verify && git tag v$(version)
	pnpm publish --access public --recursive
