# AI 文档索引

`ai-docs` 同时面向后续接手项目的 AI/自动化代理和维护者，集中存放玩法、架构、协议和任务事实。文档按 `product`、`architecture`、`contracts`、`standards`、`task` 分组，出现冲突时按以下优先级处理：

1. 当前代码与测试体现的可执行契约
2. `product/game-rules.md` 和 `contracts/protocol.md`
3. `ai-docs` 中的架构和约定
4. 临时任务描述

如果第 1 项与第 2、3 项不一致，任务不能直接继续；必须先把冲突记录为明确的修复项。

## 目录

```text
ai-docs/
├─ README.md
├─ product/                 # 玩法与产品边界
│  ├─ README.md
│  ├─ game-rules.md
│  └─ roadmap.md
├─ architecture/            # 系统边界与工程约定
│  ├─ README.md
│  ├─ overview.md
│  ├─ conventions.md
│  ├─ paths-and-boundaries.md
│  ├─ match-state-machine.md
│  └─ testing.md
├─ contracts/               # 对外协议与技术选型
│  ├─ README.md
│  ├─ protocol.md
│  ├─ tech-stack.md
│  └─ library-research.md
├─ standards/               # 提交、测试与文档等长期规范
│  ├─ README.md
│  └─ commits.md
├─ task-index.md            # 任务队列与状态摘要
└─ task/                    # 单次任务的范围、决策与验证证据
   ├─ _template.md
   └─ 001-riftcards-rename-task-workflow.md
```

## 阅读顺序

1. `product/game-rules.md`：玩法唯一事实来源。
2. `contracts/protocol.md`：WebSocket 消息唯一事实来源。
3. `architecture/overview.md`：系统边界、数据流、权威状态与回放模型。
4. `architecture/paths-and-boundaries.md`：路径语义、关键目录、允许依赖和禁止路径。
5. `architecture/conventions.md`：命名、协议、错误、日志和代码组织约定。
6. `architecture/match-state-machine.md`：对局生命周期、回合阶段、合法迁移与事件顺序。
7. `architecture/testing.md`：测试层次、必测场景和验证命令。
8. `product/roadmap.md`：当前阶段、下一阶段和暂缓事项。
9. `contracts/tech-stack.md` 与 `contracts/library-research.md`：工具链和依赖决策。
10. `task-index.md`：当前任务、依赖和状态；开始改动前阅读对应 task。

## 文档更新规则

- 规则变化：更新 `product/game-rules.md`、引擎实现、引擎测试。
- 协议变化：更新 `contracts/protocol.md`、Go DTO、C# DTO、协议测试。
- 架构边界变化：更新 `architecture/overview.md`；路径或依赖边界变化同时更新 `architecture/paths-and-boundaries.md`。
- 工具变化：更新 `Taskfile.yml`、`global.json`、`contracts/tech-stack.md`、CI 和对应 task。
- 新依赖：更新 `contracts/library-research.md`，说明替代方案和退出成本。
- 非平凡工作：先建 task 并登记 `task-index.md`；每个 commit 同步 task 记录与验证结果。
