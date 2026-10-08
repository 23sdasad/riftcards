# 提交规范

更新日期：2026-10-08。均为项目约定，不依赖特定 Git 客户端。

## 一个 commit 是一个可验证的行为单元

每次 commit 表达一个清楚的目的，包含实现它所需的代码、测试、配置、文档与 task 更新；不提交已知损坏的中间状态。允许按可构建、可验证的纵向小步骤提交，每一步上已有功能继续成立。协议或跨端契约变更时，同一 commit 更新全部消费方与对应验证。

提交前检查：

- 行为与标题一致，替换实现时旧代码、调用点、依赖和文档一并清理。
- 在最终将提交的状态上运行 `task fmt`、`task lint`、`task test` 并记录结果。
- 受影响的 task 与 `ai-docs/task-index.md` 在同一 commit 更新。
- 暂存区没有无关改动、临时文件或机器特定产物。

## 消息格式

```text
<type>(<scope>): <具体行为>

说明触发条件、结果与必要取舍；简单变更可省略。

Task: NNN
Validation: 实际检查及结果；不能只写 tested
Cleanup: 删除项，或写无废弃项
```

正文可中文，`type` / `scope` 使用简短英文。`type` 取 `feat`、`fix`、`refactor`、`cleanup`、`build`、`test`、`docs`、`ci`、`chore`、`revert`；`scope` 使用实际模块名，例如 `server`、`client`、`protocol`、`docs`、`repo`。不用 `update`、`misc`、`WIP` 描述行为；破坏既有接口时标题冒号前加 `!` 并说明迁移影响。

## Task 状态同步

开始实现时把 task 标为 `in-progress`；中间 commit 记录本次完成、验证和剩余工作，只勾选有证据的验收项；完成时补齐摘要，并将 task 与 `ai-docs/task-index.md` 一起标为 `done`。一个 commit 原则上对应一个 task；跨任务的不可分割改动须列出全部关联编号及原因。
