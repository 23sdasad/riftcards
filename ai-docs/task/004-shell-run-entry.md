# 004 — 跨平台进程 API

- 状态：done
- 依赖：003
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

原范围把本任务写成“统一命令入口 + 依赖安装”，这部分已经由 [Taskfile.yml](../../Taskfile.yml) 与 [tools/toolchain](../..//tools/toolchain)（task 012、014）交付：开发者用 `task` 获得跨平台命令，用 Go 引导器获得全部依赖。因此本任务重新界定为**代码层的进程 API**。

仍然缺的能力是：Go 代码需要以编程方式启动并管理子进程，用于本地端到端验收（010 需要拉起服务端与两个客户端）、回放/诊断工具和后续测试夹具。当前仓库里 `server/internal/platform/pathutil` 已经提供 `ExecutableName`、`Join`、`Within` 等跨平台语义，但没有任何生产代码使用它；进程层正是它的第一个消费者。

## 必读

[Path 与工程边界](../architecture/paths-and-boundaries.md) · [测试策略](../architecture/testing.md) · [技术栈](../contracts/tech-stack.md) · [提交规范](../standards/commits.md)。

## 范围与非目标

交付：

- 新增 `server/internal/platform/proc`，提供进程启动与生命周期管理：
  - 可执行文件查找：用 `pathutil.ExecutableName` 补平台后缀、用 `pathutil.Join` 拼接、按显式传入的附加目录与 `PATH` 顺序查找。
  - 参数数组：不经过系统 shell，不做字符串分词、转义或插值。
  - 工作目录、环境变量（继承或整体替换）、标准输入输出与标准错误、退出状态。
  - 取消与超时：`context` 驱动；超时或被取消时终止整棵子进程树，不遗留孤儿进程。
  - 标准输出/标准错误在未显式接管时被有上限地捕获，超限截断并在结果中标记。
- 为正常退出、非零退出、标准错误、标准输入、取消、超时和进程树终止添加测试；测试用测试二进制自身作为子进程助手，不依赖外部工具。
- 在文档中固定边界：只有 `platform` 适配层可以使用该包，`game`、`match`、`protocol` 不得依赖。

非目标：

- 不再承担“统一命令入口”和“依赖安装”：前者是 `Taskfile.yml`，后者是 `tools/toolchain`。
- 不实现 shell 解析、管道编排或作业调度器。
- 不自动安装外部工具；找不到可执行文件时返回带工具名与查找路径的错误。
- 不改变现有 `task` 命令语义。

## 前置条件与待决策

- 包位置：`server/internal/platform/proc`，与 `pathutil` 同级，保持“平台适配层”边界。
- Windows 进程树终止：使用系统自带的 `taskkill /T /F`；Unix 使用进程组（`Setpgid`）后按组终止。不引入 `golang.org/x/sys` 依赖。
- 环境变量语义：`Spec.Env == nil` 表示继承父进程环境；非 nil 表示整体替换，避免“继承一半”的歧义。

## 实施步骤

1. 定义 `proc.Spec`、`proc.Result` 与错误类型，写清参数、目录、环境、I/O、退出码、取消与超时语义。
2. 实现可执行文件查找，复用 `pathutil` 的平台后缀与路径拼接。
3. 实现启动、捕获、超时与取消，并实现进程树终止。
4. 用测试二进制助手覆盖正常、失败、标准错误、标准输入、取消、超时和进程树场景。
5. 更新路径边界、技术栈和 task 文档，运行 `task ci` 并推送确认三平台。

## 预计改动

- 新增：`server/internal/platform/proc/proc.go`、`server/internal/platform/proc/proc_test.go`。
- 修改：`ai-docs/architecture/paths-and-boundaries.md`、`ai-docs/contracts/tech-stack.md`、`AGENTS.md`、`ai-docs/task/010-local-end-to-end.md`、`ai-docs/task-index.md`。

## 清理与兼容例外

删除文档中“业务代码尚未引入 shell 进程层”的旧表述，改为明确允许 `platform/proc` 且禁止规则层依赖。不保留平台专用的命令字符串分支。

## 验收标准

- [x] 可执行文件查找使用 `pathutil` 的平台后缀与拼接，不手写 `.exe` 或平台分隔符（`LookPath` 先 `pathutil.ExecutableName` 再 `pathutil.Join`）。
- [x] 命令只以参数数组传递，不经过系统 shell（`exec.CommandContext` 直接使用 `Args`）。
- [x] 正常退出、非零退出、标准错误、标准输入、取消、超时、进程树终止都有测试。
- [x] 取消或超时后不遗留子进程；结果中可区分 `Canceled` 与 `TimedOut`。
- [x] `game`、`match`、`protocol` 不依赖 `proc`（依赖方向写入边界文档与 AGENTS）。
- [x] `task ci` 与三平台 CI 通过（run 37824684913 已包含本包，三平台全绿）。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | Windows / `go vet ./internal/platform/proc` | 无问题 | 通过 |
| 2026-10-08 | Windows / `go test ./internal/platform/proc` | 全部场景通过 | 通过：参数数组/工作目录/环境、非零退出与标准错误、标准输入、超时、取消、孙进程随进程树终止、输出截断、`LookPath`（平台后缀、拒绝带分隔符名字、`ErrNotFound`）、`HostOS` 与运行时一致 |
| 2026-10-08 | Windows / `task ci` | 门禁通过 | 通过：`proc` 在 `-race` 下 10.45s 通过；其余 Go/C# 门禁保持绿色 |
| 2026-10-08 | GitHub Actions run [37824684913](https://github.com/23sdasad/riftcards/actions/runs/37824684913) | 三平台通过 | 通过：ubuntu 2m38s、windows 4m56s、macos 3m1s，`task ci` 含 `proc` 的 race 测试 |

## 风险与回退

主要风险是 Windows 与 Unix 的终止语义不同，以及捕获大量输出导致内存增长。回退时保留“参数数组 + 退出码 + 超时”最小集合，把进程树终止退化为“仅终止直接子进程”，并在文档中记录该限制。

## 决策与工作记录

- 2026-10-08：创建任务。当时假设 Taskfile 已删除，范围是“跨平台命令入口 + 依赖安装”。
- 2026-10-08：需求方更正 Taskfile 保留（012），命令入口与依赖安装分别由 Taskfile 和 014 的引导器交付，本任务被标记 cancelled。
- 2026-10-08：需求方指出代码层进程 API（可执行文件查找、参数数组、工作目录、环境变量、标准输入输出、退出状态）仍然需要，且 `pathutil` 目前没有生产消费者。本任务恢复并重新界定为 `server/internal/platform/proc`，不再承担命令入口与依赖安装。

## 完成摘要

新增 `server/internal/platform/proc`：`LookPath` 用 `pathutil` 补平台后缀并按附加目录与 `PATH` 顺序查找；`Run` 以参数数组启动进程，支持工作目录、继承或替换的环境、标准输入输出、有上限的输出捕获、退出状态，以及基于 `context` 的取消与超时。取消或超时时终止整棵进程树：Unix 用进程组 `SIGKILL`，Windows 用 `taskkill /T /F`；结果区分 `Canceled` 与 `TimedOut`。测试用测试二进制自身作为子进程助手，覆盖正常、失败、标准错误、标准输入、取消、超时、孙进程清理与输出截断。本机 `task ci`（含 `-race`）通过；三平台 CI 待推送确认。命令入口与依赖安装仍由 `Taskfile.yml` 与 `tools/toolchain` 负责，不在本包范围内。
