#####################################################################
####### RELEASES
#####################################################################

release: full-gate
	@go run ./internal/cmd/release

release-unsafe:
	@UNSAFE=1 go run ./internal/cmd/release

full-gate:
	@go run ./internal/cmd/full_gate

#####################################################################
####### GO
#####################################################################

gotest:
	@go test -race ./...

gotestloud:
	@go test -race -v ./...

staticcheck:
	@staticcheck ./...

runtime-deps-check:
	@go run ./internal/cmd/runtime_deps_guard

# call with `make gobench pkg=./kit/mux` (or whatever)
gobench:
	@go test -bench=. $(pkg)

#####################################################################
####### TYPESCRIPT
#####################################################################

tstest: tstest-source tstest-dist

tstest-source: npmbuild
	@pnpm vitest run --exclude "typescript/vorma/black_box_tests/dist/**"

tstest-dist: npmbuild
	@pnpm vitest --run --config typescript/vorma/black_box_tests/dist/vitest.config.ts

tstestwatch:
	@pnpm vitest --exclude "typescript/vorma/black_box_tests/dist/**"

tsbench:
	@npx vitest bench

nuke-node-modules:
	@go run ./internal/cmd/tsstate nuke-node-modules

tsinstall:
	@go run ./internal/cmd/tsstate install

tsreset: nuke-node-modules tsinstall

tslint:
	@pnpm oxlint --type-aware

tscheck: tscheck-kit tscheck-fw-client tscheck-fw-client-dist tscheck-fw-react tscheck-fw-solid tscheck-fw-preact tscheck-fw-vite tscheck-fw-create

tscheck-kit:
	@pnpm tsgo --noEmit --project ./typescript/kit

tscheck-fw-client:
	@pnpm tsgo --noEmit --project ./typescript/vorma/client

tscheck-fw-client-dist:
	@pnpm tsgo --noEmit --project ./vorma2/client/black_box_tests/dist/tsconfig.json

tscheck-fw-react:
	@pnpm tsgo --noEmit --project ./typescript/vorma/ui-adapters/react

tscheck-fw-solid:
	@pnpm tsgo --noEmit --project ./typescript/vorma/ui-adapters/solid

tscheck-fw-preact:
	@pnpm tsgo --noEmit --project ./typescript/vorma/ui-adapters/preact

tscheck-fw-vite:
	@pnpm tsgo --noEmit --project ./typescript/vorma/vite

tscheck-fw-create:
	@pnpm tsgo --noEmit --project ./typescript/vorma/create

tsfmt:
	@pnpm oxfmt

tsfmtcheck:
	@pnpm oxfmt --check

npmbuild:
	@go run ./internal/cmd/buildts

#####################################################################
####### E2E
#####################################################################

e2e-install:
	@go run ./internal/cmd/e2e install

e2e-install-browsers:
	@go run ./internal/cmd/e2e install-browsers

e2e-setup: e2e-install e2e-install-browsers

e2e-test: npmbuild e2e-setup
	@go run ./internal/cmd/e2e test $(PLAYWRIGHT_ARGS)

e2e-test-dev: npmbuild e2e-setup
	@go run ./internal/cmd/e2e test-dev $(PLAYWRIGHT_ARGS)

e2e-test-prod: npmbuild e2e-setup
	@go run ./internal/cmd/e2e test-prod $(PLAYWRIGHT_ARGS)

#####################################################################
####### OTHER
#####################################################################

docker-site:
	@docker build -t vorma-site -f Dockerfile.site .

docker-run-site:
	@docker run -d -p $(PORT):$(PORT) -e PORT=$(PORT) vorma-site

run-create: tsreset npmbuild nuke-node-modules
	@mkdir -p test_create.local && \
		cd test_create.local && \
		node ../typescript/vorma/create/dist/main.js --local-test

sum:
	@go run ./internal/scripts/sum.local

print-client-types-from-dist:
	@for f in \
		npm_dist/vorma2/client/_index.d.ts \
		npm_dist/vorma2/client/_testing.d.ts \
		npm_dist/vorma2/client/_buildtime.d.ts \
		npm_dist/vorma2/client/_internal.d.ts \
		npm_dist/vorma2/client/ui-adapters/react/_index.d.ts \
		npm_dist/vorma2/client/ui-adapters/preact/_index.d.ts \
		npm_dist/vorma2/client/ui-adapters/solid/_index.d.ts; do \
		echo "=== $$f ==="; cat "$$f"; echo; echo; \
	done

print-client-types-typecheck-file:
	@cat vorma2/client/black_box_tests/dist/public_api_types.typecheck.ts
