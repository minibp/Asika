Asika 详细设计文档（LLM 实现指南）

---

0. 项目概述

Asika 是一个轻量级、纯后端的 PR 管理器与多平台 Git 代码互通器。它以单一静态二进制分发，可在 1c1g VPS 上稳定运行，提供 CLI 管理工具 与 WebUI（内嵌） 两种交互方式。核心使命是自动化管理 Pull Request（PR）/Merge Request（MR），并在 GitHub、GitLab、Gitea（兼容 Forgejo）之间实现可选的代码同步。

关键设计目标：

· 单二进制、零外部依赖
· 内存占用低于 50MB（1c1g 友好）
· 事件驱动（Webhook/Polling 二选一，冷切换）
· 通用合并队列（不依赖平台原生的 Merge Queue）
· 支持常规模式与 Spam 模式
· 内嵌自定义 Git Hooks 执行
· 细粒度权限管理（用户名+密码+JWT）
· 支持单仓库模式（纯 PR 管理，不做跨平台同步）与多平台互通模式

---

1. 核心概念定义

1.1 仓库组（Repo Group）

一个业务逻辑上的“仓库”，可以绑定到三个不同平台（GitHub、GitLab、Gitea/Forgejo）。例如：

```
github: owner/repo
gitlab: group/repo
gitea:  owner/repo
```

所有平台仓库共享同一个默认分支，Asika 负责在这些不同平台的同名仓库间协调 PR 状态和代码同步。

1.2 工作模式

· 多平台互通模式（默认）：三个平台仓库均配置，任一平台 PR 被合并后，自动通过 cherry‑pick 将代码同步到其他两个平台。分支删除也会同步删除。
· 单仓库模式：仅配置一个平台仓库（如只选 GitHub），Asika 不执行任何跨平台代码同步。管理员需自行手动同步到其他平台。但 Asika 仍然提供该仓库的全套 PR 管理功能：审计、标签、合并队列、Spam 检测、通知等。
· 模式由 mode 配置项决定：single 或 multi。若为 single，必须通过 mirror_platform 指定唯一的镜像源平台。

1.3 PR（Pull Request）统一抽象

无论来自 GitHub Pull Request、GitLab Merge Request 还是 Gitea Pull Request，Asika 内部都映射为统一的数据结构 PRRecord，包含通用字段（标题、作者、状态、标签、事件时间线等），平台差异仅在 Platform 字段和底层 API 适配器中处理。

1.4 通用合并队列（Internal Merge Queue）

Asika 维护自己的合并队列，不依赖任一平台的原生 Queue 功能（如 GitHub Merge Queue）。队列会检查：核心贡献者批准数量、CI 状态（若存在），条件满足后由 Bot 通过平台 API 执行合并。

1.5 Spam 模式

应对恶意自动化攻击或垃圾 PR 的机制。可手动触发，也可根据时间窗口内同作者/同标题/关键词自动触发。进入 Spam 的 PR 会被 Bot 立即关闭并通知管理员。管理员审查后可通过 reopen 命令恢复，此时不走正常合并流程，而是使用 git cherry-pick 直接将 PR 提交推送到目标分支。

---

2. 技术栈

2.1 语言 & 编译

· Go 1.21+（利用 embed 标准库打包静态资源）
· 编译为单二进制，使用 -ldflags="-s -w" 瘦身
· 内存限制通过环境变量 GOMEMLIMIT=256MiB 配置

2.2 关键依赖（go.mod）

类别 库 版本 用途
HTTP 框架 github.com/gin-gonic/gin v1.12.0 WebUI 后端、API
CLI github.com/spf13/cobra v1.9.0 asika 管理 CLI
配置解析 github.com/BurntSushi/toml v1.6.0 解析 asika.toml
嵌入式存储 go.etcd.io/bbolt v1.4.3 所有持久化数据（PR 记录、日志、规则等）
Git 操作 github.com/go-git/go-git/v5 v5.18.0 本地临时的 cherry‑pick、推送、分支操作（纯 Go，无需系统 Git）
GitHub API github.com/google/go-github/v69 v69.0.0 GitHub 交互
GitLab API gitlab.com/gitlab-org/api/client-go v0.115.0 官方 GitLab 客户端（注意：新包路径，旧 xanzy/go-gitlab 已弃用）
Gitea/Forgejo API code.gitea.io/sdk/gitea v0.13.2 Gitea 及兼容 Forgejo 的 API（自定义 URL 支持）
日志轮转 gopkg.in/natefinch/lumberjack.v2 v2.2.1 日志文件滚动
UUID github.com/google/uuid v1.6.0 PR 记录唯一标识
定时任务 github.com/go-co-op/gocron/v2 v2.16.0 轮询、心跳、超时检查
邮件通知 github.com/wneessen/go-mail v0.7.2 SMTP 邮件发送
密码哈希 golang.org/x/crypto v0.45.0 bcrypt 密码存储
JWT github.com/golang-jwt/jwt/v5 v5.3.1 管理 API 认证令牌
YAML（可选） gopkg.in/yaml.v3 v3.0.1 备选的标签规则格式（默认使用 TOML）

2.3 前端

· 完全使用 Go 的 html/template 模板引擎，不引入任何外部独立前端文件。
· 所有 HTML、CSS、JS 逻辑通过模板直接内嵌在二进制中，无需 Node.js 构建。
· 样式与交互力求轻量：内联 CSS 与少量 Alpine.js 片段（不再额外安装 npm 包，通过 CDN 或直接写入模板中）
· WebUI 的所有路由均由 gin 服务端渲染（SSR），无 SPA 架构。

2.4 外部自定义 URL 支持

· GitLab API 端点可以自定义，例如自托管实例 https://gitlab.example.com
· Gitea 同样支持自定义 URL，并且客户端兼容 Forgejo（API 一致）。
· 在配置中为每个平台提供 base_url 字段。

---

3. 配置系统（asika.toml）

3.1 配置文件位置

· 默认路径：/etc/asika_config.toml
· 可通过环境变量 ASIKA_CONFIG 指定
· 若启动时该文件不存在，且是以 Web 服务模式运行，自动进入 WebUI 初始化向导（引导页面），生成初始配置。

3.2 配置结构详解

```toml
# ========== 服务器基本 ==========
[server]
listen = ":8080"          # 监听地址
mode   = "release"        # "debug" 或 "release"（影响日志级别和模板缓存）

# ========== 数据库 ==========
[database]
path = "./asika.db"       # bbolt 数据库文件路径

# ========== 认证 ==========
[auth]
jwt_secret = "change-me"  # JWT 签名密钥
token_ttl  = "72h"        # 令牌有效期

# ========== 事件源配置 ==========
[events]
mode = "webhook"          # "webhook" 或 "polling"，冷切换
webhook_secret = "shared-secret"  # Webhook HMAC 密钥（webhook 模式必填）
polling_interval = "30s"          # 轮询间隔（polling 模式使用）

# ========== Git 操作临时目录 ==========
[git]
workdir = "./_asika_work" # 临时裸仓库执行目录

# ========== 通知（可选多路） ==========
[[notify]]
type = "smtp"             # "smtp", "wecom", "github_at", "gitlab_at"
[notify.smtp]
host = "smtp.example.com"
port = 587
username = "asika@example.com"
password = "password"
to      = ["admin@example.com"]

# 第二路通知（可选）
#[[notify]]
#type = "github_at"

# ========== 标签规则（可热更新） ==========
[[label_rules]]
pattern = "**/*.go"
label   = "language/go"

[[label_rules]]
pattern = "scripts/*.sh"
label   = "scripts"

# ========== Spam 检测 ==========
[spam]
enabled = true
time_window = "10m"       # x 分钟
threshold   = 3           # y 个 PR 触发
trigger_on_author = true  # 检查作者是否相同
trigger_on_title_kw = ["spam", "hack"]  # 标题中的关键词

# ========== 合并队列默认值（可被仓库组覆盖） ==========
[merge_queue]
required_approvals = 1       # 所需核心贡献者批准数
ci_check_required  = true    # 是否需要 CI 通过
core_contributors = ["admin", "lead-dev"] # 核心贡献者用户名列表
# 如果仓库未明确声明 CI 提供者，则自动探测；也可显式全局指定：
# ci_provider = "github_actions"  # 可选 "github_actions", "gitlab_ci", "gitea_actions", "none"

# ========== 仓库组 ==========
# 多平台互通模式例子
[[repo_groups]]
name = "main-project"
mode = "multi"                # 可省略，默认为全局模式
github   = "myorg/myrepo"
gitlab   = "mygroup/myrepo"
gitea    = "myuser/myrepo"
default_branch = "main"
hookpath = "hooks/main-project"   # 自定义 hooks 路径，为空则禁用
# 覆盖合并队列参数
[repo_groups.merge_queue]
required_approvals = 2
ci_provider = "github_actions"   # 显式声明，避免自动探测失败

# 单仓库模式例子
[[repo_groups]]
name = "docs-only"
mode = "single"               # 强制单仓库模式
mirror_platform = "github"    # 镜像源平台
github = "myorg/docs"
gitlab = ""                   # 留空即可
gitea  = ""
default_branch = "gh-pages"
```

3.3 配置加载与更新策略

· 冷更新（需重启 daemon）：events.mode、仓库组增删、平台 API 令牌、database.path
· 热更新（无需重启）：标签规则、通知渠道列表、核心贡献者列表、hookpath（下次操作生效）
· 热更新通过 API PUT /api/v1/config 或 SIGHUP 信号触发重载，配置模块使用原子指针保证并发安全。

---

4. 数据模型（bbolt 存储）

数据库使用单一文件 asika.db，内部划分多个 Bucket。

4.1 Bucket 设计

Bucket 名称 键格式 值类型 说明
config 配置项名（如 "label_rules"） JSON 字节 可热更新的动态配置，方便从 DB 读取
repos 仓库组名称（字符串） JSON RepoGroup 仓库组绑定信息及状态
prs <repo_group>#<platform>#<pr_number> JSON PRRecord PR 全量数据，platform 取值：github，gitlab，gitea
logs <timestamp_nano>_<random> JSON AuditLog 审计日志（操作记录、错误）
queue_items <repo_group>#<pr_id> JSON QueueItem 合并队列项状态
users 用户名 JSON User Asika 管理用户及权限

4.2 重要结构体定义

```go
// RepoGroup 仓库组配置（部分字段与 TOML 保持一致）
type RepoGroup struct {
    Name           string
    Mode           string // "multi", "single"
    MirrorPlatform string // 单仓库模式下的源平台，如 "github"
    GitHub         string // 仓库全名，如 "owner/repo"
    GitLab         string
    Gitea          string
    DefaultBranch  string
    HookPath       string
    MergeQueue     MergeQueueConfig // 覆盖队列配置
}

// PRRecord 统一 PR 记录
type PRRecord struct {
    ID              string    // UUID
    RepoGroup       string
    Platform        string    // "github","gitlab","gitea"
    PRNumber        int
    Title           string
    Author          string
    State           string    // "open","closed","merged","spam"
    Labels          []string
    MergeCommitSHA  string
    SpamFlag        bool
    LastUpdated     time.Time
    Events          []PREvent
}

// PREvent 事件
type PREvent struct {
    Timestamp time.Time
    Action    string // "opened","closed","approved","labeled","synced","cherry_picked","comment"等
    Actor     string // 操作者（用户或 Bot）
    Detail    string
}

// AuditLog 审计日志
type AuditLog struct {
    Timestamp time.Time
    Level      string
    Message    string
    Context    map[string]interface{}
}

// QueueItem 合并队列项
type QueueItem struct {
    PRID      string
    Status    string // "waiting","checking","merging","done","failed"
    AddedAt   time.Time
    Criteria  MergeCriteria
}

// MergeCriteria 合并条件快照
type MergeCriteria struct {
    RequiredApprovals int
    ApprovedBy        []string
    CIStatus          string // "pending","success","failure","none"
}

// User 管理用户
type User struct {
    Username string
    PasswordHash string
    Role     string // "admin","operator","viewer"
}
```

所有值均以 JSON 序列化存储，便于读写和调试。

---

5. 包结构与职责

5.1 顶层目录结构

```
asika/
├── cmd/
│   ├── asika/          # CLI 管理工具入口 (package main)
│   └── asikad/         # 守护进程入口 (package main)
├── common/             # 共享代码：配置、数据库、平台适配器、模型、工具函数
├── lib/                # CLI 特有：cobra 命令实现、输出格式化、配置向导（init）
├── daemon/             # 守护进程特有：HTTP 服务、API 路由、队列调度、同步引擎、Spam 监控、Web 初始化向导
├── daemon/service/     # 系统服务文件（systemd unit, openrc script）
├── go.mod
└── go.sum
```

5.2 模块职责

cmd/asika

· 构建 asika 二进制，只调用 lib 和 common，不启动 HTTP 服务。

cmd/asikad

· 构建 asikad 二进制，启动后台守护进程（Web 服务 + 事件监听 + 队列等）。调用 daemon 和 common。

common

· 配置加载与监听（热/冷更新）
· bbolt 数据库封装（所有 Bucket 操作的接口）
· 平台客户端接口定义与实现（GitHub、GitLab、Gitea/Forgejo）
· 数据模型（PRRecord, QueueItem, User 等）
· Git 操作工具（基于 go-git 的 cherry‑pick、分支删除、推送）
· JWT 生成/验证、密码哈希工具
· 通知接口（Notifier interface）及各渠道实现

lib

· Cobra 命令树构建（pr, queue, sync, config 等子命令）
· CLI 专用输出格式化（终端表格、JSON 等）
· init 交互式配置向导（CLI 方式，非 Web）

daemon

· asikad 启动逻辑：加载配置、初始化数据库、创建平台客户端、启动合并队列调度器、Spam 监控、事件监听器（Webhook 路由或轮询 ticker）
· Web UI 初始化向导（当配置文件不存在时，自动启动向导的 HTTP 路由）
· API 路由注册（全部 /api/v1/*）
· 通用合并队列实现（状态机、条件检查循环）
· 同步引擎（syncer）：多平台模式下的代码同步（cherry‑pick）、分支删除；单仓库模式下，同步引擎完全闲置，仅 Spam 的 cherry‑pick 由 daemon 直接调用 common 的 Git 工具执行
· Spam 监控协程：分析近期 PR 事件，触发自动标记和通知
· 守护进程启动时的合并方式检查：若某仓库有多种合并方式且无法确定默认值，daemon 立即以 exit code 1 退出。
· 系统信号处理（SIGHUP 重载配置）

daemon/service/

· asikad.service：systemd unit 文件模板
· asikad.openrc：openrc init 脚本
· 默认日志交给 journald 处理

---

6. 平台适配器接口设计（common 中定义）

Go 伪接口，确保可 Mock：

```go
// 统一 PR 对象（用于屏蔽平台差异）
type PullRequest struct {
    Number  int
    Title   string
    Author  string
    State   string // "open","closed","merged"
    Labels  []string
    MergeCommitSHA string
    DiffFiles []string // 变动文件列表（用于标签规则）
}

// 平台客户端接口
type PlatformClient interface {
    GetPR(owner, repo string, number int) (*PullRequest, error)
    ListPRs(owner, repo string, state string) ([]*PullRequest, error)
    ApprovePR(owner, repo string, number int) error
    MergePR(owner, repo string, number int, method string) error // method 可为空，用平台默认
    ClosePR(owner, repo string, number int) error
    ReopenPR(owner, repo string, number int) error
    AddLabel(owner, repo string, number int, label string) error
    GetCIStatus(owner, repo string, commitSHA string) (string, error) // "success","failure","pending","none"
    GetDefaultMergeMethod(owner, repo string) (string, error) // "merge","squash","rebase"，返回空表示未找到
    DeleteBranch(owner, repo string, branch string) error
    CommentPR(owner, repo string, number int, body string) error
    ListBranches(owner, repo string) ([]string, error)
    // Webhook 签名验证（每个实现自己处理）
    VerifyWebhookSignature(rawBody []byte, signature string) bool
}
```

GitHub、GitLab、Gitea 分别实现该接口。Gitea 实现兼容 Forgejo，自定义 URL 通过构造函数传入。

---

7. 核心工作流

7.1 启动初始化检查（asikad）

1. 加载配置文件（若缺失且为 Web 模式 → 进入 Web 配置向导）。
2. 初始化 bbolt 数据库。
3. 创建所有 repo_groups 中配置的平台客户端（通过令牌认证）。
4. 合并方式预检（致命级）：
   · 遍历每个仓库组，通过 API 获取仓库的默认合并方式。
   · 如果该仓库支持多种合并方式（merge, squash, rebase 中的多个）且 API 无法返回默认值，则 daemon 打印错误并 os.Exit(1)。
   · 如果仓库仅支持一种合并方式，直接采用该方式。
   · ci_provider 若未显式配置，Asika 会通过探测仓库中的 CI 配置文件（例如 .github/workflows, .gitlab-ci.yml）或平台特性来判断是否存在 CI；若无法判定，默认视为 "none" 来保证运行，但会发出警告。
5. 启动合并队列调度器、Spam 监控协程。
6. 根据 events.mode 启动 Webhook 监听器或轮询 ticker。
7. 启动 HTTP 服务（gin）。

7.2 事件摄入前端

Webhook 模式：

· 注册路由：/webhook/:repo_group/:platform，每个平台签名验证通过后，将事件转换为统一的内部事件对象放入事件总线（channel）。
· 支持 GitHub、GitLab、Gitea 的 Webhook 数据格式（适配器负责解析）。

轮询模式：

· 按 polling_interval 定期调用各平台 ListPRs，对比本地存储，检测变更（新增、关闭、合并、分支删除等），生成事件放入总线。

7.3 PR 生命周期管理（常规模式）

1. 事件：PR 打开 / 更新
   · 记录到 prs 表，触发标签规则引擎，匹配文件变动，调用平台 API 打上相应标签。
   · 不自动加入合并队列（仅当 Bot 收到批准事件或手动入队）。
2. 事件：核心贡献者批准
   · 检查该 PR 是否已在队列中，若不在且满足初步条件（非 Draft、无冲突），则加入合并队列。
3. 合并队列处理循环（独立 goroutine）
   · 扫描队列中 waiting 项：
          a. 获取最新 PR 信息，检查批准人数是否达标。
          b. 若 ci_check_required 为真且 CI provider 不为 none，获取最新 commit 的 CI 状态（需为 success）。
          c. 条件满足 → 调用平台 API 执行合并（使用预检确定的合并方式）。
          d. 成功后，生成 PRRecord.State = "merged"，并触发代码同步（仅多平台模式）。
4. 代码同步（syncer，多平台模式专属）
   · 在 git.workdir 下创建临时裸仓库，添加三平台 remote。
   · Fetch 源平台的目标分支。
   · 执行自定义 hooks（pre-receive, update 等），环境变量注入新旧 rev。
   · 对另外两个平台的对应分支执行 git cherry-pick <merge_commit>。
   · 推送结果到其他平台。
   · 记录同步日志；若某平台推送失败，记录错误并发送通知。
5. 分支删除同步（多平台模式）
   · 当检测到某平台的分支被删除，检查其他两平台是否存在同名分支：
     · 若存在且无未合并 PR 或已关闭 → 直接通过 API 删除。
     · 若存在且关联 open PR → 在该 PR 下评论警告，然后强行删除（可配置），记录日志。

7.4 Spam 模式处理

触发路径：

· 手动：管理员在 CLI/WebUI 标记某个 PR 为 spam。
· 自动：Spam 监控协程统计最近 time_window 内的新 PR，若来自同一作者或标题匹配关键词的数量 ≥ threshold，则将这些 PR 标记为 spam。

Spam 执行步骤：

1. Bot 调用对应平台 API 关闭这些 PR。
2. 通过配置的所有通知渠道向管理员发送告警。
3. 管理员审查后，对误判 PR 执行 reopen（CLI 或 WebUI）。
4. reopen 后的处理：
   · PR 状态改为 open，但不重新加入合并队列。
   · 使用 common 中的 Git 工具，将 PR 的提交通过 git cherry-pick 的方式应用到目标分支（所有配置的平台均需推送，即使单仓库模式下也是如此，但单仓库模式只推送到镜像源平台本身）。
   · 推送成功后，PR 状态更新为 merged（特殊标记：通过 cherry‑pick 完成）。
   · 此操作完全绕过平台 Merge API，直接基于 Git 操作。

7.5 单仓库模式的行为差异

· 配置 mode = "single" 并指定 mirror_platform。
· 不加载 syncer 模块，不监听其他两个平台（配置可留空）。
· 所有 PR 管理功能仍然有效：审计、标签、合并队列、Spam（仅针对镜像源平台）。
· Spam reopen 时的 cherry‑pick 仅推送到镜像源平台的目标分支。
· 管理员需自行使用第三方工具同步代码到其他平台（Asika 不参与）。

---

8. REST API 设计（daemon 提供）

所有管理 API 前缀 /api/v1，需 JWT 认证（Authorization: Bearer <token>）。

8.1 认证

· POST /api/v1/login — 登录，返回 JWT
· GET/POST /api/v1/users — 管理员管理用户（权限 admin）
· DELETE /api/v1/logout — 令牌加入黑名单（可选）

8.2 PR 与审计

· GET /api/v1/repos/:repo_group/prs — 列表，支持 ?state=open&platform=github
· GET /api/v1/repos/:repo_group/prs/:pr_id — 详情含事件时间线
· POST /api/v1/repos/:repo_group/prs/:pr_id/approve — Bot 批准
· POST /api/v1/repos/:repo_group/prs/:pr_id/close
· POST /api/v1/repos/:repo_group/prs/:pr_id/reopen — 用于 Spam 恢复
· POST /api/v1/repos/:repo_group/prs/:pr_id/spam — 手动标记/取消 spam
· GET /api/v1/logs — 审计日志，支持分页 ?since=&until=&level=

8.3 合并队列

· GET /api/v1/queue/:repo_group — 查看队列
· POST /api/v1/queue/:repo_group/recheck — 手动重检

8.4 配置与规则

· GET /api/v1/config — 当前生效配置（敏感信息脱敏）
· PUT /api/v1/config — 更新可热更新的配置（标签规则、通知等）
· 上述 put 会触发热加载

8.5 同步（仅多平台模式）

· GET /api/v1/sync/history — 同步历史
· POST /api/v1/sync/retry/:sync_id — 重试失败同步

8.6 通知测试

· POST /api/v1/test/notify — 发送测试通知

---

9. CLI 命令体系（asika）

二进制 asika 通过 Cobra 构建，所有命令需认证（JWT 令牌可通过 --token 或环境变量 ASIKA_TOKEN 传入）。

命令 子命令 说明
asika version - 打印版本信息
asika init - 交互式生成/修改配置（访问本地文件）
asika pr list, show, approve, close, reopen, spam PR 管理
asika queue list, recheck 队列管理
asika sync history, retry 同步操作
asika config show, reload 配置查看与热重载

CLI 命令内部通过 HTTP 调用本地 asikad 的 API，或直接访问数据库（某些本地操作），实现与 WebUI 相同的业务逻辑。

---

10. WebUI 向导（首次配置）

若 asikad 启动时未找到 /etc/asika_config.toml：

1. 启动一个临时的 HTTP 服务（端口可配置，默认 8080）。
2. 访问根路径，呈现多步表单（纯 Go 模板渲染），收集：
   · 工作模式（单 / 多平台）
   · 平台仓库地址、令牌
   · 管理员用户名密码
   · 通知邮箱等
3. 提交后写入配置文件，重启服务（或热加载）。

---

11. 测试策略

· cmd/asika 和 cmd/asikad 不包含单元测试。
· common、lib、daemon 必须提供充分的测试覆盖。
· Mock 机制：
  · common 中定义平台客户端接口，提供 MockPlatformClient 实现（模拟 PR 列表、CI 状态等）。
  · 提供 testutil 包，内含：临时 bbolt 数据库创建、模拟 JWT 生成、测试 fixtures 等。
· lib 测试主要验证 cobra 命令的参数解析与执行逻辑（可注入 mock 客户端）。
· daemon 的 HTTP handler 使用 httptest.NewServer 进行端到端测试，注入 mock 依赖。
· 所有测试可脱离真实外部服务运行。

---

12. 部署与运维

12.1 系统服务

daemon/service/ 目录包含：

· asikad.service：systemd 模板，定义：
  · 工作目录、用户、重启策略
  · 环境变量 ASIKA_CONFIG
  · 内存限制 MemoryMax=256M
· asikad.openrc：openrc init 脚本，类似功能

12.2 日志

· 应用日志使用 log/slog 输出到 stdout，由 systemd/journald 接管，不自行处理文件轮转（避免复杂性）。
· 审计日志同时写入 bbolt。

12.3 资源要求

· 1 vCPU / 1 GB 内存 VPS 即可稳定运行，推荐 GOMEMLIMIT=256MiB。

---

13. 边界与异常处理

· 所有外部 API 调用必须有超时（默认 10s），失败重试 1 次后记录错误。
· git cherry-pick 冲突时，不尝试自动解决，记录错误并通知管理员，暂停该 PR 的同步，等待手动干预。
· Webhook 重复事件通过检查 commit SHA 是否已处理来幂等。
· 数据库事务操作必须原子化，避免状态不一致。
· 配置文件格式错误时，daemon 直接 exit 1，打印错误位置。

---

14. 可能引起疑惑的澄清点（LLM 特别注意）

· “打标签” 是指在平台 PR 上添加标注（如 GitHub labels），不是 Git tag。
· 通用合并队列是内部状态机，与平台提供的 Merge Queue 功能无关。
· Spam 模式下的 reopen 后合并不走队列，通过 cherry-pick 直接推送。
· CI 状态检测：优先使用配置的 ci_provider，未配置时扫描仓库配置文件（如 .github/workflows/*.yml）判断，若无上述文件则认为 ci = none。
· 所有自定义 Git hooks 在临时裸仓库中执行，不影响用户本地仓库。
· 多仓库组之间严格隔离，各自独立的数据库 bucket 和事件处理。
· 权限管理仅针对登录 Asika 的管理员，不影响各平台的实际仓库权限（平台令牌统一配置控制）。
· 热更新和冷更新的具体列表严格遵循第 3.3 节，不可混用。

---

本文档已穷尽 Asika 的全部设计意图，请严格照此实现，不得引入任何超出范围的特性。