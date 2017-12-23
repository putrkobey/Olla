PKG := github.com/thushan/olla/internal/version
RUNTIME := Go v$(shell go version | awk '{print $$3}' | sed 's/go//')
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
USER := $(shell git config user.name 2>/dev/null || whoami)
TOOL := "make"

# Tool versions (pinned)
GOLANGCI_LINT_VERSION := v2.11.4
BETTERALIGN_VERSION := v0.8.2
BUN_VERSION := 1.3.5

# Frontend locations. WEB_DIR is the Svelte source tree; EMBED_DIST is the
# go:embed source the binary ships. It is gitignored and regenerated at build
# time by build-web; only a .gitkeep sentinel stays committed so a fresh
# checkout compiles without Bun.
WEB_DIR := web/dashboard
EMBED_DIST := internal/app/handlers/dashboard/dist
# Sentinel proving build-web actually populated the embed (see build-web).
EMBED_INDEX := index.html

LDFLAGS := -ldflags "\
	-X '$(PKG).Version=$(VERSION)' \
	-X '$(PKG).Runtime=$(RUNTIME)' \
	-X '$(PKG).Commit=$(COMMIT)' \
	-X '$(PKG).Date=$(DATE)' \
	-X '$(PKG).Tool=$(TOOL)' \
	-X '$(PKG).User=$(USER)'"

.PHONY: smoke-build-local run clean build test test-verbose test-short test-race test-cover bench version install-deps check-deps vet test-script-integration test-script-sticky test-auth-bearer test-auth-env-fatal test-auth-manual build-web install-web clean-web lint-web test-web ci-web check-fonts-web

# Build the application with version info. Depends on build-web so the binary
# always embeds the current frontend; this is where the "build fresh every
# time" guarantee is enforced for plain go builds.
build: build-web
	@echo "Building olla $(VERSION)..."
	@go build $(LDFLAGS) -o bin/olla .

# Build release version (optimised). build-web first for the same reason.
build-release: build-web
	@echo "Building olla $(VERSION) for release..."
	@mkdir -p bin
	@CGO_ENABLED=0 go build $(LDFLAGS) -a -installsuffix cgo -o bin/olla$(shell go env GOEXE) .

# Platform-specific validation (used by CI - builds both archs for current OS but runs only AMD64)
validate-linux:
	@echo "Building and testing Linux binaries..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/olla-linux-amd64 .
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/olla-linux-arm64 .
	@echo "Testing Linux AMD64 binary..."
	@bin/olla-linux-amd64 --version > /dev/null && echo "Linux AMD64: OK"
	@echo "Testing Linux ARM64 binary..."
	@if bin/olla-linux-arm64 --version > /dev/null 2>&1; then \
		echo "Linux ARM64: OK (tested via QEMU)"; \
	else \
		echo "Linux ARM64: Build OK (runtime test failed/skipped)"; \
	fi
	@rm -f bin/olla-linux-*

validate-darwin:
	@echo "Building and testing macOS binaries..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/olla-darwin-amd64 .
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/olla-darwin-arm64 .
	@echo "Testing macOS binary..."
	@if [ "$$(uname -m)" = "arm64" ]; then \
		bin/olla-darwin-arm64 --version > /dev/null && echo "Darwin ARM64: OK"; \
		echo "AMD64 build: OK (compile-only)"; \
	else \
		bin/olla-darwin-amd64 --version > /dev/null && echo "Darwin AMD64: OK"; \
		echo "ARM64 build: OK (compile-only)"; \
	fi
	@rm -f bin/olla-darwin-*

validate-windows:
	@echo "Building and testing Windows binaries..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/olla-windows-amd64.exe .
	@CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/olla-windows-arm64.exe .
	@echo "Testing Windows AMD64 binary..."
	@bin/olla-windows-amd64.exe --version > /dev/null && echo "Windows AMD64: OK"
	@echo "ARM64 build: OK (compile-only)"
	@rm -f bin/olla-windows-*

# Validate all platforms (for local testing or releases - builds all 6 targets)
#
# Platform Support Matrix:
#   OS       | AMD64 | ARM64 |
#   ---------|-------|-------|
#   Linux    |  ✓    |  ✓    |
#   macOS    |  ✓    |  ✓    |  (ARM64 = Apple Silicon M1/M2/M3)
#   Windows  |  ✓    |  ✓    |  (ARM64 = Windows on ARM devices)
#
# This target builds all combinations but does NOT test execution
# Use the OS-specific targets (validate-linux, etc) to test execution
validate-all-platforms:
	@echo "Building all platforms (6 targets)..."
	@mkdir -p bin
	@echo "Linux AMD64..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/olla-linux-amd64 .
	@echo "Linux ARM64..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/olla-linux-arm64 .
	@echo "macOS AMD64 (Intel)..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/olla-darwin-amd64 .
	@echo "macOS ARM64 (Apple Silicon)..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/olla-darwin-arm64 .
	@echo "Windows AMD64..."
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/olla-windows-amd64.exe .
	@echo "Windows ARM64..."
	@CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/olla-windows-arm64.exe .
	@echo "All platforms built successfully (6 targets)"
	@rm -f bin/olla-*

# Run the application. Depends on build-web so the embedded dashboard is
# regenerated; without it the embed carries only the .gitkeep sentinel and
# serves a 503 not-built response.
run: build-web
	@go run $(LDFLAGS) .

# Run with debug logging
run-debug: build-web
	@OLLA_LOG_LEVEL=debug go run $(LDFLAGS) .

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	@go test -v ./...

# Run tests with short flag
test-short:
	@echo "Running short tests..."
	@go test -short ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	@go test -race -short ./...

# Run tests with race detection and a coverage profile. Same test selection as
# test-race - this only adds -coverprofile so CI can feed coverage.out to
# Codecov without running the suite twice.
test-race-cover:
	@echo "Running tests with race detection and coverage..."
	@go test -race -short -coverprofile=coverage.out ./...

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	@go test -cover ./...

# Run stress tests (comprehensive testing)
test-stress:
	@echo "Running comprehensive stress tests..."
	@go test -v ./... -run "Stress"

# Run tests with coverage profile and generate HTML report
test-cover-html:
	@echo "Running tests with coverage and generating HTML report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmark tests
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Run repository benchmarks
bench-repo:
	@echo "Running repository benchmarks..."
	@go test -bench=BenchmarkRepository -benchmem ./internal/adapter/discovery/

# Run balancer benchmarks
bench-balancer:
	@echo "Running balancer benchmarks..."
	@go test -bench=BenchmarkFactory -benchmem ./internal/adapter/balancer/
	@go test -bench=BenchmarkPriority -benchmem ./internal/adapter/balancer/
	@go test -bench=BenchmarkRoundRobin -benchmem ./internal/adapter/balancer/
	@go test -bench=BenchmarkLeastConnections -benchmem ./internal/adapter/balancer/

# Show version information that would be embedded
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"
	@echo "User:    $(USER)"

# Show embedded version from built binary
version-built: build
	@./bin/olla --version 2>/dev/null || echo "Add --version flag to main.go to see embedded version"

# Clean build artifacts and logs
clean: clean-web
	@rm -rf bin/ build/ dist/ logs/ coverage.out coverage.html

# Download dependencies
deps:
	@go mod download && go mod tidy

ready-tools: fmt vet lint align
	@echo -e "\033[32mCode is clean for tests!\033[0m"

# Make code ready for commit (test, test-race, fmt, vet, lint, align)
ready: test-short test-race fmt vet lint align
	@echo -e "\033[32mCode is ready for commit!\033[0m"

# Build binaries only (no archives) to ./build directory. --output names the binary file:
# the goreleaser before-hooks make build/ a directory, so a bare ./build/ collides with it.
build-local:
	@echo "Building local binaries to ./build/..."
	@goreleaser build --snapshot --clean --single-target --output ./build/olla$(shell go env GOEXE)
	@echo "Binary created: ./build/olla$(shell go env GOEXE)"

# Prove build-local works from a clean and a pre-populated build/ (needs goreleaser and bun).
# Deletes ./build, so it needs OLLA_SMOKE_OK=1 and belongs in a scratch worktree.
smoke-build-local:
	@./scripts/smoke-build-local.sh

# Build full snapshot release (with archives) to ./dist directory
build-snapshot:
	@echo "Building full snapshot release to ./dist/..."
	@goreleaser release --snapshot --clean
	@echo "Release artifacts in ./dist/"

# Deprecated: use build-local or build-snapshot instead
ready-local: build-snapshot

DOCKER_ARCH ?= amd64

# Build Docker image without goreleaser (for local development)
docker-build-local: build-web
	@echo "Building Docker image locally (without goreleaser) for $(DOCKER_ARCH)..."
	@echo "Building olla binary to root..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(DOCKER_ARCH) go build $(LDFLAGS) -o olla .
	@echo "Staging docker-flavoured config (mirrors the goreleaser before-hook)..."
	@bash ./scripts/generate-container-config.sh
	@mkdir -p build
	@cp config/docker.yaml build/docker-config.yaml
	@echo "Building Docker image..."
	@docker build -t ghcr.io/thushan/olla:local-$(DOCKER_ARCH) .
	@echo "Cleaning up binary..."
	@rm -f olla
	@echo "Docker image built: ghcr.io/thushan/olla:local-$(DOCKER_ARCH)"
	@docker images --filter reference='*olla*local*'

# Build Docker image for ARM64 (convenience target)
docker-build-local-arm64:
	@$(MAKE) docker-build-local DOCKER_ARCH=arm64

# Build and test Docker image locally
docker-build:
	@if ! command -v goreleaser > /dev/null 2>&1; then \
		echo "❌ goreleaser not found. Use 'make docker-build-local' instead (no goreleaser required)"; \
		exit 1; \
	fi
	@echo "Building Docker image locally..."
	@goreleaser release --snapshot --clean --skip=publish,announce,sign,sbom
	@echo "Docker images:"
	@docker images | grep olla | head -5

# Run Docker image with local config (defaults to amd64)
docker-run:
	@echo "Running Docker image with local config (amd64)..."
	@docker run --rm -it \
		-p 40114:40114 \
		-v "$(shell pwd)/config/config.local.yaml:/config/config.yaml:ro" \
		-e OLLA_CONFIG_FILE=/config/config.yaml \
		ghcr.io/thushan/olla:local-amd64

# Test full release locally (binaries + docker + archives)
release-test:
	@echo "Testing full release locally..."
	@goreleaser release --snapshot --clean
	@echo "Release artifacts in ./dist/"
	@echo "Docker images:"
	@docker images | grep olla | head -5

# Test goreleaser configuration
goreleaser-check:
	@echo "Checking goreleaser configuration..."
	@goreleaser check

# ── Frontend (dashboard SPA) ──────────────────────────────────────────────────
# The dashboard dist is gitignored and built fresh whenever a binary is
# produced: build, build-release, docker-build-local, run, run-debug and dev
# depend on build-web, and goreleaser runs build-web via its before.hooks
# (covering build-local, build-snapshot and release). test, vet, lint, align
# and ready stay Bun-free: a committed .gitkeep sentinel keeps //go:embed
# non-empty so a fresh checkout compiles, and the embed handler serves a loud
# 503 if a binary was built without the SPA.

# Install frontend deps (bun install). Requires Bun 1.3.5+ (pinned above as BUN_VERSION).
install-web:
	@echo "Installing frontend dependencies (bun install --frozen-lockfile)..."
	@cd $(WEB_DIR) && bun install --frozen-lockfile
	@echo "Frontend dependencies installed."

# Build the SPA. Vite's outDir points straight at the gitignored go:embed
# source (see vite.config.js), so there is no separate rm/cp/touch copy step
# any more. A writeBundle plugin hook in vite.config.js restores the
# .gitkeep sentinel emptyOutDir wipes on every build.
build-web: install-web
	@echo "Building frontend into embed source..."
	@cd $(WEB_DIR) && bun run build
	@# A vite outDir that stops pointing at EMBED_DIST still exits 0, and the
	@# binary would then embed only .gitkeep and serve a 503 dashboard - which
	@# no Go test catches, since they run against the unbuilt sentinel. Fail
	@# the build here rather than ship it.
	@if [ ! -f "$(EMBED_DIST)/$(EMBED_INDEX)" ]; then \
		echo "build-web: $(EMBED_DIST)/$(EMBED_INDEX) missing after 'bun run build' - check vite.config.js outDir; refusing to embed an empty dashboard"; \
		exit 1; \
	fi
	@echo "Embed source regenerated (gitignored). Rebuild the binary to embed it."

# Run svelte-check (type/syntax checks) on the SPA.
lint-web: install-web
	@echo "Running svelte-check..."
	@cd $(WEB_DIR) && bun run check

# Run vitest unit tests on the SPA.
test-web: install-web
	@echo "Running frontend tests..."
