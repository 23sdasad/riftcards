# 技术栈

本文档记录推荐版本和接入方式。版本号在首次执行 `task bootstrap` 后需要按团队实际 CI 环境固定。

## 总览

| 层 | 选择 | 用途 | 状态 |
| --- | --- | --- | --- |
| 客户端 | Godot 4.4+，.NET 8 | 场景、渲染、输入 | 骨架已接入 |
| 客户端网络 | Godot `WebSocketPeer` | WebSocket 文本帧 | 骨架已接入 |
| 客户端协议 | `System.Text.Json` | JSON DTO 和事件载荷 | 骨架已接入 |
| 客户端核心 | 纯 .NET 类库 | 协议、会话、回放缓存 | 骨架已接入 |
| 服务端语言 | Go 1.25+ | 权威结算、房间、连接 | 骨架已接入 |
| 服务端网络 | `net/http` + `github.com/coder/websocket` | HTTP 和 WebSocket | 骨架已接入 |
| 服务端日志 | `log/slog` | 结构化日志 | 骨架已接入 |
| 服务端随机 | `math/rand` 注入种子 | 可记录随机来源 | 骨架已接入 |
| 协议 | WebSocket JSON，版本 `0.1` | 意图、事件、快照 | 已定义 |
| 规则测试 | Go `testing` | 纯规则和房间测试 | 已接入 |
| 客户端单测 | xUnit | 协议和会话逻辑 | 已接入 |
| 格式化 | `gofmt`、`dotnet format` | 两端格式化 | 已接入 |
| 静态检查 | `go vet`、`golangci-lint` | Go 静态分析 | 已接入 |
| 安全扫描 | `govulncheck` | Go 依赖漏洞扫描 | 已接入 |
| 任务入口 | Go Task | 跨平台统一命令 | 已接入 |

## 为什么服务端选择 Go

- 单二进制部署，适合权威房间服务。
- 标准库 HTTP、并发和日志足够稳定。
- 规则引擎可以保持纯包，便于大量单元测试。
- 内存模型和 WebSocket 写队列容易明确控制。
- 后续接数据库、指标和容器部署成本低。

## 为什么客户端 Core 与 Godot 分层

Godot 负责生命周期、渲染和输入；协议解析、会话状态、断线补帧和回放游标放在纯 .NET 类库。这样可以：

- 不启动 Godot 就运行协议和会话单测。
- 降低 C# 脚本与引擎 API 的耦合。
- 将来复用到编辑器工具或服务端回放查看器。

## 服务端模块

```text
server/
  cmd/riftcards-server/     进程入口、配置、HTTP 生命周期
  internal/protocol/        JSON 信封、DTO、错误码
  internal/game/            规则、状态、事件、视图裁剪
  internal/match/           队列、房间、串行结算、广播
  internal/transport/ws/    WebSocket 适配
```

依赖方向：

```text
cmd -> match -> game
cmd -> transport/ws -> match -> protocol -> game DTO
transport/ws -> protocol
```

`game` 不依赖网络。`protocol` 只定义传输结构，不做玩法结算。

## 客户端模块

```text
client/
  project.godot
  src/Core/                纯 .NET 协议、会话、回放
  src/Godot/               WebSocketPeer、Node 适配
  src/Main.cs              Godot 诊断界面
  tests/                   xUnit 测试
```

## 环境要求

- Go：1.25 或更新，实际以 `server/go.mod` 为准。
- .NET SDK：8.0 或更新。
- Godot：4.4 或更新，必须使用 .NET 版本。
- Go Task：3.x。
- golangci-lint：v2.x。

Godot 版本和 `Godot.NET.Sdk` 版本必须匹配。升级 Godot 时同步修改 `client/Riftcards.Client.csproj` 中的 SDK 版本并运行客户端编译。

## 运行

```powershell
task run:server
task run:client
```

服务端默认监听 `127.0.0.1:8080`，客户端默认连接 `ws://127.0.0.1:8080/ws`。

## CI 建议

CI 至少分三个阶段：

1. `task fmt` 的只读校验：`gofmt -l`、`dotnet format --verify-no-changes`。
2. `task lint`：`go vet`、`golangci-lint`、`dotnet build --warnaserror`。
3. `task test`：`go test -race`、`dotnet test`。

Godot 编辑器导入和场景测试作为独立 job，避免所有改动都依赖图形工具。

## 暂不接入

- 数据库：当前房间只在内存中，Phase 2 再接持久化。
- Redis：单实例配对足够时不需要。
- Kafka/NATS：事件日志先写进程内和文件，规模扩大后再引入。
- Kubernetes：先保证单进程服务行为和可观测性。
- 客户端状态同步框架：规则规模小，显式事件模型更易审查。
