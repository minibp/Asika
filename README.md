# Asika

Asika ([/əˈsiːkə/](https://ipa-reader.com/?text=əˈsiːkə), pronounced *uh-SEE-kuh*) is a portmanteau of 
**Akira** (明, “bright, intelligent”) and **seeker**. 
Like an intelligent seeker, it scans your repositories, finds pull requests,
detects spam, and applies labels — keeping everything clear and under control.

## Why Asika?

Managing pull requests across multiple platforms is messy.

You switch between GitHub, GitLab, or Gitea, keep dozens of tabs open, and still risk merging too early or missing important changes.

**Asika fixes this by giving you a single control plane to manage, automate, and safely merge PRs — without leaving your workflow.**

### 🔹 One place for everything  
No more tab-switching. See and manage PRs across platforms from one dashboard or chat.

### 🔹 Safe merges, not early merges  
Built-in merge queue ensures PRs are only merged when approvals and CI checks are complete.

### 🔹 Automate the boring parts  
Labels, stale PRs, and repetitive actions are handled automatically.

### 🔹 Works where you already are  
Approve, close, or check PRs directly from chat via Telegram or Feishu (Lark).

### 🔹 Simple to run  
Single binary. No Node.js. No external dependencies.

## Quick Start

### 1. Get a binary

Download from [releases](https://github.com/minibp/asika/releases) or build it:

```bash
git clone https://github.com/minibp/asika.git
cd asika
bash build.sh
```

### 2. Configure

First time? Run the wizard:

```bash
./asika wizard
```

Or fire up the daemon and use the web wizard at `http://localhost:8080`:

```bash
sudo ./asikad
```

Minimal config (`/etc/asika_config.toml`):

```toml
[server]
listen = ":8080"

[tokens]
github = "ghp_xxx"

[[repo_groups]]
name   = "my-project"
github = "org/repo"
```

See `asika.toml.example` for the full reference — it covers notifications, spam detection, label rules, and more.

### 3. Start managing

```bash
# CLI
./asika pr list my-project

# Or open the dashboard
# http://localhost:8080
```

## Chat Bots

### Telegram

Start a chat with your bot:

```
/prs my-project        → List PRs
/pr my-project 42      → Show PR #42
/approve my-project 42 → Approve
/close my-project 42   → Close
/reopen my-project 42  → Reopen
/spam my-project 42    → Mark as spam
/queue my-project      → Check merge queue
/recheck my-project    → Trigger recheck
/config                → Show config summary
/help                  → All commands
```

### Feishu (Lark)

Send messages directly to the bot:

```
prs my-project    → List PRs
pr my-project 42  → Show PR #42
approve my-project 42 → Approve
close my-project 42   → Close
spam my-project 42    → Mark as spam
queue my-project      → Check queue
recheck my-project    → Trigger recheck
config                → Show config
help                  → All commands
```

## New Features (v20260504DEV)

### 🔹 PR Comments
Comment on PRs directly via CLI or API:
```bash
asika pr comment my-project 42 "LGTM! Ready to merge."
```
API: `POST /api/v1/repos/:repo_group/prs/:pr_id/comment`

### 🔹 Draft PR Detection
Draft PRs (GitHub) and WIP PRs (GitLab) are automatically detected and skipped in the merge queue. Filter by draft status:
```bash
asika pr list my-project --state open  # Add ?is_draft=true/false in API
```

### 🔹 Conflict Detection
PRs with merge conflicts are automatically detected and skipped in the merge queue.

### 🔹 Batch Operations
Manage multiple PRs at once:
```bash
asika pr batch-approve my-project 42,43,44
asika pr batch-close my-project 42,43 --label "wontfix"
```

### 🔹 Enhanced Search & Filtering
List PRs with advanced filters:
```bash
# Filter by author, label, date
asika pr list my-project --author "username" --label "bug"

# API supports: ?author=, ?label=, ?created_after=, ?updated_after=
# Pagination: ?page=1&per_page=20
```

### 🔹 Audit Logging
Track all PR operations with built-in audit logs:
```bash
# API: GET /api/v1/logs?level=info|warn|error
```

### 🔹 Webhook Retry
Failed webhooks are automatically retried with exponential backoff (max 10 attempts).

---

## CLI Cheatsheet
All commands need a token: `asika --token <token>` or set `ASIKA_TOKEN`.

```bash
# PR operations
asika pr list [group]          # --state open|closed|merged, --platform github|gitlab
asika pr show [group] [id]     # PR details
asika pr approve [group] [id]  # Approve PR
asika pr close [group] [id]    # Close PR
asika pr reopen [group] [id]   # Reopen PR
asika pr spam [group] [id]     # Mark/unmark spam (--undo)
asika pr comment [group] [id] [body]  # Comment on PR

# Batch operations
asika pr batch-approve [group] [id1,id2,...]  # Batch approve PRs
asika pr batch-close [group] [id1,id2,...]    # Batch close PRs
asika pr batch-label [group] [id1,id2,...] --label <name> [--color <hex>]  # Batch add label

# Merge queue
asika queue list [group]       # Show queue
asika queue recheck [group]     # Trigger recheck

# Sync (multi mode)
asika sync history              # Show sync history
asika sync retry [sync_id]     # Retry failed sync

# Self-update
asika self-update              # Update to latest version
asika self-update --check     # Check for updates
asika self-update --rollback  # Rollback to previous version
asika self-update --dry-run   # Preview without making changes

# Stale PR management
asika stale check [group]      # Check for stale PRs (--dry-run)
asika stale unmark [group] [id] # Remove stale label

# Config
asika config show              # Show current config (secrets masked)
asika config reload            # Hot reload config
```

## Configuration Highlights

### Repo Groups

**Multi Mode** — Sync PRs across platforms:

```toml
mode = "multi"  # default

[[repo_groups]]
name           = "my-project"
github         = "org/repo"
gitlab         = "org/repo"
gitea          = "org/repo"
default_branch = "main"
```

**Single Mode** — One platform only:

```toml
mode = "single"

[single_repo]
platform       = "github"
repo           = "org/repo"
default_branch = "main"
```

### Label Rules

Auto-label by file patterns (glob or regex):

```toml
[[label_rules]]
pattern = "**/*.go"
label   = "go"

[[label_rules]]
pattern = "docs/**"
label   = "documentation"

[[label_rules]]
pattern = "regex:^.*test.*$"
label   = "has-tests"
```

Rules are hot-reloadable — edit and they apply without restart.

### Spam Detection

Catch bad PRs automatically:

```toml
[spam]
enabled  = true
threshold = 3           # max PRs per time window
time_window = "10m"      # lookback window
trigger_on_author = true
trigger_on_similar_title = true
title_similarity_threshold = 0.6
trigger_on_title_kw = ["spam", "fix typo", "readme update"]
```

### Notifications

Get alerts where you work:

```toml
# Email
[[notify]]
type   = "smtp"
config = { host = "smtp.example.com", port = 587, username = "bot@example.com",
           password = "xxx", to = ["team@example.com"] }

# WeCom
[[notify]]
type   = "wecom"
config = { webhook_url = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx" }

# GitHub @mentions
[[notify]]
type   = "github_at"
config = { owner = "org", repo = "repo", to = ["admin1", "admin2"] }

# Telegram
[[notify]]
type   = "telegram"
config = { token = "bot-token", to = ["@channel", "123456789"] }

# Feishu/Lark
[[notify]]
type   = "feishu"
config = { webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/xxx",
           app_id = "cli_xxx", app_secret = "xxx" }
```

## Contributing

We'd love your help! For first contributing, see [contributing guide](./CONTRIBUTING.md) first.

## License

BSD 3-Clause — see [LICENSE.md](LICENSE.md) for details.

## Issues?

Found a bug? Want a feature? [Open an issue](https://github.com/minibp/asika/issues).

For detailed technical docs, see `PROJECT.md` (for developers) and `asika.toml.example` (for configuration).
