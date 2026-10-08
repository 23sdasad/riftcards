# 001 — riftcards 更名与 task 工作流基线

- 状态：done
- 依赖：无
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

仓库原有项目标识与新名称不一致。本次将产品名统一为 `riftcards`，并同步 Go module、服务入口、C# 项目与命名空间、Godot 配置和文档路径。同时移植参考模板的 task 工作流：非平凡改动先登记 task，每个 commit 记录任务编号、验证结果和清理项。

## 必读

[AI 文档索引](../README.md) · [架构约定](../architecture/conventions.md) · [技术栈](../contracts/tech-stack.md) · [提交规范](../standards/commits.md) · [测试策略](../architecture/testing.md)。

参考：[bun-vue-tauri-template](https://github.com/Yuki-Nagori/bun-vue-tauri-template) 的 task-index、task 模板与提交规范，查阅日期 2026-10-08。

## 范围与非目标

交付：清除受管文件、路径和构建标识中的旧项目名；使用 `riftcards` 作为产品与仓库 slug，使用 `Riftcards` 作为 C# / .NET 标识；建立 task 索引、task 模板、提交规范和对应文档接线。

非目标：不修改游戏规则、协议语义、卡牌数据或权威状态模型；不引入参考模板中的 Bun、Vue、Rust 或 Tauri 技术栈。

## 前置条件与待决策

- 决策：产品显示名使用小写 `riftcards`，Go module、目录和命令沿用小写；C# 命名空间、项目名和程序集遵循 .NET 惯例使用 `Riftcards`。
- 决策：task 文档只移植工作流、状态和提交约束，测试命令沿用本仓库的 `task fmt`、`task lint`、`task test`。

## 实施步骤

1. 重命名 Go 服务入口、C# 项目和客户端测试项目，更新所有引用路径。
2. 替换 Go module、C# 命名空间、Godot 显示名与程序集名、README 和长期文档中的旧标识。
3. 新增 `ai-docs/task-index.md`、`ai-docs/task/_template.md`、`ai-docs/standards/`，并接入 `AGENTS.md` 与 `ai-docs/README.md`。
4. 全仓复查旧标识、执行格式化、静态检查和测试，登记验证证据后完成提交。

## 预计改动

修改：`Taskfile.yml`、`README.md`、`AGENTS.md`、Go module 与导入、客户端项目文件、源码命名空间、Godot 配置及相关 ai-docs。重命名：旧服务入口目录、客户端三个旧项目文件及旧测试目录。新增：task 与 standards 文档。

## 清理与兼容例外

删除全部受管目录、文件名和文本中的旧项目标识。无兼容例外；项目尚无外部发布与持久化契约。

## 验收标准

- [x] 受管文件内容、文件名和目录名不再包含旧项目标识，旧标识扫描无结果。
- [x] Taskfile 项目路径均存在，C# 项目 XML 与项目引用解析通过。
- [x] task 索引、模板和提交规范可从 README、AGENTS 与 ai-docs 索引到达，相对链接检查通过。
- [x] 已尝试执行 `task fmt`、`task lint`、`task test`；本机缺失的 `task`、Go、.NET SDK 和 Godot 已在验证结果中记录。
- [x] 提交消息包含 `Task: 001`、`Validation:` 和 `Cleanup:`。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | `rg` 全仓旧标识检查 | 无受管引用 | 通过，无结果 |
| 2026-10-08 | Markdown 相对链接检查 | 全部可解析 | 通过 |
| 2026-10-08 | C# 项目 XML 与项目引用检查 | 路径有效 | 通过 |
| 2026-10-08 | `git diff --check` | 无空白错误 | 通过 |
| 2026-10-08 | `task fmt`、`task lint`、`task test` | 完成格式化、静态检查与测试 | 未执行：本机缺少 `task`、Go 和 Godot；.NET 仅有运行时，无 SDK |
| 2026-10-08 | `dotnet format`、`dotnet build`、`dotnet test` | 完成 .NET 侧检查 | 未执行：命令返回“No .NET SDKs were found” |

## 风险与回退

主要风险是 Godot 项目文件、C# 程序集名和项目引用不一致导致客户端无法加载。回退时统一恢复旧路径、命名空间与程序集名，并将 task 标为 `blocked` 或 `cancelled`，保留原因。

## 决策与工作记录

- 2026-10-08：创建任务。确认以参考模板的 task-index、task 模板和提交规范为基线，适配本仓库 Go + Godot/C# 工具链。
- 2026-10-08：完成项目标识替换、路径重命名与 task 工作流文档接线；静态一致性检查通过，项目级工具链检查因本机环境缺失未执行。

## 完成摘要

项目名、路径、Go module、C# 标识、Godot 配置和 task 工作流已完成统一。旧标识扫描、Markdown 链接、C# 项目引用和差异空白检查通过；本机缺少 `task`、Go、.NET SDK 和 Godot，因此格式化、lint 与测试未运行，未将项目级检查描述为已验证。
