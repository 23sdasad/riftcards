# Path 与工程边界

本文是仓库路径语义、关键目录、依赖方向和禁止路径的唯一事实来源。系统组件与数据流见[架构总览](overview.md)；本文只补充路径规则和边界检查方式。

## 当前实现与目标边界

当前实现：

- `server/internal/platform/pathutil` 提供与运行主机无关的 Windows、macOS、Linux 路径解析、规范化、拼接、比较和词法 confinement。
- `server/internal/platform/proc` 是代码层的进程适配器：用 `pathutil` 解析可执行文件，按参数数组启动子进程，并负责工作目录、环境、标准输入输出、退出状态、取消、超时和进程树终止。它只服务工具与端到端测试。
- 仓库命令入口是根目录 `Taskfile.yml`；全部依赖的版本声明也集中在该文件的 `vars:`。
- 依赖由独立 Go 模块 `tools/toolchain` 引导到仓库内 `.tools/`；`tools/toolchain` 不依赖 `server` 模块，也不使用 `proc`。

目标边界：

- 用户输入路径、配置路径、构建产物路径和可执行文件名必须先经过 `pathutil`，不能依赖宿主机的 `filepath` 或字符串拼接。
- 只有真正访问本机文件系统的适配层可以调用 Go 标准库 `os`、`path/filepath`；该适配层仍必须接收显式目标系统或使用已经解析的规范化路径。
- C# 客户端当前不需要平台文件路径 API。若未来需要，不得复制 Go 语义；应先定义跨端行为并复用同一路径契约。

## 目标系统语义

`pathutil.OS` 必须显式传入，不能让路径结果随构建机或运行机变化。

| 目标 | 根路径 | 规范分隔符 | 输入兼容 | 默认大小写 | 可执行名 |
| --- | --- | --- | --- | --- | --- |
| Windows | 驱动器、UNC、根相对 | `\` | `/` 与 `\` | 不敏感 | 缺少扩展名时补 `.exe` |
| macOS | POSIX 根 | `/` | 仅 `/` | 不敏感 | 原样 |
| Linux | POSIX 根 | `/` | 仅 `/` | 敏感 | 原样 |

补充规则：

- macOS 默认大小写不敏感是 API 的默认比较策略，不表示所有卷都无法配置为大小写敏感。
- Windows 支持驱动器绝对路径（`C:\dir`）、驱动器相对路径（`C:dir`）、根相对路径（`\dir`）和 UNC 路径（`\\server\share\dir`）。
- Windows 设备路径（例如 `\\?\C:\dir`）和格式不完整的 UNC 路径返回错误，不由业务层猜测。
- 规范化会折叠空段和 `.`；绝对路径在根处截断多余 `..`，相对路径保留无法消解的起始 `..`。

## pathutil API

位置：`server/internal/platform/pathutil`。该包只依赖 Go 标准库，不依赖 `game`、`match`、`protocol`、WebSocket、Godot 或进程层。

| API | 作用 | 关键错误 |
| --- | --- | --- |
| `Parse` / `Normalize` | 按目标系统校验并得到规范文本 | `ErrInvalidOS`、`ErrInvalidPath` |
| `Join` | 拼接相对后缀并规范化 | `ErrAbsoluteComponent` |
| `IsAbs` / `Equal` | 判断绝对性并按目标系统比较 | `ErrInvalidOS`、`ErrInvalidPath` |
| `Within` | 词法判断绝对 candidate 是否位于绝对 base 内 | `ErrBaseNotAbsolute` |
| `ExecutableName` | 返回目标系统的可执行文件名 | `ErrInvalidPath` |
| `UserHome` / `ExpandUser` | 从显式环境读取并展开用户目录 | `ErrMissingHome`、`ErrUnsupportedHome` |

`Within` 只做词法 confinement，不解析符号链接，也不检查真实文件系统。安全敏感的文件访问必须在解析符号链接后再次验证真实路径，不能把字符串前缀判断当作最终安全边界。

## 仓库关键路径

下列路径从仓库根目录解析；命令默认从根目录执行。

| 类别 | 权威路径 | 约定 |
| --- | --- | --- |
| 入口文档 | `README.md`、`AGENTS.md` | 人和自动化代理的仓库入口 |
| 命令入口 | `Taskfile.yml`、`global.json` | 统一 `task` 命令入口、依赖声明与 .NET SDK 锁定 |
| 依赖引导 | `tools/toolchain/`、`.tools/` | Go 引导器（独立模块）与它安装的本地依赖（`.tools/` 由 git 忽略） |
| AI 事实来源 | `ai-docs/` | 长期规则、协议、架构、规范和 task |
| 服务端 | `server/go.mod`、`server/cmd/riftcards-server/`、`server/internal/` | Go 模块、进程入口、内部模块 |
| 平台适配 | `server/internal/platform/pathutil/`、`server/internal/platform/proc/` | 跨平台路径语义与进程启动/终止；只有适配层可以使用它们 |
| 客户端 | `client/Riftcards.Client.csproj`、`client/src/`、`client/tests/`、`client/**/packages.lock.json` | Godot 工程、Core/Godot 分层、纯 .NET 测试和 NuGet 锁定 |
| LLVM 工具链 | `tools/llvm/`、`.clang-format`、`.clang-tidy` | 固定编译器版本、Windows `clang-cl` 入口和检查规则 |
| CI | `.github/workflows/ci.yml` | Windows、macOS、Linux 执行同一 `task ci` 门禁 |

生成或机器相关目录不得作为源码事实来源，也不得提交：

| 类别 | 路径 |
| --- | --- |
| Godot 导入与导出 | `.godot/`、`client/.godot/`、`client/export/` |
| .NET 构建与测试 | `**/bin/`、`**/obj/`、`TestResults/`、`coverage/` |
| Go 构建与覆盖率 | `server/bin/`、`server/dist/`、`server/coverage.out` |
| 本地临时与秘密 | `.env*`、`*.local.json`、`tmp/`、`temp/` |
| 引导安装的依赖 | `.tools/`（可删除后由 `task bootstrap` 重建） |

具体忽略规则由根目录 `.gitignore` 维护；本文只说明路径类别，不复制通配符清单。

## 单向数据路径

客户端意图到权威状态只能沿以下方向前进：

```text
Godot 输入
  -> C# 构造 command DTO（commandId、expectedRevision）
  -> WebSocket 文本帧
  -> transport/ws 解码并确认连接
  -> match 定位房间并串行处理
  -> game 校验回合、费用、目标、手牌和修订号
  -> 接受：修改权威状态，递增 revision，生成连续 seq 事件
     或拒绝：不修改状态，不推进 revision / seq
  -> match 按玩家投影事件与 MatchView
  -> WebSocket 广播
  -> C# 记录事件、播放表现，并以 MatchView 校正缓存
  -> Godot 渲染
```

每段只承担一种职责：

- Godot 只收集输入和渲染，不决定规则结果。
- C# Core 只构造意图、解析响应、维护缓存和回放游标。
- `transport/ws` 只处理连接、帧和 DTO 编解码。
- `match` 只负责房间、串行化和广播，不实现卡牌规则。
- `game` 只在服务端校验和推进权威状态，不依赖网络或表现层。

客户端缓存、动画队列和 UI 状态都不能作为后续指令的合法性依据。

## 允许依赖与禁止路径

允许依赖：

```text
Godot -> Core
cmd -> transport/ws -> match -> protocol -> game
cmd -> match
server 各适配层 -> pathutil
server 各适配层 -> proc -> pathutil
pathutil -> Go 标准库
tools/toolchain -> Taskfile.yml（只读取依赖声明）
```

禁止路径：

| 禁止行为 | 判断方式 |
| --- | --- |
| 客户端计算最终结算或直接修改权威状态 | C# 中查找游戏规则分支、状态变更 API 或把本地预测当提交结果 |
| `game` 依赖网络、子进程、环境或 Godot 概念 | 检查 `server/internal/game` 的 import 和参数类型 |
| 规则或传输层依赖 `platform/proc` 或 `os/exec` | 检查 `server/internal/{game,match,protocol}` 的 import |
| 业务代码手工拼接 `/`、`\` 或平台可执行后缀 | 扫描 `filepath.Join`、`Path.Combine`、`DirectorySeparatorChar`、手写 `.exe` 和字符串分隔符 |
| 传输 DTO 层启动 I/O、房间或进程 | 检查 `server/internal/protocol` 是否导入 `net/http`、WebSocket、`os/exec` 或 `match` |
| 向所有玩家广播未投影的完整事件或隐藏区域 | 检查广播路径与房间测试，确认对手手牌、牌库和实例 ID 不泄露 |

路径改动提交前至少执行：

```powershell
task test
go -C server test ./internal/platform/pathutil ./internal/platform/proc
git diff --check
```

Windows、macOS、Linux 的最终一致性以 `.github/workflows/ci.yml` 的三平台结果为证据；本机缺少 Go 或 .NET SDK 时必须在 task 中记录未执行项。
