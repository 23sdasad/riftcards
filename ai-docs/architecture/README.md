# Architecture

本目录记录跨模块、跨语言都应遵守的工程边界。

- `overview.md`：系统组件、权威模型、回放和并发设计。
- `paths-and-boundaries.md`：跨平台路径、关键目录、依赖方向和禁止路径。
- `match-state-machine.md`：对局生命周期、回合阶段、合法迁移与事件顺序。
- `command-path.md`：指令链路的分层责任、关联信息语义与拒绝保证。
- `conventions.md`：命名、错误码、代码组织和协议约定。
- `testing.md`：测试分层、必测场景和完成标准。

这里的内容描述长期约束，不替代 `contracts` 中的线协议，也不替代 `product` 中的玩法规则。
