release: release-gate
	@go run ./internal/scripts/release

release-gate:
	@$(MAKE) gotest
	@$(MAKE) tsreset
	@$(MAKE) tstest-source
	@$(MAKE) tslint
	@$(MAKE) tscheck
	@$(MAKE) npmbuild
	@$(MAKE) tstest-dist

#####################################################################
####### GO
#####################################################################

gotest:
	@go test -race ./...

gotestloud:
	@go test -race -v ./...

# call with `make gobench pkg=./kit/mux` (or whatever)
gobench:
	@go test -bench=. $(pkg)

#####################################################################
####### TS
#####################################################################

tstest: tstest-source tstest-dist

tstest-source:
	@pnpm vitest run --exclude "vormaclient/client/dist_tests/**"

tstest-dist:
	@pnpm vitest --run --config vormaclient/client/vitest.dist.config.ts

tstestwatch:
	@pnpm vitest --exclude "vormaclient/client/dist_tests/**"

tsbench:
	@npx vitest bench

nuke-node-modules:
	@rm -rf node_modules 2>/dev/null || true
	@find . -path "*/node_modules" -type d -exec rm -rf {} \; 2>/dev/null || true

tsinstall:
	@pnpm i
	@cd vormaclient/create && pnpm i

tsreset: nuke-node-modules tsinstall

tslint:
	@pnpm oxlint

tscheck: tscheck-kit tscheck-fw-client tscheck-fw-client-dist tscheck-fw-react tscheck-fw-solid tscheck-fw-preact tscheck-fw-vite tscheck-fw-create

tscheck-kit:
	@pnpm tsgo --noEmit --project ./kit/_typescript

tscheck-fw-client:
	@pnpm tsgo --noEmit --project ./vormaclient/client

tscheck-fw-client-dist:
	@pnpm tsgo --noEmit --project ./vormaclient/client/dist_tests/tsconfig.json

tscheck-fw-react:
	@pnpm tsgo --noEmit --project ./vormaclient/react

tscheck-fw-solid:
	@pnpm tsgo --noEmit --project ./vormaclient/solid

tscheck-fw-preact:
	@pnpm tsgo --noEmit --project ./vormaclient/preact

tscheck-fw-vite:
	@pnpm tsgo --noEmit --project ./vormaclient/vite

tscheck-fw-create:
	@pnpm tsgo --noEmit --project ./vormaclient/create

npmbuild:
	@go run ./internal/scripts/buildts

docker-site:
	@docker build -t vorma-site -f Dockerfile.site .

docker-run-site:
	@docker run -d -p $(PORT):$(PORT) -e PORT=$(PORT) vorma-site

sum:
	@go run ./internal/scripts/sum

run-create: tsreset npmbuild nuke-node-modules
	@mkdir -p test_create.local && \
		cd test_create.local && \
		node ../vormaclient/create/dist/main.js --local-test
