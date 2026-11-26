# ============================================
# LOCAL DEVELOPMENT ONLY - NOT FOR CI/CD
# CI/CD pipelines MUST use explicit commands
# ============================================

# PROJECT_NAME and PROJECT_ORG: IDEA.md first, then the Git remote, then the project path.
PROJECT_NAME := $(shell value=$$(sed -n 's/^project_name:[[:space:]]*//p' IDEA.md 2>/dev/null | head -n1 | tr -d '[:space:]'); if [ -n "$$value" ]; then printf '%s' "$$value"; else remote=$$(git remote get-url origin 2>/dev/null || true); if [ -n "$$remote" ]; then printf '%s\n' "$$remote" | sed -E 's|.*/([^/]+)(\.git)?$$|\1|'; else basename "$$(pwd)"; fi; fi)
PROJECT_ORG := $(shell value=$$(sed -n 's/^project_org:[[:space:]]*//p' IDEA.md 2>/dev/null | head -n1 | tr -d '[:space:]'); if [ -n "$$value" ]; then printf '%s' "$$value"; else remote=$$(git remote get-url origin 2>/dev/null || true); if [ -n "$$remote" ]; then printf '%s\n' "$$remote" | sed -E 's|.*[:/]([^/:]+)/[^/:]+(\.git)?$$|\1|'; else basename "$$(dirname "$$(pwd)")"; fi; fi)

# Frozen on-disk identity — read from IDEA.md (never re-derived; may differ from
# PROJECT_NAME after a project rename), falls back to PROJECT_NAME on first run
INTERNAL_NAME := $(shell grep -E '^internal_name:[[:space:]]*.+$$' IDEA.md 2>/dev/null | sed -E 's/^internal_name:[[:space:]]*//' | tr -d '[:space:]')
ifeq ($(INTERNAL_NAME),)
INTERNAL_NAME := $(PROJECT_NAME)
endif

# Local version precedence: explicit VERSION environment override > non-empty release.txt > short Git commit id > devel outside Git.
VERSION ?= $(shell value="$$(tr -d '[:space:]' < release.txt 2>/dev/null)"; if [ -n "$$value" ]; then printf '%s' "$$value"; else git rev-parse --short=7 HEAD 2>/dev/null || printf devel; fi)

# Build info — BUILD_EPOCH is the single captured time source (AI.md PART 25)
# Unix build timestamp (seconds, UTC) — embedded via -ldflags; BuildDate is derived from it at process start
BUILD_EPOCH := $(shell date -u +%s)
# Derived from BUILD_EPOCH — used only for Docker OCI labels, never an ldflag
BUILD_DATE  := $(shell date -u -d @$(BUILD_EPOCH) +"%Y-%m-%dT%H:%M:%SZ")
COMMIT_ID   := $(shell git rev-parse --short=7 HEAD 2>/dev/null || echo "N/A")

# Official site URL (OPTIONAL — never guess or assume)
# Sources in priority order: site.txt → OFFICIAL_SITE env → empty
OFFICIAL_SITE := $(shell [ -f site.txt ] && cat site.txt || echo "$${OFFICIAL_SITE:-}")

# Release tag: add 'v' prefix ONLY to numeric semver VERSION, never to text/timestamp versions
RELEASE_TAG := $(shell v="$(VERSION)"; case "$$v" in (v*) echo "$$v" ;; ([0-9]*.[0-9]*.[0-9]*) echo "v$$v" ;; (*) echo "$$v" ;; esac)

# go build flags applied to every binary (AI.md PART 7/8: -trimpath is a go build flag, not a linker flag)
BUILD_FLAGS := -trimpath -buildvcs=false

# Linker flags to embed build info into server binary (AI.md PART 7/25)
# BuildDate is NOT embedded - it is derived from BuildEpoch at process start
LDFLAGS := -s -w \
	-X 'main.Version=$(VERSION)' \
	-X 'main.CommitID=$(COMMIT_ID)' \
	-X 'main.BuildEpoch=$(BUILD_EPOCH)' \
	-X 'main.OfficialSite=$(OFFICIAL_SITE)'

# Linker flags for CLI client binary (AI.md PART 8/25)
# BuildDate is NOT embedded - it is derived from BuildEpoch at process start
CLI_LDFLAGS := -s -w \
	-X 'main.ProjectName=$(PROJECT_NAME)' \
	-X 'main.Version=$(VERSION)' \
	-X 'main.CommitID=$(COMMIT_ID)' \
	-X 'main.BuildEpoch=$(BUILD_EPOCH)' \
	-X 'main.OfficialSite=$(OFFICIAL_SITE)'

# Directories
BINDIR := binaries
RELDIR := releases
BINDIR_IN_CONTAINER := $(if $(filter /%,$(BINDIR)),$(abspath $(BINDIR)),/app/$(BINDIR))
BINDIR_MOUNT := $(if $(filter /%,$(BINDIR)),-v "$(abspath $(BINDIR)):$(abspath $(BINDIR))")

# Go cache directories (bind-mounted from host for persistence across builds)
GO_CACHE ?= $(HOME)/go/pkg/mod
GO_BUILD ?= $(HOME)/.cache/go-build/$(PROJECT_NAME)

# Build targets — all 8 platforms (space-separated for for-loop)
PLATFORMS ?= linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64,freebsd/amd64,freebsd/arm64

# Registry for Docker target
REGISTRY ?= ghcr.io/$(PROJECT_ORG)/$(PROJECT_NAME)

# Resource limits for build containers (AI.md PART 25)
DOCKER_MEM  ?= 4g
DOCKER_CPUS ?= 2
DOCKER_CPU_SHARES ?= 256
DOCKER_TIMEOUT ?= 3600

# Internal: base docker run options without the image.
# GO_DOCKER (below) is the standard single-command form for simple Go commands.
# GO_DOCKER_RUN is used when extra -e or -v flags must appear before the image name
# (e.g. cross-compile with -e GOOS/-e GOARCH, or temp-dir mounts for test/dev).
GO_DOCKER_RUN = timeout -k 30 $(DOCKER_TIMEOUT) docker run --rm --no-healthcheck \
	--name $(PROJECT_NAME)-$$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n') \
	--memory=$(DOCKER_MEM) --cpus=$(DOCKER_CPUS) --cpu-shares=$(DOCKER_CPU_SHARES) \
	-v $(PWD):/app \
	-v $(GO_CACHE):/usr/local/share/go/pkg/mod \
	-v $(GO_BUILD):/usr/local/share/go/cache \
	-w /app \
	-e CGO_ENABLED=0 \
	-e GOFLAGS=-buildvcs=false

# Standard Go Docker command (per AI.md PART 25 — includes image for simple go commands)
GO_DOCKER = $(GO_DOCKER_RUN) casjaysdev/go:latest

.PHONY: build local release release-check docker test dev clean

# =============================================================================
# CLEAN — Remove build artifacts (AI.md PART 25)
# =============================================================================
clean:
	@rm -rf $(BINDIR) $(RELDIR)

# =============================================================================
# BUILD — Compile local server/CLI binaries and all 8 platform pairs via Docker (AI.md PART 25)
# =============================================================================
build: clean
	@mkdir -p $(BINDIR) $(GO_CACHE) $(GO_BUILD)
	@echo "Building version $(VERSION)..."
	@$(GO_DOCKER_RUN) $(BINDIR_MOUNT) -e BUILD_LDFLAGS="$(LDFLAGS)" -e CLI_LDFLAGS="$(CLI_LDFLAGS)" casjaysdev/go:latest sh -c 'set -eu; go mod tidy; go mod download; go generate ./src/swagger; \
		GOOS=$$(go env GOOS) GOARCH=$$(go env GOARCH) go build $(BUILD_FLAGS) -ldflags "$$BUILD_LDFLAGS" -o $(BINDIR_IN_CONTAINER)/$(PROJECT_NAME) ./src; \
		if [ -d src/client ]; then GOOS=$$(go env GOOS) GOARCH=$$(go env GOARCH) go build $(BUILD_FLAGS) -ldflags "$$CLI_LDFLAGS" -o $(BINDIR_IN_CONTAINER)/$(PROJECT_NAME)-cli ./src/client; fi; \
		for platform in $$(echo "$(PLATFORMS)" | tr "," " "); do OS=$${platform%/*}; ARCH=$${platform#*/}; EXT=""; \
			if [ "$$OS" = windows ]; then EXT=.exe; fi; \
			GOOS=$$OS GOARCH=$$ARCH go build $(BUILD_FLAGS) -ldflags "$$BUILD_LDFLAGS" -o $(BINDIR_IN_CONTAINER)/$(PROJECT_NAME)-$$OS-$$ARCH$$EXT ./src; \
			if [ -d src/client ]; then GOOS=$$OS GOARCH=$$ARCH go build $(BUILD_FLAGS) -ldflags "$$CLI_LDFLAGS" -o $(BINDIR_IN_CONTAINER)/$(PROJECT_NAME)-cli-$$OS-$$ARCH$$EXT ./src/client; fi; \
		done'
	@echo "Build complete: $(BINDIR)/"

# =============================================================================
# LOCAL — Build host-platform binaries only (fast local test builds)
# =============================================================================
local: clean
	@mkdir -p $(BINDIR) $(GO_CACHE) $(GO_BUILD)
	@echo "Building local binaries version $(VERSION)..."
	@$(GO_DOCKER_RUN) $(BINDIR_MOUNT) -e BUILD_LDFLAGS="$(LDFLAGS)" -e CLI_LDFLAGS="$(CLI_LDFLAGS)" casjaysdev/go:latest sh -c 'set -eu; go mod tidy; go mod download; go generate ./src/swagger; \
		GOOS=$$(go env GOOS) GOARCH=$$(go env GOARCH) go build $(BUILD_FLAGS) -ldflags "$$BUILD_LDFLAGS" -o $(BINDIR_IN_CONTAINER)/$(PROJECT_NAME) ./src; \
		if [ -d src/client ]; then GOOS=$$(go env GOOS) GOARCH=$$(go env GOARCH) go build $(BUILD_FLAGS) -ldflags "$$CLI_LDFLAGS" -o $(BINDIR_IN_CONTAINER)/$(PROJECT_NAME)-cli ./src/client; fi'
	@echo "Local build complete: $(BINDIR)/"

# =============================================================================
# RELEASE — Manual local release build with source archive
# =============================================================================
release: release-check build
	@mkdir -p $(RELDIR)
	@echo "Preparing release $(VERSION)..."
	@echo "$(VERSION)" > $(RELDIR)/version.txt
	@for f in $(BINDIR)/$(PROJECT_NAME)-*; do \
		[ -f "$$f" ] || continue; \
		strip "$$f" 2>/dev/null || true; \
		cp "$$f" $(RELDIR)/; \
	done
	@tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
		--exclude='binaries' --exclude='releases' --exclude='*.tar.gz' \
		-czf $(RELDIR)/$(PROJECT_NAME)-$(VERSION)-source.tar.gz .
	@echo "Generating checksums..."
	@cd $(RELDIR) && rm -f sha256.txt sha512.txt && \
		sha256sum $$(ls | grep -v -e '^sha256.txt$$' -e '^sha512.txt$$') > sha256.txt && \
		sha512sum $$(ls | grep -v -e '^sha256.txt$$' -e '^sha512.txt$$') > sha512.txt
	@gh release delete $(RELEASE_TAG) --yes 2>/dev/null || true
	@git tag -d $(RELEASE_TAG) 2>/dev/null || true
	@git push origin :refs/tags/$(RELEASE_TAG) 2>/dev/null || true
	@gh release create $(RELEASE_TAG) $(RELDIR)/* \
		--title "$(PROJECT_NAME) $(RELEASE_TAG)" \
		--notes "Release $(RELEASE_TAG)" \
		--latest
	@echo "Release complete: $(RELEASE_TAG)"

# =============================================================================
# DOCKER — Build and push multi-arch container image to registry
# =============================================================================
docker:
	@echo "Building Docker image $(VERSION)..."
	@docker buildx version > /dev/null 2>&1 || (echo "docker buildx required" && exit 1)
	@docker buildx create --name multiarch-builder --use 2>/dev/null || \
		docker buildx use multiarch-builder
	@timeout -k 30 $(DOCKER_TIMEOUT) docker buildx build \
		-f docker/Dockerfile \
		--platform linux/amd64,linux/arm64 \
		--push \
		--build-arg VERSION="$(VERSION)" \
		--build-arg BUILD_EPOCH="$(BUILD_EPOCH)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		--build-arg COMMIT_ID="$(COMMIT_ID)" \
		--build-arg OFFICIAL_SITE="$(OFFICIAL_SITE)" \
		--label "maintainer=$(PROJECT_ORG)" \
		--label "org.opencontainers.image.vendor=$(PROJECT_ORG)" \
		--label "org.opencontainers.image.authors=$(PROJECT_ORG)" \
		--label "org.opencontainers.image.title=$(PROJECT_NAME)" \
		--label "org.opencontainers.image.base.name=$(PROJECT_NAME)" \
		--label "org.opencontainers.image.description=$(PROJECT_NAME) - privacy-respecting meta search engine" \
		--label "org.opencontainers.image.licenses=MIT" \
		--label "org.opencontainers.image.created=$(BUILD_DATE)" \
		--label "org.opencontainers.image.version=$(VERSION)" \
		--label "org.opencontainers.image.schema-version=$(VERSION)" \
		--label "org.opencontainers.image.revision=$(COMMIT_ID)" \
		--label "org.opencontainers.image.url=https://github.com/$(PROJECT_ORG)/$(PROJECT_NAME)" \
		--label "org.opencontainers.image.source=https://github.com/$(PROJECT_ORG)/$(PROJECT_NAME)" \
		--label "org.opencontainers.image.documentation=https://github.com/$(PROJECT_ORG)/$(PROJECT_NAME)" \
		--label "org.opencontainers.image.vcs-type=Git" \
		--label "com.github.containers.toolbox=false" \
		--annotation "maintainer=$(PROJECT_ORG)" \
		--annotation "org.opencontainers.image.title=$(PROJECT_NAME)" \
		--annotation "org.opencontainers.image.vendor=$(PROJECT_ORG)" \
		--annotation "org.opencontainers.image.authors=$(PROJECT_ORG)" \
		--annotation "org.opencontainers.image.base.name=$(PROJECT_NAME)" \
		--annotation "org.opencontainers.image.description=$(PROJECT_NAME) - privacy-respecting meta search engine" \
		--annotation "org.opencontainers.image.schema-version=$(VERSION)" \
		--annotation "org.opencontainers.image.vcs-type=Git" \
		--annotation "com.github.containers.toolbox=false" \
		--annotation "org.opencontainers.image.licenses=MIT" \
		--annotation "org.opencontainers.image.created=$(BUILD_DATE)" \
		--annotation "org.opencontainers.image.version=$(VERSION)" \
		--annotation "org.opencontainers.image.revision=$(COMMIT_ID)" \
		--annotation "org.opencontainers.image.url=https://github.com/$(PROJECT_ORG)/$(PROJECT_NAME)" \
		--annotation "org.opencontainers.image.source=https://github.com/$(PROJECT_ORG)/$(PROJECT_NAME)" \
		--annotation "org.opencontainers.image.documentation=https://github.com/$(PROJECT_ORG)/$(PROJECT_NAME)" \
		-t $(REGISTRY):$(VERSION) \
		-t $(REGISTRY):latest \
		.
	@set -eu; \
		image="$(REGISTRY):$(VERSION)"; \
		suffix=$$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n'); \
		container="$(PROJECT_NAME)-$$suffix"; \
		created=false; \
		cleanup_image_check() { \
			if [ "$$created" = true ] || [ "$$(docker inspect --format '{{ index .Config.Labels "vidveil.image-check" }}' "$$container" 2>/dev/null || true)" = "$$suffix" ]; then \
				docker rm -f "$$container" >/dev/null 2>&1 || true; \
			fi; \
		}; \
		trap cleanup_image_check EXIT; \
		trap 'exit 130' INT; \
		trap 'exit 143' TERM; \
		if docker container inspect "$$container" >/dev/null 2>&1; then echo "ERROR: refusing to replace existing container $$container" >&2; exit 73; fi; \
		timeout -k 30 $(DOCKER_TIMEOUT) docker run --rm --no-healthcheck --name "$(PROJECT_NAME)-$$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')" --memory=$(DOCKER_MEM) --cpus=$(DOCKER_CPUS) --cpu-shares=$(DOCKER_CPU_SHARES) --entrypoint sh "$$image" -c 'for c in bash curl tor i2pd tini; do command -v "$$c" || exit 1; done'; \
		timeout -k 30 $(DOCKER_TIMEOUT) docker run -d --rm --no-healthcheck --name "$$container" --label "vidveil.image-check=$$suffix" --memory=$(DOCKER_MEM) --cpus=$(DOCKER_CPUS) --cpu-shares=$(DOCKER_CPU_SHARES) "$$image" >/dev/null; \
		created=true; \
		wait_seconds=180; \
		outbound=true; \
		if ! timeout -k 5 10 docker exec "$$container" curl -q -fsS --connect-timeout 3 --max-time 5 https://check.torproject.org/ -o /dev/null >/dev/null 2>&1; then outbound=false; fi; \
		tor_status=""; \
		while [ "$$wait_seconds" -gt 0 ]; do \
			health_json=$$(timeout -k 5 15 docker exec "$$container" curl -q -sS --max-time 10 http://localhost/server/healthz 2>/dev/null || true); \
			tor_status=$$(printf '%s\n' "$$health_json" | awk '/"tor"[[:space:]]*:[[:space:]]*[{]/ { inside=1 } inside && /"status"[[:space:]]*:/ { sub(/^.*"status"[[:space:]]*:[[:space:]]*"/, ""); sub(/".*$$/, ""); print; exit } inside && /[}]/ { inside=0 }'); \
			if [ "$$tor_status" = healthy ]; then break; fi; \
			if [ "$$tor_status" = error ]; then echo "ERROR: shipped image reports Tor status error" >&2; exit 1; fi; \
			if [ "$$outbound" = false ] && { [ "$$tor_status" = starting ] || [ "$$tor_status" = unavailable ]; }; then \
				if [ "$${REQUIRE_OVERLAY:-0}" = 1 ]; then echo "ERROR: Tor cannot bootstrap because outbound network access is unavailable" >&2; exit 1; fi; \
				echo "SKIP: no outbound network; shipped image Tor status is $$tor_status" >&2; break; \
			fi; \
			sleep 1; wait_seconds=$$((wait_seconds - 1)); \
		done; \
		if [ "$$tor_status" != healthy ] && [ "$$outbound" = true ]; then echo "ERROR: shipped image Tor status did not become healthy before bootstrap timeout (status: $${tor_status:-missing})" >&2; exit 1; fi
	@echo "Docker build complete: $(REGISTRY):$(VERSION)"

# =============================================================================
# TEST — Run unit tests with coverage enforcement (AI.md PART 25, 29)
# Coverage minimum: 60% (AI.md PART 25 default floor, IDEA.md coverage_minimum).
# Coverage output goes to temp dir — never to the project tree.
# One bounded toolchain container runs checks and coverage together.
# =============================================================================
test:
	@echo "Running tests with coverage..."
	@mkdir -p $(GO_CACHE) $(GO_BUILD)
	@mkdir -p "$${TMPDIR:-/tmp}/$(PROJECT_ORG)" $(GO_CACHE) $(GO_BUILD)
	@COVDIR=$$(mktemp -d "$${TMPDIR:-/tmp}/$(PROJECT_ORG)/$(INTERNAL_NAME)-XXXXXX") && \
	trap 'rm -rf "$$COVDIR"' EXIT && \
	$(GO_DOCKER_RUN) -v "$$COVDIR:/coverage" casjaysdev/go:latest sh -c 'set -eu; go mod download; go generate ./src/swagger; go vet ./...; govulncheck ./...; \
		go test -coverprofile=/coverage/coverage.out ./...; \
		COVERAGE=$$(go tool cover -func=/coverage/coverage.out | awk '\''/^total:/ {gsub("%", "", $$3); print $$3}'\''); \
		awk -v coverage="$$COVERAGE" '\''BEGIN { if (coverage < 60) { printf "coverage %.1f%% is below the 60%% minimum\\n", coverage; exit 1 } }'\''; \
		printf "Coverage: %s%%\\n" "$$COVERAGE"'

# =============================================================================
# DEV — Quick build to a temp dir for rapid iteration (AI.md PART 25)
# Builds linux/amd64 (container native). Output is isolated per run.
# =============================================================================
dev:
	@mkdir -p $(GO_CACHE) $(GO_BUILD)
	@mkdir -p "$${TMPDIR:-/tmp}/$(PROJECT_ORG)" && \
	BUILD_DIR=$$(mktemp -d "$${TMPDIR:-/tmp}/$(PROJECT_ORG)/$(INTERNAL_NAME)-XXXXXX") && \
	echo "Quick dev build to $$BUILD_DIR..." && \
	$(GO_DOCKER_RUN) -v "$$BUILD_DIR:/build" casjaysdev/go:latest sh -c 'set -eu; go mod tidy; go generate ./src/swagger; \
		go build $(BUILD_FLAGS) -o "/build/$(PROJECT_NAME)" ./src; \
		if [ -d src/client ]; then go build $(BUILD_FLAGS) -o "/build/$(PROJECT_NAME)-cli" ./src/client; fi' && \
	echo "Built: $$BUILD_DIR/$(PROJECT_NAME)" && \
	if [ -f "$$BUILD_DIR/$(PROJECT_NAME)-cli" ]; then echo "Built: $$BUILD_DIR/$(PROJECT_NAME)-cli"; fi && \
	echo "Test:  docker run --rm -it --name $(PROJECT_NAME)-test -v $$BUILD_DIR:/app alpine:latest /app/$(PROJECT_NAME) --help"
