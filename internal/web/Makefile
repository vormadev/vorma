pre:
	@pnpm i && go mod tidy

dev: pre
	@go run ./cmd/build --dev

build: pre
	@go run ./cmd/build
	@echo "\nbuild complete. now run:\n\tmake run port=9090"

# call with `make run port=whatever`
run:
	@PORT=$(port) ./dist/.wavedist/main

vercel-install-go:
	@curl -L https://go.dev/dl/go1.26.1.linux-amd64.tar.gz | tar -C /tmp -xz

vercel-install: vercel-install-go
	@export PATH=/tmp/go/bin:$$PATH && \
		cd ../.. && make tsinstall
	@pnpm i

outer-npmbuild:
	@export PATH=/tmp/go/bin:$$PATH && \
		cd ../.. && make npmbuild

vercel-build: outer-npmbuild
	@export PATH=/tmp/go/bin:$$PATH && \
		go run ./cmd/build
