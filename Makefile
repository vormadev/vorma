# Workflow notes:
# - `make gate` is the release gate: it runs rust-gate, ts-gate, and e2e as
#   separate steps with per-step logs in logs.local/ (via xtask). Gates run
#   formatters in WRITE mode on purpose — the gate normalizes the tree, then
#   verifies it; lint/type/test steps are check-only.
# - e2e consumes packages/vorma/.dist, so it depends on ts-build: ad-hoc
#   `make e2e` can never run against a stale package build.

FUZZ_RUNS ?= 4096
# Bench recordings are per-machine and additive: one file per machine under
# docs/maintainer/bench-results/<crate>/. Override on machines whose hostname
# is not the id you want (e.g. BENCH_MACHINE_ID=m3-max make bench-tasks).
BENCH_MACHINE_ID ?= $(shell hostname -s | tr '[:upper:]' '[:lower:]')
BENCH_RESULTS_DIR = docs/maintainer/bench-results
E2E_CMD_BASE = cd tests/framework && cargo run -p vorma-framework-tests --bin framework-bombadil --
RUST_PACKAGE_TARGET_DIR = target/package-gate
RUST_PACKAGE_LOCAL_CRATE_PATCHES = \
	--config 'patch.crates-io.vorma.path="crates/vorma"' \
	--config 'patch.crates-io.vorma-contract.path="crates/vorma-contract"' \
	--config 'patch.crates-io.vorma-macros.path="crates/vorma-macros"' \
	--config 'patch.crates-io.vorma-matcher.path="crates/vorma-matcher"' \
	--config 'patch.crates-io.vorma-tasks.path="crates/vorma-tasks"'
XTASK_CMD_BASE = cargo run -p vorma-xtask --quiet --

#####################################################################
####### GLOBAL
#####################################################################

# Runs the generic framework browser/runtime coverage across supported adapters.
e2e: ts-build
	$(E2E_CMD_BASE) test-prod
	$(E2E_CMD_BASE) test-dev
	$(E2E_CMD_BASE) test-dev-changes

# Runs the shortest end-to-end pass that still proves the whole pipeline
# (production build plus the react dev + dev-changes scenarios).
e2e-smoke: ts-build
	$(E2E_CMD_BASE) test-prod
	$(E2E_CMD_BASE) test-dev -variant react
	$(E2E_CMD_BASE) test-dev-changes -variant react

# Removes retained framework-test artifacts.
clean-bombadil:
	rm -rf tests/framework/.bombadil tests/framework/.dist.*

# Removes all local build/test/gate artifacts.
clean: clean-bombadil
	rm -rf logs.local $(RUST_PACKAGE_TARGET_DIR) packages/vorma/.dist packages/create-vorma/.dist

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

# Runs matcher benchmarks and records them for this machine.
bench-matcher:
	@mkdir -p $(BENCH_RESULTS_DIR)/vorma-matcher
	cargo bench -p vorma-matcher --bench matching --no-run
	cargo bench -p vorma-matcher --bench matching 2>/dev/null | tee $(BENCH_RESULTS_DIR)/vorma-matcher/$(BENCH_MACHINE_ID).bench.results.txt

# Model-checks the task store's wait/notify protocol under loom,
# exploring all thread interleavings the memory model allows.
loom-tasks:
	RUSTFLAGS="--cfg loom" LOOM_MAX_PREEMPTIONS=3 cargo test -p vorma-tasks --lib --release

# Runs task-runtime benchmarks and records them for this machine.
bench-tasks:
	@mkdir -p $(BENCH_RESULTS_DIR)/vorma-tasks
	cargo bench -p vorma-tasks --bench tasks --no-run
	cargo bench -p vorma-tasks --bench tasks 2>/dev/null | tee $(BENCH_RESULTS_DIR)/vorma-tasks/$(BENCH_MACHINE_ID).bench.results.txt

# Runs Rust fuzz targets against copied corpora.
rust-fuzz:
	@FUZZ_RUNS=$(FUZZ_RUNS) $(XTASK_CMD_BASE) rust-fuzz

# Packages publishable crates in dependency order.
rust-package:
	cargo clean --target-dir $(RUST_PACKAGE_TARGET_DIR)
	cargo package -p vorma-matcher --allow-dirty --target-dir $(RUST_PACKAGE_TARGET_DIR)
	cargo package -p vorma-tasks --allow-dirty --target-dir $(RUST_PACKAGE_TARGET_DIR)
	cargo package -p vorma-macros --allow-dirty --no-verify --target-dir $(RUST_PACKAGE_TARGET_DIR)
	cargo package -p vorma-contract --allow-dirty --no-verify --target-dir $(RUST_PACKAGE_TARGET_DIR)
	cargo package -p vorma --allow-dirty --no-verify --target-dir $(RUST_PACKAGE_TARGET_DIR) $(RUST_PACKAGE_LOCAL_CRATE_PATCHES)
	cargo package -p vorma-build --allow-dirty --no-verify --target-dir $(RUST_PACKAGE_TARGET_DIR) $(RUST_PACKAGE_LOCAL_CRATE_PATCHES)

# Runs Rust tests and doctests.
rust-test:
	cargo test --workspace --all-targets
	cargo test --workspace --doc

# Runs the full Rust confidence gate.
rust-gate: rust-fmt rust-policy rust-lint rust-test loom-tasks rust-build rust-doc rust-bench rust-build-client-wasm rust-package rust-fuzz

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

# rust-build-client-wasm and ts-build are reached through ts-typecheck.
ts-gate: ts-install ts-fmt ts-lint ts-typecheck ts-test

#####################################################################
####### RELEASES
#####################################################################

gate:
	$(XTASK_CMD_BASE) gate

ts-publish:
	$(XTASK_CMD_BASE) ts-publish

#####################################################################
####### PHONY
#####################################################################

.PHONY: e2e e2e-smoke clean clean-bombadil gate ts-publish \
	rust-build rust-build-client-wasm rust-fmt rust-fmt-check rust-lint \
	rust-lint-fix rust-policy rust-doc rust-bench bench-matcher bench-tasks loom-tasks rust-fuzz rust-package \
	rust-test rust-gate \
	ts-install ts-fmt ts-fmt-check ts-lint ts-lint-fix ts-typecheck \
	ts-test ts-build ts-gate
