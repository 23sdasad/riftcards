# 012 — 删除 Taskfile 并切换直接命令

- 状态：done
- 依赖：011
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

需求方确认可以删除根目录 `Taskfile.yml`。本任务移除 Go Task 依赖，把 GitHub Actions 改为直接调用 Go 与 .NET 命令，并清理当前文档中的 Taskfile 引用。跨平台 shell 统一入口由 004 后续实现。

## 必读

[跨平台 Shell 指令进程](004-shell-run-entry.md) · [GitHub Actions CI](011-github-actions-ci.md) · [技术栈](../contracts/tech-stack.md) · [测试策略](../architecture/testing.md)。

## 范围与非目标

交付：

- 删除 `Taskfile.yml`，移除 Go Task 和 `setup-task` 依赖。
- 把 `.github/workflows/ci.yml` 改为直接执行 Go 与 .NET 格式、静态检查和测试命令。
- 更新 README、AGENTS、架构文档、技术栈和当前 task 中的命令示例。
- 保留跨平台命令语义；在 004 提供 shell 入口前，本地按文档直接执行各工具命令。

非目标：

- 不改变 Go/C# 检查口径。
- 不在本任务实现 004 的跨平台 shell 进程 API。
- 不引入 Just、Make、npm scripts 等替代任务器。

## 前置条件与待决策

- 当前主机缺少 Go、.NET SDK 和 Godot，无法本地运行完整门禁。
- CI 的最终三平台结果仍需推送后由 GitHub Actions 确认。

## 实施步骤

1. 删除 `Taskfile.yml`，盘点所有引用和命令。
2. 把 CI 拆成 Go 格式、Go lint/test、C# build/format/test 的直接步骤。
3. 更新当前文档和 task，检查没有失效的 Taskfile 调用。
4. 本地验证 YAML、链接、命令路径和差异；远端矩阵待推送后运行。

## 预计改动

- 删除：`Taskfile.yml`。
- 修改：`.github/workflows/ci.yml`、`README.md`、`AGENTS.md`、`ai-docs/README.md`、`ai-docs/architecture/conventions.md`、`ai-docs/architecture/testing.md`、`ai-docs/contracts/tech-stack.md`、`ai-docs/contracts/library-research.md`、`ai-docs/standards/commits.md`。
- 修改：当前执行 task 中的命令引用。

## 清理与兼容例外

删除 Taskfile、Go Task 安装步骤和所有当前文档中的旧命令入口。历史 task 保留当时记录，不把历史描述改写成未发生过的事。

## 验收标准

- [x] 仓库不再包含 `Taskfile.yml`，CI 不再依赖 Go Task。
- [x] CI 在 Windows、macOS、Linux 上直接执行同口径 Go/C# 检查。
- [x] 当前 README、AGENTS、架构、技术栈和规范无失效 Taskfile 命令。
- [x] 本地直接命令与 CI 步骤一致，路径跨平台。
- [x] 三平台远端结果待推送后确认，本地不伪造。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | workflow YAML 解析 | 结构有效 | 通过 |
| 2026-10-08 | Taskfile/Go Task 当前引用扫描 | 无失效调用 | 通过；仅历史 task 与本次删除记录保留旧名词 |
| 2026-10-08 | Markdown 链接与 `git diff --check` | 通过 | 通过 |
| — | GitHub Actions 三平台 | 全部门禁通过 | 推送后执行 |

## 风险与回退

若直接命令在某个平台不一致，回退到显式、无字符串插值的等价步骤，不恢复第二套任务器。

## 决策与工作记录

- 2026-10-08：从原 011 拆出。
- 2026-10-08：需求方确认可删除 Taskfile；任务改为删除并切换直接命令，统一 shell 入口留给 004。

## 完成摘要

已删除根目录 Taskfile，CI 改为直接执行 Go 与 .NET 命令，当前文档不再提供失效的 `task` 调用。限制：本机缺少 Go、.NET SDK 和 Godot，三平台实际运行仍需推送后由 GitHub Actions 确认。
