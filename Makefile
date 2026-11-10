.PHONY: all build build-daemon build-web test test-daemon test-web test-scripts test-installer audit-spoke-tags codegen lint clean config-check verify-remotes publish-release

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
test: test-daemon test-web test-scripts

test-daemon:
	go test -v ./...

test-web:
	npm --prefix web test

test-scripts:
	./tests/verify_remotes_test.sh
	./tests/installer_test.sh
	./tests/audit_spoke_tags_test.sh
	./tests/publish_release_test.sh

test-installer:
	./tests/installer_test.sh

# Run Spoke Dependency Governance and Tag Audit Suite
audit-spoke-tags:
	./scripts/audit-spoke-tags.sh

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

# Verify dual-remote synchronization parity
verify-remotes:
	./scripts/verify-remotes.sh

# Publish release tag across dual remotes and trigger GitHub Actions release pipeline
publish-release:
	@if [ -z "$(TAG)" ]; then echo "Usage: make publish-release TAG=v<MAJOR>.<MINOR>.<PATCH>[-rc.<N>]"; exit 1; fi
	./scripts/publish-release.sh $(TAG)

