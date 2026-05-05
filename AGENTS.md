# Asika — Agent Guide

## Project

Go 1.25.0 PR management tool. Two binaries: `asika` (CLI) and `asikad` (daemon). Module path is `asika` (not `github.com/...`).

## Architecture

```
cmd/asika/main.go    → CLI binary (cobra → lib/commands)
cmd/asikad/main.go   → Daemon binary (gin HTTP server → daemon/server/core)

common/              → Shared: config (TOML), db (bbolt), models, auth (JWT),
                       platforms (GitHub/GitLab/Gitea), notifier, events, gitutil
daemon/              → Server-only: handlers, queue, syncer, labeler, polling,
                       platform bots (Telegram/Feishu/Discord), stale, templates
lib/                 → CLI command implementations + output formatter
testutil/            → Shared test helpers (mocks, db setup, auth)
```

Data flow: Platform APIs → Syncer → Event Bus → Consumer → Labeler + Queue → Notifiers.

## Build & Test

```bash
# Download deps
go mod download

# Build both binaries (uses build.sh which auto-generates YYYYMMDD-DEV version)
bash build.sh

# Build manually with version
go build -ldflags="-X 'asika/common/version.Version=v1.0.0'" -o asika ./cmd/asika
go build -ldflags="-X 'asika/common/version.Version=v1.0.0'" -o asikad ./cmd/asikad

# All tests
go test ./common/... ./lib/... ./daemon/...

# Single package
go test ./common/config/...

# Single test
go test ./common/config -run TestLoad

# Clean
bash build.sh clean
```

## Key Conventions

- **No linting tool configured** — there is no golangci-lint or similar; don't add lint steps to workflows.
- **Version format**: `YYYYMMDD` + suffix (`DEV`, `HF`, `CVE`, `DEP`). Set via ldflags at build time into `asika/common/version.Version`.
- **Config format**: TOML, hot-reloaded via fsnotify.
- **Storage**: bbolt (embedded), no external DB required.
- **Web UI templates**: embedded in the `asikad` binary via Go embed (`daemon/templates/`).
- **Pre-built binaries** (`asika`, `asikad`) are committed to repo root — this is intentional.
- **Packaging** (deb/pkg/docker/installer) lives in the separate `AsikaProject/pack` repo, not here.

## Contributing

- Branch from main, open an issue for significant changes, then link it in the PR.
- Commit messages: capitalize first letter, title ≤ 50 chars, body ≤ 200 chars (explain what/why/how).
- If a PR receives "Update ChangeLog.md finally", update `ChangeLog.md` with a concise entry like: `- Add feishu bot support by PR #xxx`
- Only one main branch; releases are linear.
