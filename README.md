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
├─ tools/                   # 仓库工具
│  ├─ toolchain/            # 用 Go 获取全部依赖（独立 Go 模块）
│  └─ llvm/                 # LLVM 版本文件、clang-cl 包装器与说明
├─ .tools/                  # 引导器安装的本地依赖（git 忽略，可随时删除重装）
└─ AGENTS.md                # 后续 AI/自动化代理必须遵守的入口
```

## 依赖与本地命令

全部依赖的版本、下载地址和校验哈希只在 [Taskfile.yml](Taskfile.yml) 的 `vars:` 声明一次。**开发者只需要本机有 Go**，其余依赖（Go Task、golangci-lint、govulncheck、.NET SDK、Godot、Windows LLVM 与 LLVM-MinGW、NuGet 包）由 [tools/toolchain](tools/toolchain) 用 Go 获取到仓库内 `.tools/`：

```powershell
go -C tools/toolchain run . bootstrap   # 首次引导；此命令不依赖 task
task env                                # 检查依赖与版本，列出缺失项和安装入口
task check                              # 本地完整门禁
```

引导完成后即可使用统一入口（Windows、macOS、Linux 同一份定义）：

```powershell
task bootstrap    # 与首次引导等价；可用 task bootstrap -- --without godot 跳过组件
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

`.tools/` 已被 git 忽略，删除它即可完全重来。`.tools/toolchain.env` 记录各依赖的实际路径，Taskfile 通过它把 `CC`、`CXX`、`DOTNET_ROOT`、`LLVM_MINGW_ROOT`、`LLVM_CLANG_CL` 指向仓库内工具链，因此不需要修改系统 PATH 或用户环境变量；`task` 本身安装在 `$(go env GOPATH)/bin`，若该目录不在 PATH，引导结束时会打印需要追加的路径。`go test -race` 在 Windows 上通过 [tools/llvm/clang-cl.cmd](tools/llvm/clang-cl.cmd) 使用固定的 LLVM 23.1.3 与 LLVM-MinGW；Linux/macOS 使用系统 `clang`。

`.github/workflows/ci.yml` 只保留 `setup-go`，随后执行同一个引导器和 `task ci`；仅修改文档时跳过完整构建。

依赖来源分工：

| 依赖 | 版本声明 | 获取方式 |
| --- | --- | --- |
| Go 版本与 Go 模块 | `server/go.mod`、`server/go.sum` | 开发者安装的 Go |
| Go Task、golangci-lint、govulncheck | `Taskfile.yml` | `go install` 到 GOPATH/bin 与 `.tools/bin` |
| .NET SDK | `Taskfile.yml`、`global.json` | 官方 SDK 压缩包 + SHA512 校验 |
| Godot（.NET 版） | `Taskfile.yml` | 官方 release 压缩包 + SHA256 校验 |
| Windows LLVM、LLVM-MinGW | `Taskfile.yml` | 官方 release 压缩包 + SHA256 校验 |
| NuGet 包与 Godot SDK 包 | `client/**/packages.lock.json` | `dotnet restore --locked-mode` |
| Godot SDK 与工程特性版本 | `client/Riftcards.Client.csproj`、`client/project.godot` | NuGet 还原 |

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
