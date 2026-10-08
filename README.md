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
├─ server/                  # Go 权威服务端
│  ├─ cmd/riftcards-server/ # 服务入口
│  └─ internal/             # game / match / protocol / transport
├─ client/                  # Godot C# 客户端
│  ├─ src/Core/             # 与 Godot 解耦的协议和会话逻辑
│  ├─ src/Godot/            # Godot WebSocket 适配层
│  └─ tests/                # 纯 .NET 单元测试
├─ .github/workflows/       # GitHub Actions 三平台 CI
└─ AGENTS.md                # 后续 AI/自动化代理必须遵守的入口
```

## 本地命令

工具链版本见 [技术栈](ai-docs/contracts/tech-stack.md)。首次使用先安装 Go、.NET SDK 和带 .NET 支持的 Godot；跨平台统一 shell 由 task 004 提供，当前按以下直接命令执行。

```powershell
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6
go install golang.org/x/vuln/cmd/govulncheck@latest

gofmt -w server/cmd server/internal
dotnet format client/src/Core/Riftcards.Client.Core.csproj
dotnet format client/Riftcards.Client.csproj
dotnet format client/tests/Riftcards.Client.Core.Tests/Riftcards.Client.Core.Tests.csproj

go -C server vet ./...
go -C server run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6 run ./...
dotnet build client/src/Core/Riftcards.Client.Core.csproj --warnaserror
dotnet build client/Riftcards.Client.csproj --warnaserror

go -C server test -race ./...
dotnet test client/tests/Riftcards.Client.Core.Tests/Riftcards.Client.Core.Tests.csproj

go -C server run ./cmd/riftcards-server -addr 127.0.0.1:8080
godot --path client --editor
```

`.github/workflows/ci.yml` 在 Windows、macOS、Linux 上直接运行同一组 Go 与 .NET 门禁；仅修改文档时跳过完整构建。

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
