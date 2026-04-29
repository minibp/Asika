module asika

go 1.21.0

require (
	// HTTP 框架 - WebUI 后端与 API
	// 最新稳定版 v1.12.0，支持 HTTP/3、Protobuf 渲染，极致性能
	github.com/gin-gonic/gin v1.12.0

	// CLI 框架
	// 最新 v1.9.0，支持命令行解析、自动补全、帮助生成
	github.com/spf13/cobra v1.9.0

	// TOML 配置文件解析
	// 最新 v1.6.0，反射驱动解析，支持 TextUnmarshaler 接口
	github.com/BurntSushi/toml v1.6.0

	// 嵌入式 KV 数据库 - PR 记录、审计日志、标签规则、同步状态存储
	// v1.4.3，支持 ACID、MVCC、纯 Go 实现
	go.etcd.io/bbolt v1.4.3

	// 纯 Go Git 操作库 - cherry-pick、分支删除、代码推送
	// 最新 v5.18.0，无需系统 Git 依赖
	github.com/go-git/go-git/v5 v5.18.0

	// GitHub API 客户端
	// 最新 v84.0.0，支持 v3 API，包含原生迭代器
	github.com/google/go-github/v69 v69.0.0

	// GitLab API 客户端（官方维护，xanzy/go-gitlab 已弃用迁移至此）
	// 建议使用官方 client-go，版本号需按 GitLab 实例版本选择对应分支
	gitlab.com/gitlab-org/api/client-go v0.115.0

	// Gitea API 客户端
	// v0.13.2，Gitea 服务端同仓库维护
	code.gitea.io/sdk/gitea v0.13.2

	// 日志轮转 - 生产级日志文件滚动
	// v2.2.1，支持按大小轮转、压缩、过期清理
	gopkg.in/natefinch/lumberjack.v2 v2.2.1

	// UUID 生成 - PR 唯一标识
	// v1.6.0，支持 v1-v5 RFC 9562
	github.com/google/uuid v1.6.0

	// 定时任务 - Webhook 心跳、超时 PR 检查
	// v2.16.0，支持 cron 表达式、自定义调度
	github.com/go-co-op/gocron/v2 v2.16.0

	// 邮件通知 - Spam 模式告警
	// v0.7.2，支持 SMTP/ESMTP，附带到邮件
	github.com/wneessen/go-mail v0.7.2

	// 密码哈希 - 管理用户认证
	// 标准库扩展包，v0.45.0，bcrypt 实现
	golang.org/x/crypto v0.45.0

	// JWT 令牌 - CLI/WebUI 会话管理
	// v5.3.1，支持 Ed25519、PS256 等现代算法
	github.com/golang-jwt/jwt/v5 v5.3.1

	// YAML 序列化 - 标签规则文件解析（备选格式）
	// v3.0.1
	gopkg.in/yaml.v3 v3.0.1
)
