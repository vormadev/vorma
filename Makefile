# Print the enforcer command list.
enforcer-help:
	@go run ./internal/cmd/enforcer help

#####################################################################
####### Install
#####################################################################

install:
	@go run ./internal/cmd/enforcer install

install-fw:
	@go run ./internal/cmd/enforcer install --scope fw

install-matcher:
	@go run ./internal/cmd/enforcer install --scope matcher

install-other:
	@go run ./internal/cmd/enforcer install --scope other

install-go:
	@go run ./internal/cmd/enforcer install --lang go

install-ts:
	@go run ./internal/cmd/enforcer install --lang ts

# Remove every node_modules directory in the repo after confirmation.
clean-js:
	@go run ./internal/cmd/enforcer clean-js

#####################################################################
####### Format
#####################################################################

fmt:
	@go run ./internal/cmd/enforcer fmt

fmt-fw:
	@go run ./internal/cmd/enforcer fmt --scope fw

fmt-matcher:
	@go run ./internal/cmd/enforcer fmt --scope matcher

fmt-other:
	@go run ./internal/cmd/enforcer fmt --scope other

fmt-go:
	@go run ./internal/cmd/enforcer fmt --lang go

fmt-ts:
	@go run ./internal/cmd/enforcer fmt --lang ts

#####################################################################
####### Lint
#####################################################################

lint:
	@go run ./internal/cmd/enforcer lint

lint-fw:
	@go run ./internal/cmd/enforcer lint --scope fw

lint-matcher:
	@go run ./internal/cmd/enforcer lint --scope matcher

lint-other:
	@go run ./internal/cmd/enforcer lint --scope other

lint-go:
	@go run ./internal/cmd/enforcer lint --lang go

lint-ts:
	@go run ./internal/cmd/enforcer lint --lang ts

#####################################################################
####### Fix
#####################################################################

fix:
	@go run ./internal/cmd/enforcer fix

fix-fw:
	@go run ./internal/cmd/enforcer fix --scope fw

fix-matcher:
	@go run ./internal/cmd/enforcer fix --scope matcher

fix-other:
	@go run ./internal/cmd/enforcer fix --scope other

fix-go:
	@go run ./internal/cmd/enforcer fix --lang go

fix-ts:
	@go run ./internal/cmd/enforcer fix --lang ts

#####################################################################
####### Typecheck
#####################################################################

typecheck:
	@go run ./internal/cmd/enforcer typecheck

typecheck-fw:
	@go run ./internal/cmd/enforcer typecheck --scope fw

typecheck-matcher:
	@go run ./internal/cmd/enforcer typecheck --scope matcher

typecheck-other:
	@go run ./internal/cmd/enforcer typecheck --scope other

typecheck-go:
	@go run ./internal/cmd/enforcer typecheck --lang go

typecheck-ts:
	@go run ./internal/cmd/enforcer typecheck --lang ts

#####################################################################
####### Test
#####################################################################

test:
	@go run ./internal/cmd/enforcer test

test-fw:
	@go run ./internal/cmd/enforcer test --scope fw

test-matcher:
	@go run ./internal/cmd/enforcer test --scope matcher

test-other:
	@go run ./internal/cmd/enforcer test --scope other

test-go:
	@go run ./internal/cmd/enforcer test --lang go

test-ts:
	@go run ./internal/cmd/enforcer test --lang ts

#####################################################################
####### Build
#####################################################################

build:
	@go run ./internal/cmd/enforcer build

build-go:
	@go run ./internal/cmd/enforcer build --lang go

build-ts:
	@go run ./internal/cmd/enforcer build --lang ts

#####################################################################
####### Gate
#####################################################################

gate:
	@go run ./internal/cmd/enforcer gate

gate-fw:
	@go run ./internal/cmd/enforcer gate --scope fw

gate-matcher:
	@go run ./internal/cmd/enforcer gate --scope matcher

gate-other:
	@go run ./internal/cmd/enforcer gate --scope other

#####################################################################
####### Stress
#####################################################################

# Usage: make stress intensity=10
stress:
	@go run ./internal/cmd/enforcer stress --intensity $(intensity)

# Usage: make stress-fw intensity=10
stress-fw:
	@go run ./internal/cmd/enforcer stress --scope fw --intensity $(intensity)

# Usage: make stress-matcher intensity=10
stress-matcher:
	@go run ./internal/cmd/enforcer stress --scope matcher --intensity $(intensity)

# Usage: make stress-other intensity=10
stress-other:
	@go run ./internal/cmd/enforcer stress --scope other --intensity $(intensity)

# Usage: make stress-go intensity=10
stress-go:
	@go run ./internal/cmd/enforcer stress --lang go --intensity $(intensity)

# Usage: make stress-ts intensity=10
stress-ts:
	@go run ./internal/cmd/enforcer stress --lang ts --intensity $(intensity)

#####################################################################
####### Docs
#####################################################################

docs-dev:
	@cd ./internal/apps/docs && pnpm dev

docs-build:
	@cd ./internal/apps/docs && pnpm build

#####################################################################
####### Release
#####################################################################

prepare-release:
	@go run ./internal/cmd/release prepare

publish-go:
	@go run ./internal/cmd/release publish-go

#####################################################################
####### Bench
#####################################################################

# Usage: make bench-go pkg=./kit/k9
bench-go:
	go test -bench=. $(pkg)

bench-matcher-ts:
	cd ./internal/pkg/npm && pnpm vitest bench ../../matcher_tests/matcher.bench.ts

#####################################################################
####### Other
#####################################################################

# Exercise create-vorma against a local test directory.
run-create:
	@go run ./internal/cmd/enforcer install --lang ts
	@go run ./internal/cmd/enforcer build --lang ts
	@go run ./internal/cmd/enforcer --yes clean-js
	@mkdir -p test_create.local && \
		cd test_create.local && \
		node ../internal/pkg/npm/vorma/create/.dist/main.js --local-test

sum:
	@go run ./internal/cmd/sum/
