# CI/CD Rules (PART 27)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️
<!-- Rule summary checked against AI.md on 2026-10-31; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never mix local-dev and CI/CD build patterns — CI/CD uses explicit `go build` flags, never Makefile targets
- Never reference local user paths (`~/.local/share/go`) in CI — use `/tmp/` or CI-native caching
- Never depend on local Docker containers for builds outside the `casjaysdev/go:latest` job container
- Never cross-cancel different release refs — only older runs for the exact same branch/tag ref may be auto-canceled
- Never install tools inline in CI jobs (`apk add`, `go install`) — all tooling (incl. `cyclonedx-gomod`) is pre-installed in `casjaysdev/go:latest`
- Never use `default_branch` for secret-scan diff range — determine range explicitly (`github.event.before`/`.sha` on push, PR base/head on pull_request, full history on schedule)
- Never create `build-toolchain.yml` for Go projects — `casjaysdev/go:latest` is externally maintained
- Never hardcode a Go version in any workflow — always `casjaysdev/go:latest` unpinned
- Never use a floating GitHub-hosted runner label such as `ubuntu-latest` when the current AI.md workflow examples specify a fixed image label
- Never pin third-party GitHub Actions to a tag — always pin to a full commit SHA
- Never embed `BuildDate` via ldflags — ldflags embed `main.BuildEpoch` only; `BUILD_DATE` (derived from `BUILD_EPOCH`) is used solely for Docker OCI labels
- Never compute VERSION/COMMIT_ID/BUILD_EPOCH separately per matrix leg in `beta.yml`/`devel.yml` — a dedicated `version` job computes it once and passes it via job outputs
- Never let the standard `docker.yml` build job also produce the `:devel` tag — that is the separate `build-devel` job's job alone

## CRITICAL - ALWAYS DO

- Always include `ci.yml` and `release.yml` on every project (required); `beta.yml`, `devel.yml`, `docker.yml` are project-specific optional
- Always run every CI job inside `container: image: casjaysdev/go:latest`
- Always set `VERSION`, `COMMIT_ID`, `BUILD_EPOCH` (and derived `BUILD_DATE`) explicitly via a "Set build info" step (never as static `env:`)
- Always build all 8 platforms in matrix (linux/darwin/windows/freebsd × amd64/arm64)
- Always use workflow concurrency to auto-cancel older in-progress runs for branch pushes to `main`/`master`/`devel`/`dev`/`beta`
- Always use workflow concurrency per exact tag ref for tag-only release workflows (never cancels a different tag)
- Always pin third-party actions to full commit SHA with a trailing `# vX.Y.Z` comment
- Always run secret scanning (truffleHog), a `workflow-policy` SHA-pin check, `vuln-scan` (govulncheck), and `image-scan` (Trivy, if `docker/Dockerfile` exists) on every public repo as part of `ci.yml`
- Always generate an SBOM (CycloneDX, `cyclonedx-gomod`) and dual `sha256`/`sha512` checksums for every release artifact set (stable/beta/devel)
- Always attach GitHub build-provenance attestation (`actions/attest-build-provenance`) on GitHub `release.yml`/`beta.yml`/`devel.yml` releases — GitHub-only, not available on Gitea/Forgejo

## Key Rules Summary

**Provider → config location:**

| Provider | Config Location | Self-Hosted |
|----------|-----------------|-------------|
| GitHub | `.github/workflows/*.yml` | No |
| Gitea | `.gitea/workflows/*.yml` | Yes |
| Forgejo | `.forgejo/workflows/*.yml` (or `.gitea/workflows/`) | Yes (self-hosted only) |
| GitLab | `.gitlab-ci.yml` (single file, stages) | Yes |
| Jenkins | `Jenkinsfile` | Yes |

**Required workflow files (GitHub/Gitea/Forgejo):**

| File | Trigger | Purpose |
|------|---------|---------|
| `ci.yml` | Push/PR to default branch + weekly cron (`0 6 * * 1`) for security jobs | Build + test + lint + coverage + secret-scan + workflow-policy + vuln-scan + image-scan |
| `release.yml` | Tag push (`v*`, `*.*.*`) | Production releases (SBOM, attestation, dual checksums) |
| `beta.yml` | Push to `beta` branch | Beta releases (optional), dedicated `version` job |
| `devel.yml` | Daily 3am UTC + push to main/master | Rolling devel release (optional), version = short commit hash, dedicated `version` job |
| `docker.yml` | Any push (all branches) + version tags + daily 4am UTC cron | Docker images (optional); `build-standard` (skips schedule) + `build-devel` (runs on schedule too) |

`ci.yml` and `release.yml` are required on every project. `beta.yml`/`devel.yml`/`docker.yml` included only when the project needs them.

**Build info variables** — set in a dedicated step, not static `env:`:
```
VERSION      release.txt > ${GITHUB_REF_NAME#v} (release) > timestamp+suffix (beta/dev) > git rev-parse --short=7 HEAD (devel — the commit being built, never a timestamp)
COMMIT_ID    git rev-parse --short HEAD
BUILD_EPOCH  date -u +%s
BUILD_DATE   derived from BUILD_EPOCH — Docker OCI labels ONLY, never ldflags
OFFICIAL_SITE  from site.txt, else secrets.OFFICIAL_SITE (never guessed)
```
Used in `LDFLAGS="-s -w -X 'main.Version=...' -X 'main.CommitID=...' -X 'main.BuildEpoch=...' -X 'main.OfficialSite=...'"` — `BuildDate` is never an ldflags target.

**Auto-cancel concurrency policy:**

| Workflow type | Group key | Cancel condition |
|----------------|-----------|-------------------|
| Branch push (`beta.yml`, `devel.yml`, `docker.yml`, `ci.yml`) | `${{ github.workflow }}-${{ github.ref }}` | Cancel older run for same ref only |
| `devel.yml`/`docker.yml` scheduled | same group key | `cancel-in-progress` conditional, scoped to `main`/`master`/`devel`/`dev`/`beta` branches |
| Tag release (`release.yml`) | `release-${{ github.ref }}` | Cancel older run for exact same tag only — never cross-tag |

**`ci.yml` jobs:** `lint` (go vet + staticcheck), `test` (coverage, threshold 60%), `build` (needs lint+test). Security jobs `secret-scan` (truffleHog, explicit range-determination step, never `default_branch`), `workflow-policy` (greps all workflow files for `uses:` lines missing a full 40-char SHA pin, fails on any match), `vuln-scan` (govulncheck), `image-scan` (Trivy, conditional on `docker/Dockerfile` existing) — all four also run on the weekly cron; add `if: github.event_name != 'schedule'` to skip non-security jobs on scheduled runs.

**`release.yml`/`beta.yml`/`devel.yml` release steps:** build all 8 server (+CLI if `src/client/` exists) binaries → `version.txt` → source tarball → **SBOM** (`cyclonedx-gomod mod -json -output {name}-{version}-sbom.cdx.json`, run inside `casjaysdev/go:latest`, never installed on the runner) → **dual checksums** (`sha256sum`, `sha512sum`) → release upload. GitHub additionally attaches build-provenance attestation (`actions/attest-build-provenance`, requires `id-token: write` + `attestations: write`); attestation is GitHub-only.

**`beta.yml`/`devel.yml` structure:** a dedicated `version` job computes `VERSION`/`COMMIT_ID`/`BUILD_EPOCH` once and exposes them as job outputs consumed by every build-matrix leg — matrix legs never each compute their own value. `devel.yml`'s version job uses `git rev-parse --short=7 HEAD`, matching the commit-hash rule above.

**Permissions — least privilege, set explicitly per workflow/job:**

| Workflow | `contents` | `packages` | `id-token`/`attestations` |
|----------|-----------|-----------|-----------|
| `ci.yml` | `read` | — | — |
| `release.yml` / `beta.yml` / `devel.yml` release job | `write` | — | `write` (GitHub attestation step) |
| `docker.yml` build jobs | `read` | `write` | — |

**SHA-pinning (mandatory for every third-party action, enforced by `workflow-policy`):**
Pin to full 40-char commit SHA + trailing version comment, e.g. `uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1  # v7.0.1`. Never use a tag (`@v4`) or branch ref. Applies to `actions/checkout`, `actions/upload-artifact`, `actions/download-artifact`, `trufflesecurity/trufflehog`, `aquasecurity/trivy-action`, `actions/attest-build-provenance`, `docker/setup-qemu-action`, `docker/setup-buildx-action`, `docker/login-action`, `docker/build-push-action`, `softprops/action-gh-release`, etc.

**Docker image build (`docker.yml`):**

| Image | Job | Dockerfile | Base | Trigger | Tag policy |
|-------|-----|-----------|------|---------|-----------|
| Standard | `build-standard` (skips `schedule` events) | `docker/Dockerfile` | alpine | every non-tag push, version tags | `{commit_id}` (any push); `beta` (beta branch push); `{version}`, `latest`, `YYMM` (version tag) |
| Development | `build-devel` | `docker/Dockerfile.dev` | alpine | every push + daily 4am UTC cron (keeps `:devel` fresh with no new commits needed) | `:devel` only (single static tag), MODE=devel baked in via Dockerfile.dev `ENV`, debug tooling added |

Registry: `ghcr.io` (GitHub) / auto-detected from server URL (Gitea/Forgejo). Platforms: `linux/amd64,linux/arm64` via buildx. Every image build sets both full OCI `labels:` (image-level) AND `manifest:`-prefixed `annotations:` (manifest-level) — vendor, authors, title, base.name, description, licenses=MIT, version (standard only), created, revision, url, source, documentation — inlined directly in the build step, not via a separate metadata-action.

**Workflow creation order** (from project-rules.md / audit convention): security-only workflows first (secret-scan, workflow-policy, vuln-scan), then `ci.yml`, then `release.yml` last. Every staged workflow file must pass `act --list -W {file}` before commit.

**Devel build cleanup:** `devel.yml` release job deletes the previous `devel` tag/release before creating a new one (`gh release delete devel --yes` + `git push origin :refs/tags/devel`, or Gitea API `DELETE .../releases/tags/devel`).

**Cross-provider variable mapping:**

| GitHub | Gitea | Forgejo | GitLab |
|--------|-------|---------|--------|
| `${{ github.* }}` | `${{ gitea.* }}` | `${{ forgejo.* }}` | `$CI_*` |
| `GITHUB_ENV` | `GITEA_ENV` | `FORGEJO_ENV` | dotenv artifact |
| `GITHUB_REF_NAME`/`github.ref_name` | `GITEA_REF_NAME` | `FORGEJO_REF_NAME` | `$CI_COMMIT_REF_NAME` |
| `github.sha` | `gitea.sha` | `forgejo.sha` | `$CI_COMMIT_SHA` |
| `github.repository` | `gitea.repository` | `forgejo.repository` | `$CI_PROJECT_PATH` |
| `github.server_url` | `gitea.server_url` | `forgejo.server_url` | `$CI_SERVER_URL` |
| `github.token` | `secrets.GITEA_TOKEN` | `secrets.FORGEJO_TOKEN` | `$CI_JOB_TOKEN` |
| `${{ secrets.NAME }}` | same | same | `$NAME` |

**GitLab CI specifics:** single `.gitlab-ci.yml` with `stages: [build, test, package, release, docker]`; use `rules:` (preferred) or `only/except`; `parallel:matrix` in place of `strategy.matrix`; `needs:` for DAG job dependencies; scheduled devel builds and a separate devel-image schedule at `0 4 * * *` are configured in the GitLab UI, not YAML; registry at `$CI_REGISTRY_IMAGE`; built-in container registry, no external `REGISTRY` secret needed.

**Jenkins specifics:** `Jenkinsfile` required in repo root when self-hosted; `agent none` at pipeline level, per-stage `agent { label 'amd64' }` / `label 'arm64'`; global `triggers { cron('TZ=UTC\n0 3 * * *\n0 4 * * *') }` covers both the devel build (3am) and the devel-image rebuild (4am); timer-triggered stages distinguish those runs by UTC start hour so the 4am trigger runs only the devel image; `BUILD_TYPE` env var (`release`/`beta`/`devel`/`dev`) drives which release stage runs, computed from `env.TAG_NAME`/`env.BRANCH_NAME`; devel `VERSION` = `git rev-parse --short=7 HEAD`; builds run via `docker run` against `casjaysdev/go:latest` with `GO_CACHE`/`GO_BUILD` host cache mounts, `CGO_ENABLED=0`, `-buildvcs=false -trimpath`; release stages generate SBOM (`cyclonedx-gomod`, run inside the build image) and dual `sha256`/`sha512` checksums, matching GitHub Actions (no attestation step — GitHub-only feature); separate `Docker` and `Docker: Devel` stages mirror `docker.yml`'s `build-standard`/`build-devel` split; git provider (GitHub/Gitea/Forgejo/GitLab/Docker Hub) selected via one uncommented `environment` block, credentials via named Jenkins Secret text credentials (`github-token`, `gitea-token`, etc.).

For complete details, see AI.md PART 27
