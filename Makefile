# Print the maintenance CLI command list.
help:
	@go run ./internal/cmd/maint help

#####################################################################
####### JavaScript Dependencies
#####################################################################

# Install JavaScript dependencies in every workspace.
install-js:
	@go run ./internal/cmd/maint install-js

# Remove every node_modules directory after confirmation.
clean-js:
	@go run ./internal/cmd/maint clean-js

#####################################################################
####### TypeScript
#####################################################################

# Format TypeScript source with oxfmt.
fmt-ts:
	@go run ./internal/cmd/maint fmt-ts

# Lint TypeScript source with oxlint.
lint-ts:
	@go run ./internal/cmd/maint lint-ts

# Typecheck all TypeScript package projects.
typecheck-ts:
	@go run ./internal/cmd/maint typecheck-ts

# Typecheck non-framework TypeScript package projects.
typecheck-ts-other:
	@go run ./internal/cmd/maint typecheck-ts-other

# Typecheck framework TypeScript package projects.
typecheck-ts-framework:
	@go run ./internal/cmd/maint typecheck-ts-framework

# Run all TypeScript package tests.
test-ts:
	@go run ./internal/cmd/maint test-ts

# Run non-framework TypeScript package tests.
test-ts-other:
	@go run ./internal/cmd/maint test-ts-other

# Run framework TypeScript package tests.
test-ts-framework:
	@go run ./internal/cmd/maint test-ts-framework

# Build the npm package dist output.
build-ts:
	@go run ./internal/cmd/maint build-ts

#####################################################################
####### Go
#####################################################################

# Run all Go test domains.
test-go:
	@go run ./internal/cmd/maint test-go

# Run non-framework Go test domains.
test-go-other:
	@go run ./internal/cmd/maint test-go-other

# Run the root Go module with ./...
test-root-go:
	@go run ./internal/cmd/maint test-root-go

# Run internal Go package tests.
test-internal-go:
	@go run ./internal/cmd/maint test-internal-go

# Run kit Go package tests.
test-kit:
	@go run ./internal/cmd/maint test-kit

# Run kit/lab Go package tests.
test-lab:
	@go run ./internal/cmd/maint test-lab

# Run docs Go module tests.
test-docs:
	@go run ./internal/cmd/maint test-docs

# Run tests for the framework Go module.
test-go-framework:
	@go run ./internal/cmd/maint test-go-framework

#####################################################################
####### Framework Tests
#####################################################################

# Run framework Go, TypeScript, Vitest, and browser/runtime suites.
test-framework:
	@go run ./internal/cmd/maint test-framework

# Run only the production browser/runtime framework suite.
test-framework-prod:
	@go run ./internal/cmd/maint test-framework-prod

# Run only the development browser/runtime framework suite.
test-framework-dev:
	@go run ./internal/cmd/maint test-framework-dev

# Build the framework fixture without running it.
build-framework-fixture:
	@go run ./internal/cmd/maint build-framework-fixture

# Run one framework development fixture server.
serve-framework-dev:
	@go run ./internal/cmd/maint serve-framework-dev

# Inspect framework artifacts.
inspect-framework-artifacts:
	@go run ./internal/cmd/maint inspect-framework-artifacts

# Remove framework artifacts after confirmation.
clean-framework-artifacts:
	@go run ./internal/cmd/maint clean-framework-artifacts

#####################################################################
####### Stress
#####################################################################

# Repeat the non-framework and framework stress partitions.
# Usage: make stress intensity=10
stress:
	@go run ./internal/cmd/maint stress --intensity $(intensity)

# Repeat the non-framework stress partition.
# Usage: make stress-other intensity=10
stress-other:
	@go run ./internal/cmd/maint stress-other --intensity $(intensity)

# Repeat framework Go, TypeScript, Vitest, and browser/runtime suites.
# Usage: make stress-framework intensity=10
stress-framework:
	@go run ./internal/cmd/maint stress-framework --intensity $(intensity)

#####################################################################
####### Docs
#####################################################################

# Run the docs development server.
docs-dev:
	@go run ./internal/cmd/maint docs-dev

# Build the docs site.
build-docs:
	@go run ./internal/cmd/maint build-docs

#####################################################################
####### Release
#####################################################################

# Run the full repo confidence workflow.
gate:
	@go run ./internal/cmd/maint gate

# Update release versions, run the gate, and print raw npm publish commands.
prepare-release:
	@go run ./internal/cmd/maint prepare-release

# Create and push the Go module tag, then ask the Go proxy for it.
publish-go:
	@go run ./internal/cmd/maint publish-go

#####################################################################
####### Other Maintenance
#####################################################################

# Exercise create-vorma against a local test directory.
create-local-test:
	@go run ./internal/cmd/maint create-local-test

sum:
	@go run ./internal/cmd/sum/
