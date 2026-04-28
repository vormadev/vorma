sum:
	@go run ./internal/cmd/sum/

prepforpub: gotest tsreset tstest tsfmt tslint tscheck

#####################################################################
####### GO
#####################################################################

gotest:
	@go test -race ./...

gotestloud:
	@go test -race -v ./...

gobump: gotest
	@go run ./internal/cmd/bumper

# call with `make gobench pkg=./kit/mux` (or whatever)
gobench:
	@go test -bench=. $(pkg)

gofix:
	@go fix ./...
	@cd internal/integration_tests && go fix ./...
	@cd internal/apps/docs && go fix ./...

#####################################################################
####### TS
#####################################################################

tstest:
	@cd ./internal/pkg/npm/ && \
		pnpm vitest run --reporter=dot && \
		pnpm tsgo -p ./vorma/tests/tsconfig.json --noEmit

tstestwatch:
	@cd ./internal/pkg/npm/ && pnpm vitest

tsbench:
	@cd ./internal/pkg/npm/ && pnpm vitest bench

tsnuke:
	@rm -rf node_modules 2>/dev/null || true
	@find . -path "*/node_modules" -type d -exec rm -rf {} \; 2>/dev/null || true

tsi:
	@pnpm i
	@cd ./internal/pkg/npm/ && pnpm i
	@cd ./internal/pkg/npm/vorma/create && pnpm i
	@cd ./internal/integration_tests && pnpm i
	@cd ./internal/apps/docs && pnpm i

tsreset: tsi npmbuild

tslint:
	@pnpm oxlint

tsfmt:
	@pnpm oxfmt

tscheck:
	@cd ./internal/pkg/npm/ && pnpm tsgo --noEmit --project ./kit
	@cd ./internal/pkg/npm/ && pnpm tsgo --noEmit --project ./vorma/core
	@cd ./internal/pkg/npm/ && pnpm tsgo --noEmit --project ./vorma/tsx/preact
	@cd ./internal/pkg/npm/ && pnpm tsgo --noEmit --project ./vorma/tsx/react
	@cd ./internal/pkg/npm/ && pnpm tsgo --noEmit --project ./vorma/tsx/solid
	@cd ./internal/pkg/npm/ && pnpm tsgo --noEmit --project ./vorma/vite
	@cd ./internal/pkg/npm/ && pnpm tsgo --noEmit --project ./vorma/create

npmbuild:
	@cd ./internal/pkg/npm/ && pnpm tsdown

npmbump:
	@go run ./internal/cmd/npm_bumper

bombadil:
	@cd ./internal/integration_tests && $(MAKE) test

bombadil-prod:
	@cd ./internal/integration_tests && $(MAKE) prod

bombadil-dev:
	@cd ./internal/integration_tests && $(MAKE) dev

bombadil-build:
	@cd ./internal/integration_tests && $(MAKE) build

# Pass multiplier as a positive integer, e.g. make stress multiplier=2.
stress:
	@go run ./internal/cmd/stress -multiplier $(multiplier)

hegel-stress:
	@go test -race ./kit/matcher ./kit/schema ./kit/searchparams ./kit/tsgen -count=$(multiplier)

#####################################################################
####### OTHER
#####################################################################

docsdev:
	@cd ./internal/apps/docs && pnpm dev

docker-site:
	@docker build -t vorma-site -f Dockerfile.site .

docker-run-site:
	@docker run -d -p $(PORT):$(PORT) -e PORT=$(PORT) vorma-site

run-create: tsreset tsnuke
	@mkdir -p test_create.local && \
		cd test_create.local && \
		node ../internal/pkg/npm/vorma/create/.dist/main.js --local-test
