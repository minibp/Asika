## Architecture

```
┌─────────────────────────────────────────────────────────┐
│              Asika Daemon (asikad)                     │
│                                                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │  HTTP    │  │  Queue   │  │  Syncer  │          │
│  │  Server  │  │  Manager │  │  (multi) │          │
│  └──────────┘  └──────────┘  └──────────┘          │
│                                                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │ Platform │  │  Event   │  │ Consumer │          │
│  │  Clients │  │   Bus    │  │  +Labeler│          │
│  └──────────┘  └──────────┘  └──────────┘          │
│                                                       │
│  ┌─────────────────────────────────────────────────┐  │
│  │   Notifications: SMTP / Telegram / Feishu      │  │
│  └─────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
         ↑                        ↑
    Webhooks / Polling      CLI (asika)
```

## Development

### Project Structure

```
cmd/
  asika/      → CLI binary (Cobra commands)
  asikad/     → Daemon binary (HTTP server)

common/
  config/     → Config loading, validation, hot/cold reload
  db/         → bbolt database wrapper
  platforms/  → Platform clients (GitHub/GitLab/Gitea)
  models/     → Data structures (PRRecord, QueueItem, Config...)
  events/     → Internal event bus
  auth/       → JWT, password hash
  gitutil/    → Pure Go git operations (no system git)
  ci/         → CI provider auto-detection

daemon/
  server/     → Bootstrap, HTTP server, middleware
  handlers/   → API routes (auth, prs, queue, webhook, feishu)
  queue/      → Merge queue state machine
  syncer/     → Cross-platform code sync + spam detector
  notifier/   → Notification channels (SMTP/Telegram/Feishu...)
  platform/   → Telegram/Feishu bot implementations
  consumer/   → Event consumer (wires events → queue/labeler)
  labeler/    → Label rule engine
  polling/    → Polling mode (alternative to webhooks)
  templates/  → Responsive Web UI (Go html/template)
```

### Running Tests

```bash
# All tests
go test ./common/... ./lib/... ./daemon/...

# Specific package
go test ./common/config/...

# Specific test
go test ./common/config -run TestLoad
```

### Build Commands

```bash
# Build both binaries
go build -o asika ./cmd/asika
go build -o asikad ./cmd/asikad

# With version info
go build -ldflags="-X 'asika/lib/commands.Version=v1.0.0'" -o asika ./cmd/asika

# Clean up
rm -f asika asikad
```
