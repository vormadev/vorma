test: matrix

check: install go-test tscheck matrix

matrix: install
	@$(MAKE) prod-run
	@$(MAKE) dev-run

parallel: matrix

install:
	@pnpm i

go-test:
	@go test ./cmd/bombadil ./cmd/serve ./scenario -count=1

tscheck:
	@pnpm tsc -p tsconfig.json --noEmit

prod: install prod-run

prod-all: prod

prod-react:
	@$(MAKE) prod variant=react

prod-preact:
	@$(MAKE) prod variant=preact

prod-solid:
	@$(MAKE) prod variant=solid

prod-run:
	@go run ./cmd/bombadil run $(if $(multiplier),-multiplier $(multiplier),) $(if $(variant),-variant $(variant),)

dev: install dev-run

dev-all: dev

dev-react:
	@$(MAKE) dev variant=react

dev-preact:
	@$(MAKE) dev variant=preact

dev-solid:
	@$(MAKE) dev variant=solid

dev-run:
	@go run ./cmd/bombadil dev-test $(if $(multiplier),-multiplier $(multiplier),) $(if $(variant),-variant $(variant),)

build: install
	@go run ./cmd/bombadil build

inspect:
	@go run ./cmd/bombadil inspect $(if $(artifact),$(artifact),)

clean-artifacts:
	@rm -rf .bombadil
