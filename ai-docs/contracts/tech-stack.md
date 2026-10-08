# 技术栈

本文档记录推荐版本和接入方式。版本号按团队实际 CI 环境固定。

## 总览

| 层 | 选择 | 用途 | 状态 |
| --- | --- | --- | --- |
| 客户端 | Godot 4.7.2，.NET 8 target | 场景、渲染、输入 | 骨架已接入 |
| 客户端网络 | Godot `WebSocketPeer` | WebSocket 文本帧 | 骨架已接入 |
| 客户端协议 | `System.Text.Json` | JSON DTO 和事件载荷 | 骨架已接入 |
| 客户端核心 | 纯 .NET 类库 | 协议、会话、回放缓存 | 骨架已接入 |
| 服务端语言 | Go 1.27+ | 权威结算、房间、连接 | 骨架已接入 |
| 服务端网络 | `net/http` + `github.com/coder/websocket` | HTTP 和 WebSocket | 骨架已接入 |
| 服务端日志 | `log/slog` | 结构化日志 | 骨架已接入 |
| 服务端随机 | `math/rand` 注入种子 | 可记录随机来源 | 骨架已接入 |
| 协议 | WebSocket JSON，版本 `0.1` | 意图、事件、快照 | 已定义 |
| 规则测试 | Go `testing` | 纯规则和房间测试 | 已接入 |
| 客户端单测 | xUnit | 协议和会话逻辑 | 已接入 |
| 格式化 | `gofmt`、`dotnet format` | 两端格式化 | 已接入 |
| 静态检查 | `go vet`、`golangci-lint` | Go 静态分析 | 已接入 |
| 安全扫描 | `govulncheck` | Go 依赖漏洞扫描 | 已接入 |
| LLVM / cgo | LLVM 23.1.3，Windows `clang-cl` | race、交叉语言编译边界和检查工具 | 已固定 |
| 任务入口 | Go Task 3.54.0（根目录 `Taskfile.yml`） | 跨平台统一命令入口 | 已接入 |

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
  internal/platform/pathutil/  跨平台路径语义
  internal/platform/proc/      进程启动、取消与超时（工具与端到端测试使用）
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

开发机只需要 Go；其余依赖由引导器装进仓库内 `.tools/`：

```powershell
go -C tools/toolchain run . bootstrap
```

- Go：1.27 或更新，实际以 `server/go.mod` 为准（`golangci-lint` 2.14.0 要求 Go 1.26+，因此下限取 1.27 与本地验证一致）。
- .NET SDK：8.0.425，`global.json` 固定同一版本，`rollForward: latestMajor` 允许使用更高版本 SDK。
- Godot：4.7.2，必须使用 .NET 版本。
- Go Task：3.54.0，由 `go install` 装到 `$(go env GOPATH)/bin`。
- golangci-lint：2.14.0；govulncheck：1.1.4；装到 `.tools/bin`。
- LLVM：23.1.3（Windows 由引导器安装，Unix 使用系统 `clang`）；Windows cgo 运行时使用 LLVM-MinGW UCRT 20260908。

Godot 版本和 `Godot.NET.Sdk` 版本必须匹配。升级 Godot 时同步修改 `client/Riftcards.Client.csproj` 中的 SDK 版本并运行客户端编译。
LLVM 版本由 `Taskfile.yml` 的 `LLVM_VERSION` 声明；Windows 编译器入口为 `tools/llvm/clang-cl.cmd`，其 `LLVM_MINGW_ROOT`、`LLVM_CLANG_CL`、`LLVM_MAJOR`、`CC`、`CXX` 由 `.tools/toolchain.env` 提供。

## 依赖引导与版本锁定

依赖的版本、下载地址和校验哈希只在 `Taskfile.yml` 的 `vars:` 声明一次；[tools/toolchain](../../tools/toolchain)（独立 Go 模块）读取该声明并落实安装：

- `bootstrap`：安装 Go 工具，下载并校验 .NET SDK、Godot、Windows LLVM（官方 MSI 管理安装）与 LLVM-MinGW，按锁文件还原 NuGet 依赖；`--without <组件>` 跳过，`--force` 重装。
- `check`：检查依赖是否存在并满足锁定版本，缺失时给出工具名、最低版本和安装入口（`task env`）。
- `env`：打印生成的 `.tools/toolchain.env` 映射。

安装位置固定为仓库内 `.tools/`（git 忽略），`.tools/toolchain.env` 被 Taskfile 的 `dotenv` 读取，再映射成 `CC`、`CXX`、`DOTNET_ROOT`、`LLVM_MINGW_ROOT`、`LLVM_CLANG_CL`。因为 Task 的 dotenv 不会覆盖已存在的环境变量，映射而非直接设置 PATH 是刻意的：开发者环境不被改写。

| 依赖 | 版本声明 | 获取方式 |
| --- | --- | --- |
| Go 与 Go 模块 | `server/go.mod`、`server/go.sum` | 开发者安装的 Go |
| Go Task、golangci-lint、govulncheck | `Taskfile.yml` | `go install` |
| .NET SDK | `Taskfile.yml`、`global.json` | 官方压缩包 + SHA512 |
| Godot（.NET 版） | `Taskfile.yml` | 官方 release 压缩包 + SHA256 |
| Windows LLVM、LLVM-MinGW | `Taskfile.yml` | 官方 MSI 管理安装（`msiexec /a`）与官方 zip，均校验 SHA256 |
| NuGet 包、Godot SDK 包 | `client/**/packages.lock.json` | `dotnet restore --locked-mode` |
| Godot SDK 与工程特性版本 | `client/Riftcards.Client.csproj`、`client/project.godot` | NuGet 还原 |

升级任何一项时，同一 commit 更新 `Taskfile.yml` 声明、受影响文档和 task；引导器不硬编码版本，`TC_<KEY>` 环境变量可临时覆盖声明用于验证。

## 运行

```powershell
task run:server
task run:client
```

服务端默认监听 `127.0.0.1:8080`，客户端默认连接 `ws://127.0.0.1:8080/ws`。

## GitHub Actions CI

`.github/workflows/ci.yml` 在 Windows、macOS、Linux 三平台运行同一门禁：

1. `actions/setup-go` 按 `server/go.mod` 准备 Go（唯一非 Go 引导项）。
2. `go -C tools/toolchain run . bootstrap --without godot` 安装全部剩余依赖，并缓存 `.tools/cache` 与 NuGet 包。
3. `task ci`：Go 格式只读检查、`go vet`、`golangci-lint`、C# 构建与格式检查、`go test -race` 和 `dotnet test`。

CI 只保留 `setup-go` 一个工具类 action，.NET SDK、Go Task、LLVM 与 LLVM-MinGW 都来自 Taskfile 声明；配置最小权限、并发取消、超时和两级缓存。纯文档改动不触发完整构建。Godot 编辑器导入、场景测试和本地端到端测试作为后续独立验证，避免所有提交都依赖图形工具。

## 暂不接入

- 数据库：当前房间只在内存中，Phase 2 再接持久化。
- Redis：单实例配对足够时不需要。
- Kafka/NATS：事件日志先写进程内和文件，规模扩大后再引入。
- Kubernetes：先保证单进程服务行为和可观测性。
- 客户端状态同步框架：规则规模小，显式事件模型更易审查。
