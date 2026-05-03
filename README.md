# Asika

Asika ([/əˈsiːkə/](https://ipa-reader.com/?text=əˈsiːkə), pronounced *uh-SEE-kuh*) is a portmanteau of 
**Akira** (明, “bright, intelligent”) and **seeker**. 
Like an intelligent seeker, it scans your repositories, finds pull requests,
detects spam, and applies labels — keeping everything clear and under control.

## Why Asika?

1. **Stop tab-switching** — Check and manage PRs across GitHub, GitLab, and Gitea from one dashboard or chat.
2. **Merge queue that won't merge too early** — Set approval counts and CI requirements; Asika waits until everything's green.
3. **Spam? Handled.** — Auto-detects suspicious PRs by author frequency or title keywords, then closes them with a comment.
4. **Labels on autopilot** — Define rules like `*.go → go` and Asika labels PRs the moment they open.
5. **Chat ops** — Approve, close, or check queue from Telegram or Feishu (Lark) without touching a browser.
6. **Hot reload** — Update label rules or notification channels without restarting the daemon.
7. **Self-update** — Check for new versions and update in-place from GitHub Releases, with SHA256 verification and automatic rollback support.
8. **Stale PR management** — Auto-detect and label stale PRs, close after threshold, and remove labels when activity resumes.
9. **Single binary, zero drama** — No Node.js, no external git, just one Go binary and you're set.

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
