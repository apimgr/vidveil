# Vidveil Makefile
# Per BASE.md spec: EXACTLY 4 targets - build, release, docker, test
# DO NOT ADD OTHER TARGETS

BINARY_NAME := vidveil
PROJECT_ORG := apimgr
VERSION ?= $(shell cat release.txt 2>/dev/null || echo "0.2.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags="-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GitCommit=$(GIT_COMMIT)'"

# Build targets (per BASE.md: Windows, Linux, BSD, macOS - AMD64, ARM64)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64 openbsd/amd64 openbsd/arm64

.PHONY: build release docker test

# Build all platforms to ./binaries
build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p binaries
	@# Build for current host first
	@echo "  Building host binary..."
	@CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(BINARY_NAME) ./src
	@# Build for all platforms
	@$(foreach platform,$(PLATFORMS),\
		$(eval OS := $(word 1,$(subst /, ,$(platform))))\
		$(eval ARCH := $(word 2,$(subst /, ,$(platform))))\
		$(eval EXT := $(if $(filter windows,$(OS)),.exe,))\
		echo "  Building $(OS)/$(ARCH)..." && \
		CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build $(LDFLAGS) \
			-o binaries/$(BINARY_NAME)-$(OS)-$(ARCH)$(EXT) ./src && \
	) true
	@echo "Build complete. Binaries in ./binaries/"

# Release to GitHub using gh, delete tag if exists first, output to ./releases
release: build
	@echo "Creating release v$(VERSION)..."
	@mkdir -p releases
	@# Copy binaries to releases (strip -musl suffix if present)
	@for f in binaries/$(BINARY_NAME)-*; do \
		name=$$(basename $$f | sed 's/-musl//g'); \
		cp $$f releases/$$name; \
	done
	@# Create source archive (no VCS files)
	@tar --exclude='.git' --exclude='binaries' --exclude='releases' \
		-czf releases/$(BINARY_NAME)-$(VERSION)-source.tar.gz .
	@# Update release.txt
	@echo "$(VERSION)" > release.txt
	@# Delete existing tag/release if exists
	@gh release delete v$(VERSION) --yes 2>/dev/null || true
	@git tag -d v$(VERSION) 2>/dev/null || true
	@git push origin :refs/tags/v$(VERSION) 2>/dev/null || true
	@# Create new tag and release
	@git tag v$(VERSION)
	@git push origin v$(VERSION)
	@gh release create v$(VERSION) releases/* \
		--title "$(BINARY_NAME) v$(VERSION)" \
		--notes "Release v$(VERSION)"
	@echo "Release v$(VERSION) created."

# Docker build and push to ghcr.io using buildx for ARM64/AMD64
docker:
	@echo "Building Docker image for $(BINARY_NAME)..."
	@docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--tag ghcr.io/$(PROJECT_ORG)/$(BINARY_NAME):latest \
		--tag ghcr.io/$(PROJECT_ORG)/$(BINARY_NAME):$(VERSION) \
		--push \
		.
	@echo "Docker image pushed to ghcr.io/$(PROJECT_ORG)/$(BINARY_NAME)"

# Run all tests
test:
	@echo "Running tests..."
	@go test -v -race -cover ./...
	@echo "Tests complete."
