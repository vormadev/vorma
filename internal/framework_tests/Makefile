# Run the production and development browser/runtime framework suites.
test:
	@cd ../.. && go run ./internal/cmd/maint test-framework

# Run only the production framework suite.
test-prod:
	@cd ../.. && go run ./internal/cmd/maint test-framework-prod

# Run only the development framework suite.
test-dev:
	@cd ../.. && go run ./internal/cmd/maint test-framework-dev

#####################################################################
####### Variant Shortcuts
#####################################################################

# Run the production React fixture.
test-prod-react:
	@cd ../.. && go run ./internal/cmd/maint test-framework-prod --variant react

# Run the development React fixture.
test-dev-react:
	@cd ../.. && go run ./internal/cmd/maint test-framework-dev --variant react

# Run the production Preact fixture.
test-prod-preact:
	@cd ../.. && go run ./internal/cmd/maint test-framework-prod --variant preact

# Run the development Preact fixture.
test-dev-preact:
	@cd ../.. && go run ./internal/cmd/maint test-framework-dev --variant preact

# Run the production Solid fixture.
test-prod-solid:
	@cd ../.. && go run ./internal/cmd/maint test-framework-prod --variant solid

# Run the development Solid fixture.
test-dev-solid:
	@cd ../.. && go run ./internal/cmd/maint test-framework-dev --variant solid

#####################################################################
####### Stress
#####################################################################

# Repeat production and development framework suites.
# Usage: make stress intensity=10
stress:
	@cd ../.. && go run ./internal/cmd/maint stress-framework --intensity $(intensity)

# Repeat only the production suite.
# Usage: make stress-prod intensity=10
stress-prod:
	@cd ../.. && go run ./internal/cmd/maint test-framework-prod --intensity $(intensity)

# Repeat only the development suite.
# Usage: make stress-dev intensity=10
stress-dev:
	@cd ../.. && go run ./internal/cmd/maint test-framework-dev --intensity $(intensity)

#####################################################################
####### Artifacts And Fixture
#####################################################################

# Build the framework fixture without running it.
build:
	@cd ../.. && go run ./internal/cmd/maint build-framework-fixture

# Run one development fixture server.
serve-dev:
	@cd ../.. && go run ./internal/cmd/maint serve-framework-dev

# Inspect framework artifacts.
inspect:
	@cd ../.. && go run ./internal/cmd/maint inspect-framework-artifacts

# Remove framework artifacts after confirmation.
clean:
	@cd ../.. && go run ./internal/cmd/maint clean-framework-artifacts
