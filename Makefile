.PHONY: all build build-daemon build-web test test-daemon test-web codegen lint clean config-check

# Default target
all: build

# Build both daemon and web presentation client
build: build-daemon build-web

build-daemon:
	@mkdir -p bin
	go build -o bin/sovereign-daemon ./cmd/sovereign-daemon

build-web:
	npm --prefix web run build

# Run unit and integration tests across monorepo
test: test-daemon test-web

test-daemon:
	go test -v ./...

test-web:
	npm --prefix web test

# Run OpenAPI 3.1 code generation
codegen:
	npm --prefix web run codegen

# Validate Docker Compose configuration
config-check:
	docker compose config

# Static analysis and type checking
lint:
	go vet ./...
	npm --prefix web run build

# Clean transient build artifacts
clean:
	rm -rf bin
	rm -rf web/.next
