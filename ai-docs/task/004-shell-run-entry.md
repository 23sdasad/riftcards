# 004 — 跨平台 Shell 指令进程（已取消）

- 状态：cancelled
- 依赖：003
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

本任务原计划在 `Taskfile.yml` 已删除的假设下，用代码建立跨平台子进程 API（参数数组、环境、标准输入输出、退出码、取消、超时和子进程树清理）。需求方随后更正为保留根目录 `Taskfile.yml`（见 [012](012-taskfile-command-entry.md)）：跨平台命令入口、环境检查和运行前检查都由 Taskfile 交付，本任务取消，原范围未实施。

重启条件：只有业务代码真的需要以编程方式启动并管理子进程（取消、超时、进程树清理）时，才按 `_template.md` 新建 task；编号不复用。

## 必读

[技术栈](../contracts/tech-stack.md) · [Path 与工程边界](../architecture/paths-and-boundaries.md) · [测试策略](../architecture/testing.md) · [提交规范](../standards/commits.md)。

## 范围与非目标（原范围，未实施）

交付：

- 提供跨平台 shell/进程 API，覆盖可执行文件查找、参数数组、工作目录、环境变量、标准输入输出和退出状态。
- 支持取消和超时；终止进程时覆盖子进程树，不遗留孤儿进程。
- 默认不经系统 shell 解释字符串，避免平台分词、转义和注入差异；需要 shell 时必须显式选择。
- 新增环境检查任务，检查 `task`、Go、.NET SDK、Godot 和必需 Go 工具。
- 统一 `fmt`、`lint`、`test`、`check`、`run:server`、`run:client` 的进程启动和错误提示。
- 确认所有命令使用 003 的跨平台 path 能力，不拼接平台分隔符或可执行后缀。
- 更新 README 与 runbook 文档。

非目标：

- 不自动安装系统级 Godot 或 .NET SDK。
- 不改变 Go/C# 测试内容。
- 不提供 Godot 应用壳；该部分属于 008。

## 前置条件与待决策

- 需要至少一台安装 Go、.NET SDK、Godot 和 `task` 的验证环境，验证矩阵覆盖 Windows、macOS、Linux。
- 是否允许 `bootstrap` 自动安装仓库级工具由实施时决定；系统级 SDK 不自动安装。
- 进程 API 的具体位置和语言在开工前确认；不把同一套实现复制到多个端。
- 所有命令必须能在 PowerShell、POSIX shell 和 CI runner 下运行。

## 实施步骤

1. 盘点现有直接命令、路径参数、进程启动和文档引用。
2. 定义跨平台进程 API 与错误语义，覆盖参数、环境、I/O、退出码、取消和超时。
3. 实现最小 shell 层，并使用当前平台可执行测试助手验证正常、失败、取消和超时路径。
4. 让运行和验证任务复用同一进程与路径接口；避免用户手工拼接命令。
5. 更新 README、技术栈和 task 命令说明。
6. 在 Windows、macOS、Linux 环境运行 shell 命令列表、环境检查、格式化、lint、测试和服务端启动。

## 预计改动

- 新增：跨平台 shell/指令进程实现与测试。
- 修改：`README.md`、`ai-docs/contracts/tech-stack.md`、`ai-docs/architecture/testing.md`。
- 可能新增：进程 runbook 或主题文档。

## 清理与兼容例外

删除重复的路径拼接、平台专用命令分支和过期说明。保留跨平台命令边界；无法自动验证的平台需明确列出。

## 验收标准

- [ ] Windows、macOS、Linux 共用同一进程 API 语义，参数和路径不经过隐式 shell 分词。
- [ ] 正常退出、非零退出、标准错误、取消、超时和子进程终止都有测试。
- [ ] 缺少任一必要工具时，环境检查给出工具名、最低版本和安装入口。
- [ ] `fmt`、`lint`、`test`、`check` 均只引用权威项目路径。
- [ ] `run:server` 与 `run:client` 的前置检查失败时不会启动半个进程。
- [ ] README 与测试策略中的命令和 shell 实际入口一致。
- [ ] 在 Windows、macOS、Linux 完整环境执行全量门禁并记录结果。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| — | Windows / macOS / Linux shell 进程测试 | 行为一致 | 未执行 |
| — | shell 命令列表 | 命令清单完整 | 未执行 |
| — | 环境检查任务（正常与缺失工具路径） | 可判断、提示具体 | 未执行 |
| — | shell fmt/lint/test 入口 | 全部通过 | 未执行 |

## 风险与回退

风险是把平台差异藏进字符串命令或把安装逻辑塞进日常命令，导致环境漂移、注入或孤儿进程。回退时恢复显式参数数组和只读检查，把自动安装退回显式 `bootstrap`。

## 决策与工作记录

- 2026-10-08：创建任务。确认 shell 主要是跨平台指令进程层，覆盖 Windows、macOS、Linux，不处理 Godot 应用壳。
- 2026-10-08：需求方更正 Taskfile 保留；命令入口、环境检查与运行前检查改由根目录 `Taskfile.yml` 交付，本任务取消（见 012）。

## 完成摘要

已取消，未实施。统一命令入口、环境检查（`task env`）和运行前检查（`task run:*` 的前置条件）由恢复后的根目录 `Taskfile.yml` 提供，见 [012](012-taskfile-command-entry.md)。
