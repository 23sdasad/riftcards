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
    ├── 001-riftcards-rename-task-workflow.md
    ├── 002-phase-1-playable-match-plan.md
    ├── 003-path-boundaries.md
    ├── 004-shell-run-entry.md
    ├── 005-match-turn-state-machine.md
    ├── 006-command-path-error-boundaries.md
    ├── 007-rule-protocol-validation.md
    ├── 008-godot-match-ui.md
    ├── 009-event-queue-presentation.md
    ├── 010-local-end-to-end.md
    ├── 011-github-actions-ci.md
    ├── 012-taskfile-command-entry.md
    ├── 013-toolchain-gate-repair.md
    ├── 014-go-toolchain-bootstrap.md
    └── 015-windows-llvm-msi.md
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
| 002 | [Phase 1 本地完整对局：实施拆分与交互决策](task/002-phase-1-playable-match-plan.md) | 001 | done |
| 003 | [跨平台 Path 与工程边界基线](task/003-path-boundaries.md) | 002 | done |
| 004 | [跨平台进程 API](task/004-shell-run-entry.md) | 003 | done |
| 005 | [对局与回合状态机](task/005-match-turn-state-machine.md) | 003 | done |
| 006 | [指令处理路径与错误边界](task/006-command-path-error-boundaries.md) | 005 | ready |
| 007 | [规则与协议验证基线](task/007-rule-protocol-validation.md) | 006 | planned |
| 008 | [Godot 对局界面纵切](task/008-godot-match-ui.md) | 007 | planned |
| 009 | [事件队列与表现层解耦](task/009-event-queue-presentation.md) | 008 | planned |
| 010 | [本地端到端验收](task/010-local-end-to-end.md) | 004、007、008、009 | planned |
| 011 | [GitHub Actions CI](task/011-github-actions-ci.md) | 001 | done |
| 012 | [统一 Taskfile 命令入口与版本锁定](task/012-taskfile-command-entry.md) | 011 | done |
| 013 | [修复最新工具链下的本地门禁](task/013-toolchain-gate-repair.md) | 003、012 | done |
| 014 | [用 Go 引导全部依赖](task/014-go-toolchain-bootstrap.md) | 012 | done |
| 015 | [Windows LLVM 改用官方 MSI 管理安装](task/015-windows-llvm-msi.md) | 014 | done |
