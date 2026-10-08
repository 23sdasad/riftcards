# 006 — 指令处理路径与错误边界

- 状态：planned
- 依赖：005
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

`match.command` 从 WebSocket 接收后会经过 handler、manager、room 和 game，但当前链路没有把信封 `requestId` 完整传回，错误码、版本检查和事件/投影发送的边界也散落在多个位置。本任务建立从入站到广播的统一指令路径，使拒绝、版本冲突和内部错误都能被稳定观察和测试。

## 必读

[协议](../contracts/protocol.md) · [Path 与工程边界](../architecture/paths-and-boundaries.md) · [对局与回合状态机](005-match-turn-state-machine.md) · [测试策略](../architecture/testing.md)。

## 范围与非目标

交付：

- 明确 `ws -> protocol -> manager -> room -> game` 的输入责任和返回责任。
- 统一 `requestId`、`commandId`、`revision`、错误码和响应类型的传递。
- 保证拒绝路径不改状态、不增版本、不发领域事件。
- 为串行结算、版本冲突、房间归属和内部错误增加测试。
- 同步协议、Go DTO 和 C# 会话处理。

非目标：

- 不增加断线恢复、幂等缓存或持久化。
- 不在 transport 层实现游戏规则。
- 不改变已定义的命令语义。

## 前置条件与待决策

- 依赖 005 的显式状态迁移；本地门禁由根目录 `Taskfile.yml` 提供（见 012）。
- 需先检查 `requestId` 是否属于信封而非 `MatchCommandRequest` 载荷；若属于信封，应通过函数参数传递，不把它写成业务 DTO 字段。
- 协议字段变化必须执行四端同步。

## 实施步骤

1. 为指令链路绘制责任表，标出每一步可产生的错误和状态影响。
2. 调整 handler、manager、room 和 game 函数签名，传递请求关联信息。
3. 统一错误构造，确保响应带正确的 `requestId` 和当前 `revision`。
4. 增加接受、拒绝、过期版本、错误房间和终局后指令测试。
5. 同步 Go/C# DTO、协议文档和契约测试。

## 预计改动

- 修改：`server/internal/transport/ws/handler.go`、`server/internal/match/manager.go`、`server/internal/match/room.go`、`server/internal/protocol/messages.go`。
- 修改：`client/src/Core/Session/GameSession.cs`、协议 DTO 与测试。
- 修改：`ai-docs/contracts/protocol.md` 和指令路径主题文档。

## 清理与兼容例外

删除空 `requestId`、重复错误分支和绕过房间串行的调用。若必须保留旧行为，标注迁移条件，不把兼容逻辑写成第二套规则。

## 验收标准

- [ ] 所有 `match.command` 响应都保留原始 `requestId`。
- [ ] 版本冲突、非法指令和终局后指令不改变状态、`revision` 或 `seq`。
- [ ] 房间内指令严格串行，事件和投影使用同一结算版本。
- [ ] 每条错误码都有明确产生位置和测试。
- [ ] Go/C# DTO、协议文档和契约测试一致。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| — | Go room/protocol/WS 测试 | 接受与拒绝路径通过 | 未执行 |
| — | C# 协议/会话测试 | 可解析全部响应并关联请求 | 未执行 |
| — | Go 与 C# 测试命令 | 全量通过 | 未执行 |

## 风险与回退

主要风险是错误码或 `requestId` 调整破坏现有客户端解析。回退时恢复旧响应字段，只保留内部链路的显式参数，并把协议扩展退回独立任务。

## 决策与工作记录

- 2026-10-08：创建任务。确认指令校验与结算路径在状态机之后单独收口。

## 完成摘要

未完成。
