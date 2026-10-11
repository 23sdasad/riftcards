# riftcards

riftcards 是一个两人回合制卡牌对战项目：

- 客户端：Godot 4 + C#
- 服务端：Go
- 结算：服务端权威
- 通信：WebSocket + JSON
- 客户端职责：发送指令、播放服务端事件、渲染按玩家裁剪后的状态
- 服务端职责：匹配、校验、结算、生成事件、广播回放、保存权威日志

当前仓库处于 Phase 1 完成状态：规则、协议、工程边界、工具链、服务端权威结算、Godot 可操作对局界面、事件队列与本地端到端验收都已建立（见[路线图](ai-docs/product/roadmap.md)）。

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
```

## 本地启动

三步：引导依赖 → 起服务端 → 起**两个**客户端（两人对局必须两个客户端，只开一个会停在匹配队列）。

```powershell
# 1. 首次引导（只需一次；此命令不依赖 task）
go -C tools/toolchain run . bootstrap
task env                                  # 确认 godot / dotnet / go 都已就绪

# 2. 启动权威服务端（默认 127.0.0.1:8080）
task run:server

# 3. 启动客户端：打开 Godot 编辑器，按 F5 运行
task run:client
```

两个客户端有两种起法：

```powershell
# A. 命令行起两个实例（推荐本地对测，不需要编辑器）
#    task env 会打印 Godot 的实际路径；Windows 是 .tools\bin\godot.cmd，macOS/Linux 是 .tools/bin/godot
.tools\bin\godot.cmd --path client -- --server=ws://127.0.0.1:8080/ws
.tools\bin\godot.cmd --path client -- --server=ws://127.0.0.1:8080/ws

# B. 编辑器里起两个实例：调试 → 运行多个实例（Godot 4.4+），或再打开一个编辑器窗口
```

两个客户端都点「连接」后会自动进入匹配队列；凑满两人即开局，先手由服务端决定。

### 客户端开发参数

在 `--` 之后传入（`godot --path client -- <参数>`），用于无人工操作的冒烟与截图：

| 参数 | 作用 |
| --- | --- |
| `--server=ws://host:port/ws` | 启动后自动连接（连接成功即自动 `hello` 并进入匹配） |
| `--smoke-end-turn` | 轮到自己时自动提交一次结束回合，便于观察真实事件顺序 |
| `--smoke-autoplay` | 自动出牌与攻击（走与点击相同的意图路径），用于冒烟和截图 |
| `--screenshot=<绝对路径>` | 等待指定时间后保存视口为 PNG 并退出 |
| `--screenshot-delay=<秒>` | 截图等待时间，默认 8 秒 |

```powershell
# 例：固定种子起服务端，两个自动出牌的客户端，其中一个 20 秒后截图
go -C server run ./cmd/riftcards-server -addr 127.0.0.1:8080 -seed 20261008
.tools\bin\godot.cmd --path client -- --server=ws://127.0.0.1:8080/ws --smoke-autoplay --screenshot=C:\tmp\board.png --screenshot-delay=20
.tools\bin\godot.cmd --path client -- --server=ws://127.0.0.1:8080/ws --smoke-autoplay
```

### 服务端参数

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `-addr` | `127.0.0.1:8080` | 监听地址；用 `127.0.0.1:0` 让系统选端口（真实地址打印在日志里） |
| `-log-level` | `info` | `debug`、`info`、`warn`、`error` |
| `-seed` | `0` | 固定每局随机种子，**仅供本地测试**；生产保持 `0`（每局随机） |

服务端启动后监听 `http://127.0.0.1:8080`：

`.tools/` 已被 git 忽略，删除它即可完全重来。`.tools/toolchain.env` 记录各依赖的实际路径，Taskfile 通过它把 `CC`、`CXX`、`DOTNET_ROOT`、`LLVM_MINGW_ROOT`、`LLVM_CLANG_CL` 指向仓库内工具链，因此不需要修改系统 PATH 或用户环境变量；`task` 本身安装在 `$(go env GOPATH)/bin`，若该目录不在 PATH，引导结束时会打印需要追加的路径。`go test -race` 在 Windows 上通过 [tools/llvm/clang-cl.cmd](tools/llvm/clang-cl.cmd) 使用固定的 LLVM 23.1.3 与 LLVM-MinGW；Linux/macOS 使用系统 `clang`。

`.github/workflows/ci.yml` 只保留 `setup-go`，随后执行同一个引导器和 `task ci`；仅修改文档时跳过完整构建。

依赖来源分工：

| 依赖 | 版本声明 | 获取方式 |
| --- | --- | --- |
| Go 版本与 Go 模块 | `server/go.mod`、`server/go.sum` | 开发者安装的 Go |
| Go Task、golangci-lint、govulncheck | `Taskfile.yml` | `go install` 到 GOPATH/bin 与 `.tools/bin` |
| .NET SDK | `Taskfile.yml`、`global.json` | 官方 SDK 压缩包 + SHA512 校验 |
| Godot（.NET 版） | `Taskfile.yml` | 官方 release 压缩包 + SHA256 校验 |
| Windows LLVM、LLVM-MinGW | `Taskfile.yml` | 官方 MSI 管理安装（`msiexec /a`）与官方 zip，均校验 SHA256 |
| NuGet 包与 Godot SDK 包 | `client/**/packages.lock.json` | `dotnet restore --locked-mode` |
| Godot SDK 与工程特性版本 | `client/Riftcards.Client.csproj`、`client/project.godot` | NuGet 还原 |

服务端启动后监听 `http://127.0.0.1:8080`：

- 健康检查：`GET /healthz`
- 运行时指标：`GET /metrics`（`name value` 每行一条，只做累加）
- WebSocket：`GET /ws`

客户端默认连接 `ws://127.0.0.1:8080/ws`，联机时会进入两人匹配队列。

## 客户端操作

界面分为应用壳、对局面板与事件面板：

- 应用壳：服务器地址、连接、进入匹配、会话状态。
- 对局面板：双方英雄（生命、费用、牌库、手牌数量）、对手场地、己方场地、手牌、结束回合与认输。
- 事件面板：按 `seq` 顺序播放的事件、待播放数量、缺帧与未知事件提示，以及「跳过动画」。

操作方式：

1. 点「连接」后自动发送 `hello` 并进入匹配；匹配成功后进入对局。
2. 单位牌：点手牌立即召唤。法术牌：点手牌后进入选择目标状态，再点任意单位或对手英雄。
3. 攻击：先点己方单位选择攻击者，再点对手单位或对手英雄。
4. 「结束回合」「认输」在允许时可用；非法操作由服务端拒绝，错误码显示在提示行。

客户端只提交意图：不计算费用、伤害、目标合法性或胜负。提示行显示服务端返回的错误码，快照校正后无效选择会被清理。

事件播放与权威状态分离：对局面板始终渲染最新快照；事件队列只负责按 `seq` 顺序驱动表现层，积压、缺帧或表现失败都不会阻塞对局，点「跳过动画」立即收敛到最新快照。

布局与视觉约定见[客户端界面](ai-docs/architecture/client-ui.md)。

## 本地端到端验收

`task ci` 会运行可重复的端到端测试：编译并启动真实服务进程，两个独立协议客户端完成一整局，断言投影、隐藏信息、版本、`seq`、终局结果与运行时指标，并确认测试结束后没有孤儿进程。单独运行：

```powershell
go -C server test ./internal/e2e
```

## 核心不变量

1. 客户端不能直接修改权威状态。
2. 服务端先校验，再结算，再生成事件；拒绝的指令不推进版本号。
3. 每个已接受指令都是确定性、可序列化、可回放的事件批次。
4. 对手手牌、牌库内容等隐藏信息不会进入错误玩家的投影状态。
5. 协议中的时间、顺序、随机结果均以服务端为准。
6. 规则变化必须同时更新规则文档、协议文档和引擎测试。
