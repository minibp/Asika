Asika 最终设计文档

版本：1.0
目标读者：代码生成大模型（LLM）
指令：请严格按本文档实现，不得添加任何未提及的功能，不得遗漏任何已定义的行为。

---

目录

1. 技术栈与依赖
2. 项目结构
3. 配置文件规范
4. 数据模型
5. 两种工作模式
6. 核心模块设计
7. API 设计
8. CLI 命令设计
9. 工作流详解
10. 安全设计
11. 冷/热更新策略
12. 错误处理与日志
13. 部署与服务文件
14. 初始化向导
15. 测试策略
16. 行为边界与约束
17. 常见歧义澄清

---

1. 技术栈与依赖

语言：Go 1.21+

依赖（go.mod）：

```
module asika

go 1.21.0

require (
    github.com/BurntSushi/toml v1.6.0
    github.com/gin-gonic/gin v1.12.0
    github.com/go-co-op/gocron/v2 v2.16.0
    github.com/go-git/go-git/v5 v5.18.0
    github.com/golang-jwt/jwt/v5 v5.3.1
    github.com/google/go-github/v69 v69.0.0
    github.com/google/uuid v1.6.0
    github.com/spf13/cobra v1.9.0
    github.com/wneessen/go-mail v0.7.2
    go.etcd.io/bbolt v1.4.3
    golang.org/x/crypto v0.45.0
    gopkg.in/natefinch/lumberjack.v2 v2.2.1
    gopkg.in/yaml.v3 v3.0.1
    code.gitea.io/sdk/gitea v0.13.2
    gitlab.com/gitlab-org/api/client-go v0.115.0
)
```

内嵌资源：//go:embed 嵌入 WebUI 模板与静态资源。

WebUI 渲染：使用 Go 标准库 html/template，不依赖前端构建工具链。CSS 使用内联样式或轻量框架（如 Pico.css 的 CDN 内嵌版本），JavaScript 使用 Alpine.js（通过 CDN 或内嵌）。

---

2. 项目结构

```
asika/
├── cmd/
│   ├── asika/
│   │   └── main.go              # package main, CLI 管理工具入口
│   └── asikad/
│       └── main.go              # package main, 后台守护进程入口
│
├── common/                      # asika 和 asikad 共有
│   ├── config/
│   │   ├── config.go            # 配置结构体定义、TOML 加载
│   │   ├── watcher.go           # 热更新监听（fsnotify + API 触发）
│   │   └── wizard.go            # 初始化向导（CLI 交互 + Web 表单共用逻辑）
│   ├── db/
│   │   ├── db.go                # bbolt 初始化、事务封装
│   │   ├── buckets.go           # bucket 名称常量
│   │   └── migrations.go        # 数据库迁移
│   ├── models/
│   │   └── models.go            # 所有数据结构定义
│   ├── platforms/
│   │   ├── interface.go         # 平台客户端接口定义
│   │   ├── github.go            # GitHub 客户端实现
│   │   ├── gitlab.go            # GitLab 客户端实现
│   │   ├── gitea.go             # Gitea 客户端实现
│   │   └── merge_checker.go     # 启动时合并方式检查（exit 1 逻辑）
│   ├── auth/
│   │   ├── auth.go              # JWT 生成/校验、密码哈希
│   │   └── middleware.go        # gin 中间件
│   ├── gitutil/
│   │   └── git.go               # 纯 go-git 操作（cherry-pick, clone, push）
│   ├── ci/
│   │   └── detector.go          # 通过仓库文件检测 CI 配置
│   └── utils/
│       ├── utils.go             # 通用工具函数
│       └── retry.go             # 重试逻辑
│
├── lib/                         # CLI (asika) 特有
│   ├── commands/
│   │   ├── root.go              # cobra root command
│   │   ├── pr.go                # pr 子命令
│   │   ├── queue.go             # queue 子命令
│   │   ├── sync.go              # sync 子命令
│   │   ├── config.go            # config 子命令
│   │   └── wizard.go            # init 向导命令
│   └── formatter/
│       └── output.go            # CLI 输出格式化（table/json/yaml）
│
├── daemon/                      # asikad 特有
│   ├── server/
│   │   ├── server.go            # gin 服务器启动
│   │   ├── router.go            # 路由注册
│   │   └── middleware.go        # 日志、恢复、认证中间件
│   ├── handlers/
│   │   ├── auth.go              # 认证 API handler
│   │   ├── prs.go               # PR 管理 API handler
│   │   ├── queue.go             # 合并队列 API handler
│   │   ├── rules.go             # 标签规则 API handler
│   │   ├── sync.go              # 同步历史 API handler
│   │   ├── config.go            # 配置信息 API handler
│   │   └── wizard.go            # Web 初始化向导 handler
│   ├── queue/
│   │   ├── manager.go           # 合并队列管理器
│   │   └── checker.go           # 条件检查器
│   ├── syncer/
│   │   ├── syncer.go            # 同步引擎（仅多仓库模式）
│   │   └── spam.go              # Spam 检测与处理
│   ├── hooks/
│   │   └── runner.go            # Git hooks 执行器
│   ├── notifier/
│   │   ├── interface.go         # 通知接口
│   │   ├── smtp.go              # 邮件通知
│   │   ├── wecom.go             # 企业微信通知
│   │   └── platform.go          # 平台 @ 通知
│   ├── service/
│   │   ├── asikad.service       # systemd unit 文件
│   │   └── asikad.openrc        # openrc init 脚本
│   └── templates/
│       ├── index.html           # WebUI 主页模板
│       ├── login.html           # 登录页模板
│       ├── wizard.html          # 初始化向导模板
│       ├── dashboard.html       # 仪表板模板
│       ├── pr_detail.html       # PR 详情模板
│       ├── queue.html           # 合并队列模板
│       └── layout.html          # 基础布局模板
│
├── testutil/                    # 测试夹具
│   ├── db.go                    # 临时 bbolt 数据库
│   ├── mocks.go                 # Mock 平台客户端
│   └── auth.go                  # 测试 JWT 生成
│
├── go.mod
├── go.sum
└── asika.toml.example           # 示例配置文件
```

包导入规则：

· cmd/asika/main.go 导入 common + lib
· cmd/asikad/main.go 导入 common + daemon
· common 不导入 lib 或 daemon
· lib 导入 common，不导入 daemon
· daemon 导入 common，不导入 lib
· testutil 导入 common，供所有测试文件使用

---

3. 配置文件规范

路径：/etc/asika_config.toml（默认，可通过 ASIKA_CONFIG 环境变量覆盖）

完整示例：

```toml
# ============================
# 服务器基础配置
# ============================
[server]
listen = ":8080"
mode   = "release"           # "debug" | "release"

# ============================
# 工作模式：single | multi
# ============================
mode = "multi"               # "single" 为单仓库模式, "multi" 为多平台互通模式

# ============================
# 数据库
# ============================
[database]
path = "/var/lib/asika/asika.db"

# ============================
# 认证
# ============================
[auth]
jwt_secret = "your-secret-key-change-me"
token_expiry = "72h"

# ============================
# 通知（数组，可选，支持多个）
# ============================
[[notify]]
type = "smtp"
[notify.config]
host     = "smtp.example.com"
port     = 587
username = "asika@example.com"
password = "your-password"
to       = ["admin@example.com"]

# [[notify]]
# type = "github_at"
# [notify.config]
# to = ["admin-user"]

# [[notify]]
# type = "wecom"
# [notify.config]
# webhook_url = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"

# ============================
# 事件源模式
# ============================
[events]
mode = "webhook"             # "webhook" | "polling"
webhook_secret = "shared-secret"
# polling_interval = "30s"   # 仅在 polling 模式使用

# ============================
# Git 工作目录
# ============================
[git]
workdir = "/var/lib/asika/work"

# ============================
# 平台访问令牌
# ============================
[tokens]
github = "ghp_xxxxxxxxxxxx"
gitlab = "glpat-xxxxxxxxxxxx"
gitea  = "xxxxxxxxxxxxxxxx"

# ============================
# 标签规则（热更新）
# ============================
[[label_rules]]
pattern = "**/*.go"
label   = "go"

[[label_rules]]
pattern = "**/*.js"
label   = "javascript"

# ============================
# Spam 检测
# ============================
[spam]
enabled = true
time_window = "10m"
threshold   = 3
trigger_on_author   = true
trigger_on_similar_title = true
title_similarity_threshold = 0.8
trigger_on_keywords = ["spam", "hack", "crypto"]

# ============================
# 合并队列默认设置
# ============================
[merge_queue]
required_approvals = 1
core_contributors = ["admin", "lead-dev"]

# ============================
# 全局 hooks 路径（仓库组未设置时使用）
# ============================
hookpath = ""

# ============================
# 仓库组（仅 multi 模式使用）
# ============================
[[repo_groups]]
name = "main-project"
github = "owner/repo"
gitlab = "group/repo"
gitea  = "owner/repo"
default_branch = "main"
hookpath = "/etc/asika/hooks/main-project"
ci_provider = "github_actions"   # "github_actions" | "gitlab_ci" | "gitea_actions" | "none"
[repo_groups.merge_queue]         # 可覆盖全局设置
required_approvals = 2
core_contributors = ["lead-dev"]

[[repo_groups]]
name = "docs"
github = "owner/docs"
gitlab = ""
gitea  = ""
default_branch = "gh-pages"
hookpath = ""
ci_provider = "none"

# ============================
# 单仓库配置（仅 single 模式使用）
# ============================
[single_repo]
platform = "github"          # "github" | "gitlab" | "gitea"
repo     = "owner/repo"
default_branch = "main"
hookpath = ""
ci_provider = "github_actions"
```

配置结构体（common/config/config.go）：

```go
type Config struct {
    Server      ServerConfig      `toml:"server"`
    Mode        string            `toml:"mode"` // "single" | "multi"
    Database    DatabaseConfig    `toml:"database"`
    Auth        AuthConfig        `toml:"auth"`
    Notify      []NotifyConfig    `toml:"notify"`
    Events      EventsConfig      `toml:"events"`
    Git         GitConfig         `toml:"git"`
    Tokens      TokensConfig      `toml:"tokens"`
    LabelRules  []LabelRule       `toml:"label_rules"`
    Spam        SpamConfig        `toml:"spam"`
    MergeQueue  MergeQueueConfig  `toml:"merge_queue"`
    HookPath    string            `toml:"hookpath"`
    RepoGroups  []RepoGroupConfig `toml:"repo_groups"`
    SingleRepo  SingleRepoConfig  `toml:"single_repo"`
}

// ... 各子结构体定义
```

---

4. 数据模型

BoltDB Bucket 定义（common/db/buckets.go）：

Bucket 名称 键格式 值序列化 说明
config 字符串键（"label_rules" 等） JSON 可热更新的配置片段
repos 仓库组名称 JSON 仓库组元信息
prs <repo_group>#<platform>#<number> JSON PR 记录
logs <unix_nano>_<uuid_short> JSON 审计日志
queue_items <repo_group>#<pr_id> JSON 合并队列项
users 用户名 JSON 管理用户
sync_history <repo_group>#<timestamp>_<uuid> JSON 同步历史记录

核心数据结构（common/models/models.go）：

```go
// User 管理用户
type User struct {
    Username     string    `json:"username"`
    PasswordHash string    `json:"password_hash"` // bcrypt
    Role         string    `json:"role"`          // "admin" | "operator" | "viewer"
    CreatedAt    time.Time `json:"created_at"`
}

// RepoGroup 仓库组
type RepoGroup struct {
    Name          string          `json:"name"`
    GitHub        string          `json:"github"`
    GitLab        string          `json:"gitlab"`
    Gitea         string          `json:"gitea"`
    DefaultBranch string          `json:"default_branch"`
    HookPath      string          `json:"hookpath"`
    CIProvider    string          `json:"ci_provider"`
    MergeQueue    MergeQueueConfig `json:"merge_queue"`
}

// PRRecord PR 记录
type PRRecord struct {
    ID              string    `json:"id"` // UUID
    RepoGroup       string    `json:"repo_group"`
    Platform        string    `json:"platform"` // "github"|"gitlab"|"gitea"
    PRNumber        int       `json:"pr_number"`
    Title           string    `json:"title"`
    Author          string    `json:"author"`
    State           string    `json:"state"` // "open"|"closed"|"merged"|"spam"
    Labels          []string  `json:"labels"`
    MergeCommitSHA  string    `json:"merge_commit_sha"`
    SpamFlag        bool      `json:"spam_flag"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
    Events          []PREvent `json:"events"`
}

// PREvent PR 事件
type PREvent struct {
    Timestamp time.Time `json:"timestamp"`
    Action    string    `json:"action"` // "opened"|"closed"|"merged"|"approved"|"label_added"|"synced"|"comment"|...
    Actor     string    `json:"actor"`
    Detail    string    `json:"detail"`
}

// QueueItem 合并队列项
type QueueItem struct {
    PRID          string    `json:"pr_id"`
    RepoGroup     string    `json:"repo_group"`
    Status        string    `json:"status"` // "waiting"|"checking"|"merging"|"done"|"failed"
    AddedAt       time.Time `json:"added_at"`
    LastChecked   time.Time `json:"last_checked"`
    FailureReason string    `json:"failure_reason,omitempty"`
}

// AuditLog 审计日志
type AuditLog struct {
    Timestamp time.Time              `json:"timestamp"`
    Level     string                 `json:"level"` // "info"|"warn"|"error"
    Message   string                 `json:"message"`
    Context   map[string]interface{} `json:"context,omitempty"`
}

// SyncRecord 同步历史
type SyncRecord struct {
    ID           string    `json:"id"`
    RepoGroup    string    `json:"repo_group"`
    SourcePlatform string  `json:"source_platform"`
    TargetPlatform string  `json:"target_platform"`
    Branch       string    `json:"branch"`
    CommitSHA    string    `json:"commit_sha"`
    Status       string    `json:"status"` // "success"|"failed"
    ErrorMessage string    `json:"error_message,omitempty"`
    Timestamp    time.Time `json:"timestamp"`
}
```

---

5. 两种工作模式

5.1 多平台互通模式（mode = "multi"）

行为：

· 监听所有配置的仓库组中所有平台的事件
· 当某平台 PR 被合并，自动 cherry‑pick 到其他平台
· 分支删除同步到其他平台
· 完整的跨平台代码互通功能

必需配置：[[repo_groups]] 数组，每组至少填写两个平台

5.2 单仓库模式（mode = "single"）

行为：

· 只监听 [single_repo] 指定平台和仓库
· 完全不执行跨平台代码同步
· 保留以下功能：
  · PR 审计（记录所有事件）
  · 标签规则引擎（自动打标签）
  · 通用合并队列（Bot 自动批准/合并）
  · Spam 检测与处理
  · 通知（邮件/IM/平台@）
  · Webhook 或轮询监听
· Spam 模式中的 cherry‑pick 仍使用本地 Git 操作：当管理员 reopen 后，将 PR 提交 cherry‑pick 到本仓库目标分支
· 仓库管理员需自行手动处理跨平台同步（Asika 不参与）

必需配置：[single_repo] 段

---

6. 核心模块设计

6.1 平台客户端接口（common/platforms/interface.go）

```go
// PlatformClient 统一平台操作接口
type PlatformClient interface {
    // PR 操作
    GetPR(ctx context.Context, owner, repo string, number int) (*models.PRRecord, error)
    ListPRs(ctx context.Context, owner, repo string, state string) ([]*models.PRRecord, error)
    ApprovePR(ctx context.Context, owner, repo string, number int) error
    MergePR(ctx context.Context, owner, repo string, number int) error
    ClosePR(ctx context.Context, owner, repo string, number int) error
    ReopenPR(ctx context.Context, owner, repo string, number int) error
    CommentPR(ctx context.Context, owner, repo string, number int, body string) error
    AddLabel(ctx context.Context, owner, repo string, number int, label string) error
    RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error

    // 分支操作
    GetBranch(ctx context.Context, owner, repo, branch string) (bool, error)
    DeleteBranch(ctx context.Context, owner, repo, branch string) error
    GetDefaultBranch(ctx context.Context, owner, repo string) (string, error)

    // CI 状态
    GetCIStatus(ctx context.Context, owner, repo string, commitSHA string) (string, error)

    // 合并方式
    GetDefaultMergeMethod(ctx context.Context, owner, repo string) (string, error)
    HasMultipleMergeMethods(ctx context.Context, owner, repo string) (bool, error)

    // 审批状态
    GetApprovals(ctx context.Context, owner, repo string, number int) ([]string, error)

    // Webhook
    VerifyWebhookSignature(body []byte, signature string) bool

    // PR 的提交列表
    GetPRCommits(ctx context.Context, owner, repo string, number int) ([]string, error)
}

// PlatformType 平台类型
type PlatformType string

const (
    PlatformGitHub PlatformType = "github"
    PlatformGitLab PlatformType = "gitlab"
    PlatformGitea PlatformType = "gitea"
)
```

6.2 CI 检测器（common/ci/detector.go）

检测策略：检查仓库根目录是否存在 CI 配置文件，而非依赖 API 探测。

支持检测的 CI 系统：

平台 检测文件
GitHub Actions .github/workflows/*.yml, .github/workflows/*.yaml
GitLab CI .gitlab-ci.yml
Gitea Actions .gitea/workflows/*.yml, .gitea/workflows/*.yaml

接口：

```go
type CIDetector interface {
    Detect(ctx context.Context, client PlatformClient, owner, repo string, branch string) (string, error)
    // 返回 "github_actions" | "gitlab_ci" | "gitea_actions" | "none"
}
```

逻辑：

· 在配置加载后、初始化平台客户端时调用
· 如果 TOML 中显式设置了 ci_provider，则以配置为准，跳过文件检测
· 如果 ci_provider = "none"，合并条件忽略 CI

6.3 合并方式检查器（common/platforms/merge_checker.go）

启动时执行，在 asikad 初始化阶段：

```go
func CheckMergeMethods(cfg *Config, clients map[PlatformType]PlatformClient) error {
    repos := getRepos(cfg) // 根据 mode 获取所有仓库
    for _, repo := range repos {
        client := clients[repo.Platform]
        hasMultiple, err := client.HasMultipleMergeMethods(ctx, repo.Owner, repo.Name)
        if err != nil {
            return fmt.Errorf("failed to check merge methods for %s/%s: %w", repo.Owner, repo.Name, err)
        }
        if hasMultiple {
            defaultMethod, err := client.GetDefaultMergeMethod(ctx, repo.Owner, repo.Name)
            if err != nil || defaultMethod == "" {
                return fmt.Errorf("repository %s/%s has multiple merge methods but cannot determine default: %v", repo.Owner, repo.Name, err)
            }
        }
    }
    return nil
}
```

失败行为：返回错误 → cmd/asikad/main.go 输出 FATAL: merge method check failed: <error> → os.Exit(1)

6.4 合并队列（daemon/queue/manager.go）

通用合并队列：不依赖任何平台的 Merge Queue 特性，纯 Bot 自动化。

合并条件：

1. 所需数量的核心贡献者批准（从 core_contributors 列表匹配）
2. CI 状态通过（若 ci_provider != "none"）

合并方式：

· 调用平台 API 合并时，若平台支持多种合并方式，查询仓库默认值并传参
· 若平台只有一种合并方式，直接使用该方式
· 若平台有多种但查不到默认值 → 已在启动时被拦截，不会执行到此

流程图：

```
PR 事件 → 加入队列(waiting) → 定时检查(checking) →
  ├─ 批准数不足 → 继续等待
  ├─ CI 未通过 → 继续等待
  ├─ 条件满足 → 合并(merging) → 成功(done)
  └─ 超时/异常 → 失败(failed)
```

6.5 同步引擎（daemon/syncer/syncer.go）

仅在多平台模式（mode = "multi"）下启用。

触发条件：

· Webhook 收到 merge 事件
· 轮询检测到 PR 状态变为 merged

流程：

1. 检测到平台 A 的 PR 被合并
2. 在 git.workdir 下创建临时裸仓库
3. 添加平台 A、B、C 的 remote
4. Fetch 平台 A 的目标分支
5. 对平台 B 和 C 执行 git cherry-pick <merge_commit>
6. 执行 hooks（pre-receive, update, post-receive）
7. Push 到平台 B 和 C
8. 记录同步历史
9. 清理临时仓库

分支删除同步：

· 检测到分支删除 → 在其他平台删除同名分支
· 若目标平台该分支有未合并 PR，先评论再删除

6.6 Spam 检测（daemon/syncer/spam.go）

检测器在全部模式（multi 和 single）下启用。

定时扫描（每分钟）：

1. 查询 time_window 内的所有 PR 事件
2. 检查条件：
   · trigger_on_author = true：同一作者超过 threshold 个 PR
   · trigger_on_similar_title = true：标题相似度超过阈值
   · trigger_on_keywords：标题包含关键词
3. 满足任一条件 → 将匹配的 PR 标记为 spam

Spam 处理：

1. Bot 关闭所有 spam PR
2. 发送通知（所有配置的通知渠道）
3. 记录审计日志

Reopen 处理（管理员操作）：

1. 清除 spam 标记
2. 将 PR 提交使用 git cherry-pick 推送到目标分支
3. 不进入常规合并队列

6.7 Hooks 执行器（daemon/hooks/runner.go）

· 读取 hook 脚本目录（TOML hookpath 或全局 hookpath）
· 目录结构须与 .git/hooks 一致
· 按名称匹配执行（pre-receive, update, post-receive）
· 环境变量：GIT_DIR, OLD_REV, NEW_REV, REF_NAME
· 超时：30 秒
· 失败：记录警告日志，跳过该 hook，不中断主流程

6.8 通知器（daemon/notifier/）

接口：

```go
type Notifier interface {
    Type() string
    Send(ctx context.Context, title, body string) error
}
```

已实现：

· smtp：邮件通知
· wecom：企业微信 Webhook
· github_at：GitHub PR 评论 @用户
· gitlab_at：GitLab MR 评论 @用户

配置：[[notify]] 数组，支持多个通知器同时工作。

---

7. API 设计

前缀：/api/v1
认证：Authorization: Bearer <jwt_token>（除了 /api/v1/login、初始化向导相关路由）

7.1 认证

方法 路径 权限 说明
POST /api/v1/login 无 登录，返回 JWT
POST /api/v1/logout 登录 登出（token 加入黑名单）
GET /api/v1/users admin 用户列表
POST /api/v1/users admin 创建用户
DELETE /api/v1/users/:username admin 删除用户

7.2 PR 审计与操作

方法 路径 权限 说明
GET /api/v1/repos/:repo_group/prs viewer+ PR 列表，查询参数 ?state=&platform=
GET /api/v1/repos/:repo_group/prs/:pr_id viewer+ PR 详情
POST /api/v1/repos/:repo_group/prs/:pr_id/approve operator+ 批准 PR
POST /api/v1/repos/:repo_group/prs/:pr_id/close operator+ 关闭 PR
POST /api/v1/repos/:repo_group/prs/:pr_id/reopen operator+ 重新打开 PR
POST /api/v1/repos/:repo_group/prs/:pr_id/spam operator+ 标记 Spam
DELETE /api/v1/repos/:repo_group/prs/:pr_id/spam operator+ 取消 Spam

7.3 合并队列

方法 路径 权限 说明
GET /api/v1/queue/:repo_group viewer+ 队列状态
POST /api/v1/queue/:repo_group/recheck operator+ 手动触发重检

7.4 标签规则

方法 路径 权限 说明
GET /api/v1/rules/labels viewer+ 查看规则
PUT /api/v1/rules/labels admin 更新规则（写入 bbolt + 触发热加载）

7.5 同步历史

方法 路径 权限 说明
GET /api/v1/sync/history viewer+ 分页同步历史
POST /api/v1/sync/retry/:sync_id operator+ 重试同步

7.6 配置信息

方法 路径 权限 说明
GET /api/v1/config admin 返回当前配置（脱敏）

7.7 通知测试

方法 路径 权限 说明
POST /api/v1/test/notify admin 发送测试通知

7.8 初始化向导

方法 路径 权限 说明
GET /api/v1/wizard 无（仅在未初始化时可用） 获取向导步骤
POST /api/v1/wizard/step/:step 无（仅在未初始化时可用） 提交向导步骤数据

初始化检测中间件：若 /etc/asika_config.toml 不存在且请求路径不在 /api/v1/wizard* 或 /wizard，则重定向到向导页面。

7.9 WebUI 页面路由

路径 说明
/ 若未初始化 → 重定向 /wizard；已初始化 → Dashboard
/login 登录页面
/wizard Web 初始化向导
/dashboard 仪表板
/prs/:repo_group PR 列表
/prs/:repo_group/:pr_id PR 详情
/queue/:repo_group 合并队列

所有 WebUI 页面使用 Go html/template 渲染，静态资源（CSS/JS）通过 //go:embed 内嵌。

---

8. CLI 命令设计

二进制名：asika

```
asika
├── version          # 打印版本号
├── init             # 交互式配置向导（生成 /etc/asika_config.toml）
├── pr
│   ├── list         # asika pr list <repo_group> [--state=open] [--platform=github]
│   ├── show         # asika pr show <repo_group> <pr_id>
│   ├── approve      # asika pr approve <repo_group> <pr_id>
│   ├── close        # asika pr close <repo_group> <pr_id>
│   ├── reopen       # asika pr reopen <repo_group> <pr_id>
│   └── spam         # asika pr spam <repo_group> <pr_id>
│       └── --undo   # 取消 spam 标记
├── queue
│   ├── list         # asika queue list <repo_group>
│   └── recheck      # asika queue recheck <repo_group>
├── sync
│   ├── history      # asika sync history [--repo_group=] [--limit=20]
│   └── retry        # asika sync retry <sync_id>
├── config
│   ├── show         # asika config show（脱敏）
│   └── reload       # asika config reload（触发热更新）
└── wizard            # asika wizard（重新运行配置向导）
```

全局 flags：

· --token：JWT token（可替代 ASIKA_TOKEN 环境变量）
· --server：asikad 地址（默认 http://localhost:8080）
· --output：输出格式 table（默认）| json | yaml

---

9. 工作流详解

9.1 常规 PR 合并 + 代码同步（multi 模式）

```
┌──────────────────────────────────────────────────────────┐
│ 1. Webhook/轮询 检测到 PR merged 事件                      │
├──────────────────────────────────────────────────────────┤
│ 2. 记录 PR 状态为 merged，存储到 bbolt                       │
├──────────────────────────────────────────────────────────┤
│ 3. 获取 merge commit SHA                                  │
├──────────────────────────────────────────────────────────┤
│ 4. 在 git.workdir 创建临时裸仓库                            │
├──────────────────────────────────────────────────────────┤
│ 5. 添加三平台 remote，fetch 源平台分支                        │
├──────────────────────────────────────────────────────────┤
│ 6. 执行 pre-receive hooks（若配置）                         │
├──────────────────────────────────────────────────────────┤
│ 7. cherry-pick merge commit 到其他平台分支                   │
├──────────────────────────────────────────────────────────┤
│ 8. 执行 post-receive hooks（若配置）                        │
├──────────────────────────────────────────────────────────┤
│ 9. Push 到其他平台                                         │
├──────────────────────────────────────────────────────────┤
│ 10. 记录同步历史，清理临时仓库                                │
└──────────────────────────────────────────────────────────┘
```

9.2 Spam 自动触发与处理（全部模式）

```
┌────────────────────────────────────────────────────────┐
│ 1. spam_monitor 每分钟扫描 time_window 内的 PR 事件        │
├────────────────────────────────────────────────────────┤
│ 2. 条件匹配 → 标记 PR 为 spam                             │
├────────────────────────────────────────────────────────┤
│ 3. Bot 调用 API 关闭所有 spam PR                          │
├────────────────────────────────────────────────────────┤
│ 4. 发送通知（所有配置的 notifier）                          │
├────────────────────────────────────────────────────────┤
│ 5. 管理员审查 → CLI/WebUI 执行 reopen                      │
├────────────────────────────────────────────────────────┤
│ 6. 清除 spam 标记，获取 PR 提交列表                         │
├────────────────────────────────────────────────────────┤
│ 7. 使用 git cherry-pick 将提交推送到目标分支                  │
│    (single 模式: 本仓库; multi 模式: 所有平台)               │
├────────────────────────────────────────────────────────┤
│ 8. 记录审计日志，完成                                       │
└────────────────────────────────────────────────────────┘
```

9.3 合并队列自动合并（全部模式）

```
┌──────────────────────────────────────────────────────┐
│ PR 被创建/更新 → 入队(waiting)                          │
├──────────────────────────────────────────────────────┤
│ checker 定时扫描队列                                    │
├──────────────────────────────────────────────────────┤
│ 检查批准: 从平台获取 Approvals，与 core_contributors 交集  │
├──────────────────────────────────────────────────────┤
│ 检查 CI:                                                │
│   - ci_provider != "none" → 获取 CI 状态                │
│   - ci_provider == "none" → 跳过                       │
├──────────────────────────────────────────────────────┤
│ 条件满足 → 调用平台 API 合并                              │
│   - 查询默认合并方式（若有多种）                            │
│   - 使用该方式执行合并                                    │
├──────────────────────────────────────────────────────┤
│ 合并成功 → done → 触发同步（multi 模式）                   │
│ 合并失败 → failed → 记录原因                              │
└──────────────────────────────────────────────────────┘
```

---

10. 安全设计

10.1 Webhook 签名验证

· 使用 HMAC-SHA256
· 密钥通过 events.webhook_secret 配置
· 验证失败 → HTTP 403，不处理事件

10.2 用户认证

· 密码：bcrypt 哈希存储
· JWT：HS256，过期时间由 auth.token_expiry 配置
· 登出：token 加入内存黑名单（定时清理过期条目）

10.3 权限模型

角色 权限
admin 所有权限（配置管理、用户管理、PR 操作、队列管理）
operator PR 操作（批准、关闭、reopen、spam）、队列查看/重检、同步重试
viewer 只读（PR 列表、详情、队列查看、同步历史）

10.4 平台令牌

· 存储在 TOML 配置文件中
· 可通过环境变量覆盖（ASIKA_GITHUB_TOKEN, ASIKA_GITLAB_TOKEN, ASIKA_GITEA_TOKEN）
· 优先级：环境变量 > 配置文件

---

11. 冷/热更新策略

配置项 更新方式 生效时机
label_rules 热更新 API 触发或文件变化，立即生效
notify 热更新 下次通知使用新配置
core_contributors 热更新 下次队列检查时读取
hookpath（全局/仓库组） 热更新 下次同步操作时生效
spam 规则 冷更新 需重启 asikad
events.mode 冷更新 需重启 asikad
repo_groups / single_repo 冷更新 需重启 asikad
tokens 冷更新 需重启 asikad
database.path 冷更新 需重启 asikad
server.listen 冷更新 需重启 asikad
mode 冷更新 需重启 asikad

热更新实现：

· 使用 fsnotify 监听 TOML 文件变化
· API 端点 PUT /api/v1/rules/labels 直接写入 bbolt config bucket
· 配置读取函数优先从 bbolt config bucket 读取，fallback 到内存中的初始配置

---

12. 错误处理与日志

12.1 日志

· 结构化日志使用 log/slog
· 日志轮转：lumberjack，默认保留 7 天，单文件最大 10MB
· 日志输出到 /var/log/asika/asikad.log（通过 lumberjack 配置）
· 同时写入 bbolt logs bucket（持久化审计）

12.2 API 错误响应

```json
{
    "error": "human-readable message",
    "code": 400
}
```

HTTP 状态码：

· 400：请求参数错误
· 401：未认证
· 403：无权限
· 404：资源不存在
· 409：冲突（如 PR 已关闭）
· 500：内部错误

12.3 启动错误

· 合并方式检查失败 → FATAL: <error> → os.Exit(1)
· 数据库打开失败 → FATAL: <error> → os.Exit(1)
· 平台令牌缺失 → FATAL: <error> → os.Exit(1)
· 监听端口占用 → FATAL: <error> → os.Exit(1)

---

13. 部署与服务文件

13.1 systemd unit（daemon/service/asikad.service）

```ini
[Unit]
Description=Asika PR Manager Daemon
After=network.target

[Service]
Type=simple
User=asika
Group=asika
ExecStart=/usr/local/bin/asikad
Environment=ASIKA_CONFIG=/etc/asika_config.toml
Environment=GOMEMLIMIT=256MiB
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=asikad
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/asika /var/log/asika

[Install]
WantedBy=multi-user.target
```

13.2 openrc init 脚本（daemon/service/asikad.openrc）

```sh
#!/sbin/openrc-run

name="asikad"
description="Asika PR Manager Daemon"
command="/usr/local/bin/asikad"
command_args=""
pidfile="/run/asikad.pid"
command_background=yes
export ASIKA_CONFIG="/etc/asika_config.toml"
export GOMEMLIMIT="256MiB"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath --directory --owner asika:asika /var/lib/asika
    checkpath --directory --owner asika:asika /var/log/asika
}
```

---

14. 初始化向导

14.1 触发条件

当 /etc/asika_config.toml 不存在时：

· CLI：asika 任何需要配置的命令自动重定向到 asika init
· WebUI：所有页面重定向到 /wizard

14.2 向导流程（CLI 和 WebUI 共享逻辑，common/config/wizard.go）

步骤：

1. 模式选择：single / multi
2. 平台令牌：GitHub / GitLab / Gitea token
3. 仓库配置：
   · single：选择平台、输入仓库全名、默认分支
   · multi：输入仓库组名称、各平台仓库全名、默认分支
4. 事件源：webhook / polling
5. 通知配置：选择类型、填写参数
6. 管理员账户：用户名、密码
7. 确认写入：生成 TOML 写入 /etc/asika_config.toml

Web 向导：每步一个页面，数据暂存 session，最后一步写入文件并重启服务。

14.3 重新运行向导

· CLI：asika wizard（需 admin 权限）
· WebUI：/wizard（需 admin 权限，用于修改配置）

运行向导时不会覆盖现有配置，而是生成新配置并提示对比。

---

15. 测试策略

15.1 覆盖范围

· common/：全部单元测试
· lib/：全部单元测试 + 集成测试（命令执行）
· daemon/：全部单元测试 + handler 集成测试（httptest）
· cmd/：不要求测试

15.2 Mock 工具（testutil/）

testutil/db.go：

```go
// NewTestDB 创建临时 bbolt 数据库
func NewTestDB(t *testing.T) *bbolt.DB
```

testutil/mocks.go：

```go
// MockGitHubClient 实现 platforms.PlatformClient 接口
type MockGitHubClient struct {
    PRs        map[int]*models.PRRecord
    MergeMethods []string
    DefaultMethod string
    // ...
}
// 所有方法均可通过字段注入返回值
```

testutil/auth.go：

```go
// GenerateTestToken 生成用于测试的 JWT
func GenerateTestToken(username, role string) (string, error)
```

15.3 测试文件命名

· *_test.go 放在与被测代码相同目录
· testutil/ 本身不包含测试，仅提供辅助工具

---

16. 行为边界与约束

16.1 必须遵守

1. 单二进制部署：asika 和 asikad 各自编译为独立静态二进制
2. 单仓库模式零同步：不执行任何跨平台代码推送
3. 合并方式退出：启动检查失败 → os.Exit(1)
4. 合并队列不依赖平台 Merge Queue：纯内部实现
5. 通知失败不影响主流程：通知发送失败记录日志但不阻断操作
6. Hook 失败不中断同步：记录警告，继续执行
7. Webhook 幂等：通过 commit SHA 去重，避免重复同步
8. 临时仓库清理：同步完成后删除，定时清理残留

16.2 必须避免

1. 不得添加任何未提及的依赖
2. 不得实现任何未提及的功能
3. 不得引入外部服务依赖（如 Redis、MySQL）
4. 不得在 single 模式下执行跨平台同步
5. 合并方式不确定时不得 fallback 到某种默认方式（启动时直接退出）
6. 不得使用 GitHub Merge Queue 或平台专属合并队列功能

16.3 边界处理

· 平台不可达：记录错误，不影响其他平台操作
· cherry‑pick 冲突：标记失败，通知管理员，不自动解决
· 重复 Webhook 事件：通过 commit SHA 幂等处理
· 磁盘空间不足：记录错误，暂停同步
· 用户不存在（API）：返回 404

---

17. 常见歧义澄清

术语 明确含义
标签 平台原生 PR Labels（如 GitHub Labels），非 Git tags
通用合并队列 Asika 内部状态机，不依赖任何平台 Merge Queue 功能
Spam reopen cherry‑pick 使用 go-git 在本地操作，推送时绕过平台合并 API
CI 状态 以 PR 源平台的 CI 为准；通过仓库文件检测 CI 存在性
Hook 执行位置 Asika 临时裸仓库，不影响用户仓库
多仓库 一个 asikad 实例管理多组仓库，数据隔离
用户认证 Asika 自身管理用户，不影响平台仓库权限
单仓库模式 仅管理一个平台的一个仓库，不执行跨平台同步
初始化向导 配置文件不存在时自动触发，CLI 和 WebUI 均可用
热更新 部分配置（标签规则、通知、贡献者列表）无需重启即可生效
冷更新 需重启 asikad 的配置（监听模式、仓库组、令牌等）

---

文档结束。请严格按照此文档实现全部代码，不得偏离或推测性添加功能。所有未定义行为默认为不实现。
