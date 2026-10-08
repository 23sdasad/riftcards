# 011 — GitHub Actions CI

- 状态：done
- 依赖：001
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

仓库目前没有 GitHub Actions，格式化、静态检查和测试只能在本地人工执行。考虑到 path 与 shell 以 Windows、macOS、Linux 为目标，本任务先建立三平台 CI，让后续每个 task 的提交都有统一验证入口。

## 必读

[技术栈](../contracts/tech-stack.md) · [测试策略](../architecture/testing.md) · [提交规范](../standards/commits.md)。

## 范围与非目标

交付：

- 新增 `.github/workflows/ci.yml`。
- 在 Windows、macOS、Linux 上执行 Go 与 C# 格式、静态检查和测试。
- CI 快门禁前置，耗时步骤后置；文档-only 改动不触发完整构建。
- 配置最小权限、并发取消、超时和依赖缓存。
- 更新 README 与技术文档中的 CI 说明。

非目标：

- 不移动或重命名 `Taskfile.yml`；该校正由 012 处理。
- 不改变 `fmt`、`lint`、`test` 的检查口径。
- 不引入 Godot 图形化端到端测试；由 010 处理。

## 前置条件与待决策

- 当前 Taskfile 位于仓库根目录，CI 暂按其默认发现规则调用。
- GitHub Actions runner 矩阵固定为 `ubuntu-latest`、`windows-latest`、`macos-latest`。
- 首次 workflow 必须在远端实际运行后才能判定 CI 通过。

## 实施步骤

1. 新增三平台 workflow，安装 Go、.NET SDK 和 Go Task。
2. 安装门禁所需 Go 工具，并执行 Go/C# 格式检查和静态检查。
3. 执行 Go 与 C# 测试，设置缓存、超时和取消策略。
4. 更新 README、技术栈和测试策略中的 CI 说明。
5. 检查 YAML、Markdown 链接和差异空白；提交后由远端运行验证。

## 预计改动

- 新增：`.github/workflows/ci.yml`。
- 修改：`README.md`、`ai-docs/contracts/tech-stack.md`、`ai-docs/architecture/testing.md`。
- 修改：`ai-docs/task-index.md`。

## 清理与兼容例外

不删除本地门禁命令。CI 必须调用同一 Taskfile 和同一工具链版本，不维护第二套脚本。

## 验收标准

- [x] `.github/workflows/ci.yml` 语法有效，触发条件和权限范围明确。
- [x] Windows、macOS、Linux 均执行 Go 与 C# 格式、静态检查和测试。
- [x] CI 使用并发取消、超时和依赖缓存，文档-only 改动跳过完整构建。
- [x] 本地命令与 CI 使用同一 `Taskfile.yml` 和版本口径。
- [x] 已明确记录首次三平台运行需在推送后确认，本地不伪造远端结果。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | workflow YAML 解析 | 结构有效 | 通过 |
| 2026-10-08 | Taskfile 引用检查 | `bootstrap`、`lint`、`test` 存在 | 通过 |
| 2026-10-08 | Markdown 链接和 `git diff --check` | 通过 | 通过 |
| — | GitHub Actions 三平台 workflow | 全部门禁通过 | 未执行；首次运行需推送后由 GitHub 确认 |

## 风险与回退

风险是三平台工具版本、缓存路径或 Go race test 在某个 runner 上不一致。回退时保留单平台或拆出平台专用步骤，但不得降低默认门禁；失败日志必须能直接定位到 Task 和平台。

## 决策与工作记录

- 2026-10-08：创建任务。确认 CI 文件路径为 `.github/workflows/ci.yml`，三平台运行同一门禁。
- 2026-10-08：后续需求确认可删除 Taskfile；CI 的直接命令迁移由 012 接管。
- 2026-10-08：需求方更正 Taskfile 保留。CI 回到同一 `Taskfile.yml` 入口（action 按 SHA 固定、`task bootstrap` + `task ci`），并修复 LLVM 安装步骤；见 [012](012-taskfile-command-entry.md)。

## 完成摘要

已新增三平台 GitHub Actions workflow，覆盖 Go 格式检查、lint 和测试，本地结构检查通过。限制：当前未推送，三平台实际运行结果尚待 GitHub Actions 首次执行。
