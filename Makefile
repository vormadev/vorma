FUZZ_RUNS ?= 4096

#####################################################################
####### GLOBAL
#####################################################################

# Runs the generic framework browser/runtime coverage across supported adapters.
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
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev-changes -variant react
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev-changes -variant preact
	cd tests/framework && \
		cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev-changes -variant solid

# Removes retained framework-test artifacts.
clean-bombadil:
	rm -rf tests/framework/.bombadil tests/framework/.dist.*

#####################################################################
####### RUST
#####################################################################

# Compiles all Rust workspace targets.
rust-build:
	cargo build --workspace --all-targets

# Rebuilds the browser matcher WASM artifact shipped through the TS package.
rust-build-client-wasm:
	cargo build -p vorma-client-wasm \
		--target wasm32-unknown-unknown \
		--profile wasm-release
	wasm-opt --enable-bulk-memory -Oz target/wasm32-unknown-unknown/wasm-release/vorma_client_wasm.wasm \
		-o packages/vorma/core/client_wasm/vorma_client_wasm_bg.wasm

# Formats Rust sources.
rust-fmt:
	cargo fmt --all
	cargo fmt --manifest-path fuzz/Cargo.toml --all

# Checks Rust formatting.
rust-fmt-check:
	cargo fmt --all --check
	cargo fmt --manifest-path fuzz/Cargo.toml --all --check

# Runs clippy across all Rust workspace targets with warnings denied.
rust-lint:
	cargo clippy --workspace --all-targets -- -D warnings

# Applies clippy fixes where possible.
rust-lint-fix:
	cargo clippy --fix --workspace --all-targets --allow-dirty --allow-staged -- -D warnings

# Runs dependency advisory, license, ban, and source policy checks.
rust-policy:
	cargo audit
	cargo deny check licenses bans sources advisories

# Builds Rust docs with warnings denied.
rust-doc:
	RUSTDOCFLAGS="-D warnings" cargo doc --workspace --no-deps

# Compiles benchmark targets without running benchmarks.
rust-bench:
	cargo bench --workspace --no-run

# Runs Rust fuzz targets through cargo-fuzz.
rust-fuzz:
	cd fuzz && cargo +nightly fuzz run matcher_patterns -- -runs=$(FUZZ_RUNS)
	cd fuzz && cargo +nightly fuzz run client_wasm_protocol -- -runs=$(FUZZ_RUNS)

# Packages publishable crates in dependency order.
rust-package:
	cargo package -p vorma-matcher --allow-dirty
	cargo package -p vorma-tasks --allow-dirty
	cargo package -p vorma-macros --allow-dirty --no-verify
	cargo package -p vorma --allow-dirty --no-verify
	cargo package -p vorma-build --allow-dirty --no-verify

# Runs Rust tests and doctests.
rust-test:
	cargo test --workspace --all-targets
	cargo test --workspace --doc

# Runs the full Rust confidence gate.
rust-gate: rust-fmt-check rust-policy rust-lint rust-test rust-build rust-doc rust-bench rust-build-client-wasm rust-fuzz rust-package

#####################################################################
####### TYPESCRIPT
#####################################################################

PNPM_INSTALL = pnpm install --config.confirmModulesPurge=false
PNPM_EXEC = pnpm --config.confirmModulesPurge=false exec

ts-install:
	$(PNPM_INSTALL)

ts-fmt:
	$(PNPM_EXEC) oxfmt --config=oxfmt.config.ts --write .

ts-fmt-check:
	$(PNPM_EXEC) oxfmt --config=oxfmt.config.ts --check .

ts-lint:
	$(PNPM_EXEC) oxlint --config=oxlint.config.ts .

ts-lint-fix:
	$(PNPM_EXEC) oxlint --config=oxlint.config.ts --fix .

ts-typecheck: ts-build
	$(PNPM_EXEC) tsgo -p packages/vorma/kit --pretty false
	$(PNPM_EXEC) tsgo -p packages/vorma/core --pretty false
	$(PNPM_EXEC) tsgo -p packages/vorma/ui/preact --pretty false
	$(PNPM_EXEC) tsgo -p packages/vorma/ui/react --pretty false
	$(PNPM_EXEC) tsgo -p packages/vorma/ui/solid --pretty false
	$(PNPM_EXEC) tsgo -p packages/vorma/vite --pretty false
	$(PNPM_EXEC) tsgo -p packages/vorma/tests --pretty false
	$(PNPM_EXEC) tsgo -p packages/create-vorma --pretty false

ts-test:
	$(PNPM_EXEC) vitest run --reporter=dot

ts-build: ts-install rust-build-client-wasm
	rm -rf packages/create-vorma/.dist packages/vorma/.dist
	$(PNPM_EXEC) tsdown
	mkdir -p packages/vorma/.dist/core
	cp packages/vorma/core/client_wasm/vorma_client_wasm_bg.wasm \
		packages/vorma/.dist/core/vorma_client_wasm_bg.wasm

ts-gate: ts-install ts-fmt-check ts-lint ts-typecheck ts-test rust-build-client-wasm

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
