# AGENTS.md - Asika 开发指南

## 快速命令

```bash
# 构建两个二进制文件
bash build.sh

# 运行所有测试
go test ./common/... ./lib/... ./daemon/...

# 运行单个包测试
go test ./common/config/...

# 运行单个测试
go test ./common/config -run TestLoad

# 构建指定二进制
go build -o asika ./cmd/asika
go build -o asikad ./cmd/asikad
```

## 项目结构

```
cmd/asika/      → CLI 命令行工具 (Cobra)
cmd/asikad/     → 守护进程 (HTTP 服务器)

common/         → 公共库
  config/       → 配置加载、热/冷重载
  db/           → bbolt 数据库封装
  platforms/    → GitHub/GitLab/Gitea 客户端
  events/       → 内部事件总线
  auth/         → JWT、密码哈希
  gitutil/      → 纯 Go git 操作（无需系统 git）

daemon/         → 守护进程代码
  server/       → HTTP 服务器、中间件
  handlers/     → API 路由
  queue/        → 合并队列状态机
  syncer/       → 跨平台同步 + 垃圾检测
  polling/      → 轮询模式（webhook 的替代方案）
  labeler/      → 标签规则引擎
```

## 关键事实

- **Go 版本**：1.25.0+（go.mod 中明确指定）
- **配置位置**：`/etc/asika_config.toml` 或通过 `--config` 指定
- **数据库**：bbolt（嵌入式，无需外部数据库）
- **无外部 git 依赖**：使用 `go-git` 库
- **UI 嵌入**：Web UI 模板通过 `go:embed` 编译时嵌入

## 重要约定

- **配置热重载**：编辑配置文件后守护进程自动重载，无需重启
- **单二进制**：CLI 和守护进程都是独立的，无 Node.js 或其他外部依赖
- **多平台支持**：同一代码通过接口处理 GitHub、GitLab、Gitea
- **事件驱动**：内部事件总线解耦组件（webhook/轮询 → 事件 → 处理器）

## 测试说明

- 测试使用 `t.TempDir()` 创建临时目录
- 数据库测试使用内存 bbolt
- `testutil/` 提供 Mock 平台客户端用于隔离测试

## 验证命令

```bash
# 编译检查
go build ./...

# 测试
go test ./...
```

无正式 lint 管道，基本验证使用 `go build` 和 `go test`。

## 版本规则

使用日期版本：`YYYYMMDD` + 后缀（DEV、HF、CVE、DEP）。详见 CONTRIBUTING.md。

## 参考文档

- 完整配置参考：`asika.toml.example`
- 架构图：`PROJECT.md`
- 贡献指南：`CONTRIBUTING.md`