# Task 索引

采用“先写 task，再做实现”的工作方式：非平凡改动先从[模板](task/_template.md)建任务并在此登记，再动代码。任务详情是范围、决策与验证证据的主记录，索引只维护状态摘要，状态变更时两处一起更新。

## 使用方式

```text
ai-docs/
├── architecture/          # 架构总览与主题文档
├── contracts/             # 协议、技术栈与依赖调研
├── product/               # 玩法与产品边界
├── standards/             # 长期工程规范
├── task-index.md          # 本文件：任务队列与状态
└── task/
    ├── _template.md       # 创建任务时复制
    └── 001-riftcards-rename-task-workflow.md
```

1. 复制[模板](task/_template.md)为 `task/NNN-kebab-case.md`，编号取当前最大编号加一，不复用；填写范围与可判断的验收条件。
2. 在下方队列表登记，并核对依赖无环。
3. 开始实现时标为 `in-progress`，只做该任务范围；范围变化先改 task。
4. 每次 commit 更新 task 的工作记录与验证；全部验收有证据后，与索引一起标为 `done`。提交前检查见[提交规范](standards/commits.md)。

## 状态约定

| 状态 | 含义 |
| --- | --- |
| draft | 缺少范围或验收，不能开始 |
| planned | 已编排，依赖未满足 |
| ready | 可开始，尚无实现 |
| in-progress | 正在实施 |
| blocked | 有具体阻塞，已记录解除条件 |
| done | 验收完成且有证据 |
| deferred | 暂不排入 |
| cancelled | 已取消，保留编号与原因 |

## 任务队列

| 编号 | 任务 | 依赖 | 状态 |
| --- | --- | --- | --- |
| 001 | [riftcards 更名与 task 工作流基线](task/001-riftcards-rename-task-workflow.md) | — | done |
