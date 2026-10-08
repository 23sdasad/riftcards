# riftcards

riftcards 是一个两人回合制卡牌对战项目：

- 客户端：Godot 4 + C#
- 服务端：Go
- 结算：服务端权威
- 通信：WebSocket + JSON
- 客户端职责：发送指令、播放服务端事件、渲染按玩家裁剪后的状态
- 服务端职责：匹配、校验、结算、生成事件、广播回放、保存权威日志

当前仓库处于 Phase 0：规则、协议、工程边界、工具链和最小可运行骨架已经建立，玩法内容与 Godot 表现层会在后续阶段扩展。

## 文档入口

- [游戏规则](ai-docs/product/game-rules.md)
- [WebSocket 协议](ai-docs/contracts/protocol.md)
- [技术栈](ai-docs/contracts/tech-stack.md)
- [第三方库调研](ai-docs/contracts/library-research.md)
- [Path 与工程边界](ai-docs/architecture/paths-and-boundaries.md)
- [LLVM 工具链](tools/llvm/README.md)
- [AI 协作文档](ai-docs/README.md)
- [任务工作流](ai-docs/task-index.md)
- [提交规范](ai-docs/standards/commits.md)

## 目录

```text
.
├─ ai-docs/                 # 面向人和 AI 的长期事实来源
│  ├─ product/              # 规则、路线图和产品边界
│  ├─ architecture/         # 架构、约定和测试策略
│  ├─ contracts/            # 协议、技术栈和依赖调研
│  ├─ standards/            # 长期工程规范
│  ├─ task/                 # 单次任务的范围、决策与验证证据
│  └─ task-index.md         # 任务队列与状态摘要
├─ Taskfile.yml             # 跨平台统一命令入口（Go Task）
├─ global.json              # 锁定的 .NET SDK 版本
├─ server/                  # Go 权威服务端
│  ├─ cmd/riftcards-server/ # 服务入口
│  └─ internal/             # game / match / protocol / transport
├─ client/                  # Godot C# 客户端
│  ├─ src/Core/             # 与 Godot 解耦的协议和会话逻辑
│  ├─ src/Godot/            # Godot WebSocket 适配层
│  └─ tests/                # 纯 .NET 单元测试
├─ .github/workflows/       # GitHub Actions 三平台 CI
├─ tools/llvm/              # 固定 LLVM 编译器、clang-cl 包装器和检查工具说明
└─ AGENTS.md                # 后续 AI/自动化代理必须遵守的入口
```

## 本地命令

工具链版本见 [技术栈](ai-docs/contracts/tech-stack.md)。首次使用先安装 Go、.NET SDK、带 .NET 支持的 Godot 和 [Go Task](https://taskfile.dev/installation/)，然后检查环境并安装锁定版本的 Go 工具。

```powershell
task env          # 检查工具与版本，列出每个缺失项、最低版本和安装入口
task bootstrap    # 按锁定版本安装 golangci-lint、govulncheck
task fmt          # 格式化 Go 与 C#
task fmt:check    # 只读检查格式
task lint         # go vet、golangci-lint、dotnet build --warnaserror
task test         # go test -race、dotnet test
task check        # fmt + lint + test
task ci           # 只读门禁：fmt:check + lint + test，与 CI 相同
task vuln         # govulncheck 依赖漏洞扫描
task run:server   # 启动权威服务端
task run:client   # 打开 Godot 客户端工程
```

命令只在根目录 `Taskfile.yml` 定义，Windows、macOS、Linux 使用同一份定义，不经过平台专用脚本。`go test -race` 与 cgo 使用仓库固定的 LLVM 23.1.3：Windows 需要按 [LLVM 工具链](tools/llvm/README.md)设置 `LLVM_MINGW_ROOT`、`CC` 和 `CXX`；Linux/macOS 使用 `CC=clang`、`CXX=clang++`。

`.github/workflows/ci.yml` 在 Windows、macOS、Linux 上执行同一 `task ci`，先按 `packages.lock.json` 以 `--locked-mode` 还原 NuGet 依赖；仅修改文档时跳过完整构建。

版本锁定位置：

| 依赖 | 权威文件 |
| --- | --- |
| Go 版本与 Go 依赖 | `server/go.mod`、`server/go.sum` |
| .NET SDK | `global.json` |
| NuGet 包与 Godot SDK 包 | `client/packages.lock.json`、`client/src/Core/packages.lock.json`、`client/tests/Riftcards.Client.Core.Tests/packages.lock.json` |
| Godot SDK 与工程特性版本 | `client/Riftcards.Client.csproj`、`client/project.godot` |
| LLVM | `tools/llvm/VERSION` |
| Go Task、golangci-lint、govulncheck、最低工具版本 | `Taskfile.yml` |
| CI 中的 Go Task、LLVM-MinGW、action commit | `.github/workflows/ci.yml` |

服务端启动后监听 `http://127.0.0.1:8080`：

- 健康检查：`GET /healthz`
- WebSocket：`GET /ws`

客户端默认连接 `ws://127.0.0.1:8080/ws`，联机时会进入两人匹配队列。

## 核心不变量

1. 客户端不能直接修改权威状态。
2. 服务端先校验，再结算，再生成事件；拒绝的指令不推进版本号。
3. 每个已接受指令都是确定性、可序列化、可回放的事件批次。
4. 对手手牌、牌库内容等隐藏信息不会进入错误玩家的投影状态。
5. 协议中的时间、顺序、随机结果均以服务端为准。
6. 规则变化必须同时更新规则文档、协议文档和引擎测试。
