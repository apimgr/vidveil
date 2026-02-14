# ============================================
# LOCAL DEVELOPMENT ONLY - NOT FOR CI/CD
# CI/CD pipelines MUST use explicit commands
# ============================================
#
# Vidveil Makefile
# Per AI.md PART 26: 6 targets - build, release, docker, test, dev, clean

# Infer PROJECTNAME and PROJECTORG from git remote or directory path (NEVER hardcode)
PROJECTNAME := $(shell git remote get-url origin 2>/dev/null | sed -E -e 's|.*[/:]||' -e 's|\.git$$||' || basename "$$(pwd)")
PROJECTORG := $(shell git remote get-url origin 2>/dev/null | sed -E 's|.*/([^/]+)/[^/]+(\.git)?$$|\1|' || basename "$$(dirname "$$(pwd)")")

# Convenience aliases for common use
PROJECT := $(PROJECTNAME)
ORG := $(PROJECTORG)

# Version: env var > release.txt > default
VERSION ?= $(shell cat release.txt 2>/dev/null || echo "0.1.0")

# Build info - use TZ env var or system timezone
# Format: "Thu Dec 17, 2025 at 18:19:24 EST"
BUILD_DATE := $(shell date +"%a %b %d, %Y at %H:%M:%S %Z")
COMMIT_ID := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Official site URL for CLI client (empty = users must use --server flag)
OFFICIAL_SITE ?=

# Linker flags to embed build info (server) per AI.md PART 7
LDFLAGS := -s -w \
	-X 'main.Version=$(VERSION)' \
	-X 'main.CommitID=$(COMMIT_ID)' \
	-X 'main.BuildDate=$(BUILD_DATE)' \
	-X 'main.OfficialSite=$(OFFICIAL_SITE)'

# Linker flags for CLI client (AI.md PART 33)
CLI_LDFLAGS := -s -w \
	-X 'main.ProjectName=$(PROJECT)' \
	-X 'main.Version=$(VERSION)' \
	-X 'main.CommitID=$(COMMIT_ID)' \
	-X 'main.BuildDate=$(BUILD_DATE)' \
	-X 'main.OfficialSite=$(OFFICIAL_SITE)'

# Directories
BINDIR := ./binaries
RELDIR := ./releases

# Go cache directories (persistent across builds)
# Per AI.md PART 26: Use unified GODIR structure
GODIR := $(HOME)/.local/share/go
GOCACHE := $(GODIR)/build

# Build targets - Per AI.md PART 26: Linux, macOS (Darwin), Windows, FreeBSD - AMD64, ARM64
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64

# Docker
REGISTRY := ghcr.io/$(ORG)/$(PROJECT)
GO_DOCKER := docker run --rm \
	-v $(PWD):/build \
	-v $(GOCACHE):/root/.cache/go-build \
	-v $(GODIR):/go \
	-w /build \
	-e CGO_ENABLED=0 \
	golang:alpine

.PHONY: build local release docker test dev clean

# =============================================================================
# BUILD - Build all platforms + host binary (via Docker with cached modules)
# Per AI.md PART 36: Build CLI client if src/client exists
# =============================================================================
build:
	@mkdir -p $(BINDIR)
	@echo "Building version $(VERSION)..."
	@mkdir -p $(GOCACHE) $(GODIR)

	# Tidy and download modules (per AI.md PART 26)
	@echo "Tidying and downloading Go modules..."
	@$(GO_DOCKER) go mod tidy
	@$(GO_DOCKER) go mod download

	# Build server for host OS/ARCH
	@echo "Building server host binary..."
	@$(GO_DOCKER) sh -c "GOOS=\$$(go env GOOS) GOARCH=\$$(go env GOARCH) \
		go build -ldflags \"$(LDFLAGS)\" -o $(BINDIR)/$(PROJECT) ./src"

	# Build CLI client if src/client exists (AI.md PART 36)
	@if [ -d "src/client" ]; then \
		echo "Building CLI client host binary..."; \
		$(GO_DOCKER) sh -c "GOOS=\$$(go env GOOS) GOARCH=\$$(go env GOARCH) \
			go build -ldflags \"$(CLI_LDFLAGS)\" -o $(BINDIR)/$(PROJECT)-cli ./src/client"; \
	fi

	# Build server for all platforms
	@for platform in $(PLATFORMS); do \
		OS=$${platform%/*}; \
		ARCH=$${platform#*/}; \
		OUTPUT=$(BINDIR)/$(PROJECT)-$$OS-$$ARCH; \
		[ "$$OS" = "windows" ] && OUTPUT=$$OUTPUT.exe; \
		echo "Building server $$OS/$$ARCH..."; \
		$(GO_DOCKER) sh -c "GOOS=$$OS GOARCH=$$ARCH \
			go build -ldflags \"$(LDFLAGS)\" \
			-o $$OUTPUT ./src" || exit 1; \
	done

	# Build CLI client for all platforms if src/client exists
	@if [ -d "src/client" ]; then \
		for platform in $(PLATFORMS); do \
			OS=$${platform%/*}; \
			ARCH=$${platform#*/}; \
			OUTPUT=$(BINDIR)/$(PROJECT)-cli-$$OS-$$ARCH; \
			[ "$$OS" = "windows" ] && OUTPUT=$$OUTPUT.exe; \
			echo "Building CLI $$OS/$$ARCH..."; \
			$(GO_DOCKER) sh -c "GOOS=$$OS GOARCH=$$ARCH \
				go build -ldflags \"$(CLI_LDFLAGS)\" \
				-o $$OUTPUT ./src/client" || exit 1; \
		done; \
	fi

	@echo "Build complete: $(BINDIR)/"

# =============================================================================
# HOST - Build local binaries only (fast development builds)
# =============================================================================
local:
	@mkdir -p $(BINDIR)
	@echo "Building local binaries version $(VERSION)..."
	@mkdir -p $(GOCACHE) $(GODIR)

	# Tidy and download modules (per AI.md PART 26)
	@echo "Tidying and downloading Go modules..."
	@$(GO_DOCKER) go mod tidy
	@$(GO_DOCKER) go mod download

	# Build server binary
	@echo "Building $(PROJECT)..."
	@$(GO_DOCKER) sh -c "GOOS=\$$(go env GOOS) GOARCH=\$$(go env GOARCH) \
		go build -ldflags \"$(LDFLAGS)\" -o $(BINDIR)/$(PROJECT) ./src"

	# Build CLI binary (if exists)
	@if [ -d "src/client" ]; then \
		echo "Building $(PROJECT)-cli..."; \
		$(GO_DOCKER) sh -c "GOOS=\$$(go env GOOS) GOARCH=\$$(go env GOARCH) \
			go build -ldflags \"$(CLI_LDFLAGS)\" -o $(BINDIR)/$(PROJECT)-cli ./src/client"; \
	fi

	# Build agent binary (if exists)
	@if [ -d "src/agent" ]; then \
		echo "Building $(PROJECT)-agent..."; \
		$(GO_DOCKER) sh -c "GOOS=\$$(go env GOOS) GOARCH=\$$(go env GOARCH) \
			go build -ldflags \"$(LDFLAGS)\" -o $(BINDIR)/$(PROJECT)-agent ./src/agent"; \
	fi

	@echo "Host build complete: $(BINDIR)/"

# =============================================================================
# RELEASE - Manual local release (stable only)
# =============================================================================
release: build
	@mkdir -p $(RELDIR)
	@echo "Preparing release $(VERSION)..."

	# Create version.txt
	@echo "$(VERSION)" > $(RELDIR)/version.txt

	# Copy binaries to releases (strip if needed)
	@for f in $(BINDIR)/$(PROJECT)-*; do \
		[ -f "$$f" ] || continue; \
		strip "$$f" 2>/dev/null || true; \
		cp "$$f" $(RELDIR)/; \
	done

	# Create source archive (exclude VCS and build artifacts)
	@tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
		--exclude='binaries' --exclude='releases' --exclude='*.tar.gz' \
		-czf $(RELDIR)/$(PROJECT)-$(VERSION)-source.tar.gz .

	# Delete existing release/tag if exists
	@gh release delete $(VERSION) --yes 2>/dev/null || true
	@git tag -d $(VERSION) 2>/dev/null || true
	@git push origin :refs/tags/$(VERSION) 2>/dev/null || true

	# Create new release (stable)
	@gh release create $(VERSION) $(RELDIR)/* \
		--title "$(PROJECT) $(VERSION)" \
		--notes "Release $(VERSION)" \
		--latest

	@echo "Release complete: $(VERSION)"

# =============================================================================
# DOCKER - Build and push container to ghcr.io
# =============================================================================
# Uses multi-stage Dockerfile - Go compilation happens inside Docker
# No pre-built binaries needed
docker:
	@echo "Building Docker image $(VERSION)..."

	# Ensure buildx is available
	@docker buildx version > /dev/null 2>&1 || (echo "docker buildx required" && exit 1)

	# Create/use builder
	@docker buildx create --name $(PROJECT)-builder --use 2>/dev/null || \
		docker buildx use $(PROJECT)-builder

	# Build and push multi-arch (multi-stage Dockerfile handles Go compilation)
	@docker buildx build \
		-f ./docker/Dockerfile \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION="$(VERSION)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		--build-arg COMMIT_ID="$(COMMIT_ID)" \
		-t $(REGISTRY):$(VERSION) \
		-t $(REGISTRY):latest \
		--push \
		.

	@echo "Docker push complete: $(REGISTRY):$(VERSION)"

# =============================================================================
# TEST - Run all tests with coverage (via Docker with cached modules)
# Per AI.md PART 29: 100% coverage enforcement
# =============================================================================
test:
	@echo "Running tests with coverage..."
	@mkdir -p $(GOCACHE) $(GODIR)
	@$(GO_DOCKER) go mod tidy
	@$(GO_DOCKER) go mod download
	@$(GO_DOCKER) go test -v -cover ./...
	@echo "Tests complete"

# =============================================================================
# DEV - Quick build for local development/testing (to random temp dir)
# =============================================================================
# Fast: host platform only, no ldflags, random temp dir for isolation
dev:
	@mkdir -p $(GOCACHE) $(GODIR)
	@$(GO_DOCKER) go mod tidy
	@BUILD_DIR=$$(mktemp -d "$${TMPDIR:-/tmp}/$(PROJECTORG).XXXXXX") && \
		echo "Quick dev build..." && \
		$(GO_DOCKER) go build -o $$BUILD_DIR/$(PROJECTNAME) ./src && \
		echo "Built: $$BUILD_DIR/$(PROJECTNAME)" && \
		echo "Test:  docker run --rm -v $$BUILD_DIR:/app alpine:latest /app/$(PROJECTNAME) --help"

# =============================================================================
# CLEAN - Remove build artifacts
# =============================================================================
clean:
	@rm -rf $(BINDIR) $(RELDIR)
