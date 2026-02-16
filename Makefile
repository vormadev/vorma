#####################################################################
####### RELEASES
#####################################################################

release: full-gate
	@go run ./internal/cmd/release

full-gate:
	@go run ./internal/cmd/full_gate

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
####### TYPESCRIPT
#####################################################################

tstest: tstest-source tstest-dist

tstest-source:
	@pnpm vitest run --exclude "typescript/vorma/client/src/tests/dist/**"

tstest-dist:
	@pnpm vitest --run --config typescript/vorma/client/vitest.dist.config.ts

tstestwatch:
	@pnpm vitest --exclude "typescript/vorma/client/src/tests/dist/**"

tsbench:
	@npx vitest bench

nuke-node-modules:
	@rm -rf node_modules 2>/dev/null || true
	@find . -path "*/node_modules" -type d -exec rm -rf {} \; 2>/dev/null || true

tsinstall:
	@pnpm i
	@cd typescript/vorma/create && pnpm i

tsreset: nuke-node-modules tsinstall

tslint:
	@pnpm oxlint

tscheck: tscheck-kit tscheck-fw-client tscheck-fw-client-dist tscheck-fw-react tscheck-fw-solid tscheck-fw-preact tscheck-fw-vite tscheck-fw-create

tscheck-kit:
	@pnpm tsgo --noEmit --project ./typescript/kit

tscheck-fw-client:
	@pnpm tsgo --noEmit --project ./typescript/vorma/client

tscheck-fw-client-dist:
	@pnpm tsgo --noEmit --project ./typescript/vorma/client/src/tests/dist/tsconfig.json

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

npmbuild:
	@go run ./internal/cmd/buildts

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
	@go run ./internal/cmd/sum
