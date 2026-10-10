# 006 — 指令处理路径与错误边界

- 状态：done
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

- 修改：`server/internal/transport/ws/handler.go`（传递信封 `requestId`，错误码改用常量）、`server/internal/match/manager.go`（`Connect`/`JoinQueue`/`LeaveQueue`/`Submit` 接收并回填 `requestId`，未入局显式拒绝）、`server/internal/match/room.go`（`Submit` 回填 `requestId`；`Forfeit` 改用类型化 `MatchEventsData`）、`server/internal/protocol/messages.go`（传输/会话级 `Code*` 常量）、`server/internal/game/types.go`（引擎级 `Code*` 常量，`engine.go` 全部改用常量）。
- 新增测试：`server/internal/protocol/messages_test.go`、`server/internal/match/{fake_peer_test.go,room_test.go,manager_test.go}`、`server/internal/transport/ws/handler_test.go`。
- 修改：`client/src/Core/Session/GameSession.cs`（记录待处理请求并按 `requestId` 关联响应，新增 `CommandOutcome`/`LastError`）、`client/tests/.../GameSessionTests.cs`、`ProtocolJsonTests.cs`。
- 修改：`ai-docs/contracts/protocol.md`（信封语义、响应回填、错误码来源表）、`ai-docs/architecture/conventions.md`、`ai-docs/architecture/testing.md`。
- 新增：`ai-docs/architecture/command-path.md` 并登记架构索引。

## 清理与兼容例外

删除空 `requestId`、重复错误分支和绕过房间串行的调用。若必须保留旧行为，标注迁移条件，不把兼容逻辑写成第二套规则。

## 验收标准

- [x] 所有 `match.command` 响应都保留原始 `requestId`（handler 传递信封值，manager 与 room 逐层回填；ws 层用真实 WebSocket 端到端断言）。
- [x] 版本冲突、非法指令和终局后指令不改变状态、`revision` 或 `seq`，也不广播事件。
- [x] 房间内指令严格串行，事件和投影使用同一结算版本（房间锁覆盖版本检查、结算与快照；测试断言 `baseRevision`/`revision`/`state.revision` 一致）。
- [x] 每条错误码都有明确产生位置和测试（协议第 5 节表格 + 两端 `Code*` 常量 + 分层测试）。
- [x] Go/C# DTO、协议文档和契约测试一致（C# DTO 未新增字段；协议文档补 `requestId` 语义与错误码表；C# 测试解析文档示例）。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | Windows / `go test ./internal/...` | 接受与拒绝路径通过 | 通过：新增 protocol（信封严格解码、错误码）、match（接受/过期版本/外部 matchId/终局后/串行/断线/幂等/事件日志）、ws（真实 WebSocket 端到端）测试 |
| 2026-10-08 | Windows / `dotnet test` | 可解析全部响应并关联请求 | 通过：11 个测试（原 4 个），新增 requestId 关联、拒绝结果、错误响应、广播不影响关联、协议文档示例解析 |
| 2026-10-08 | Windows / `task ci` | Go/C# 全量通过 | 通过：golangci-lint 0 issues、C# 构建 0 警告、`go test -race`（game/match/pathutil/proc/protocol/ws）、toolchain 单测、`dotnet test` 11 通过 |
| 2026-10-08 | GitHub Actions run [38049279453](https://github.com/23sdasad/riftcards/actions/runs/38049279453)（`68f137e`） | 三平台通过 | 通过：ubuntu 2m11s、windows 4m53s、macos 3m10s |

## 风险与回退

主要风险是错误码或 `requestId` 调整破坏现有客户端解析。回退时恢复旧响应字段，只保留内部链路的显式参数，并把协议扩展退回独立任务。

## 决策与工作记录

- 2026-10-08：创建任务。确认指令校验与结算路径在状态机之后单独收口。
- 2026-10-08：确认 `requestId` 属于信封（`ClientEnvelope.RequestID`），不是 `MatchCommandRequest` 字段；因此作为函数参数逐层传递，不写入任何 DTO。`commandId` 留在载荷中，两者一起回填。
- 2026-10-08：把回填范围统一到所有请求/响应式消息：`welcome`、`queue.status`、`match.command_result`、`error`、`pong` 都回填原始 `requestId`；广播（`match.started`、`match.events`）不回填。此前只有 `pong` 回填。
- 2026-10-08：修复 manager 在“会话不在对局中”时静默返回的问题：现在显式返回 `not_in_match`，否则客户端拿不到任何响应。
- 2026-10-08：`Room.Forfeit` 去掉信封里非契约的 `reason` 字段，改用类型化 `MatchEventsData`；断线按认输处理，结束原因由 `match_ended` 事件给出（规则 10）。
- 2026-10-08：错误码收敛为常量：传输/会话级放 `protocol`，引擎级放 `game`，避免 `game` 反向依赖 `protocol`。`invalid_command` 此前只在代码中出现，现已补进 `conventions.md` 与协议错误码表。

## 完成摘要

指令路径已收口为分层责任：ws 只解码与校验前置条件，manager 只做会话到房间的映射，room 负责房间归属、版本检查与串行结算，game 只做规则。`requestId` 确认属于信封，逐层作为参数传递并在所有响应中回填（广播不回填）；`commandId` 留在载荷中一并回填。房间锁覆盖“版本检查 → 结算 → 事件与投影快照”，因此串行结算且 `baseRevision`/`revision`/`state.revision` 一致。拒绝路径统一保证不改状态、不推进 `revision`/`seq`、不广播、不写事件日志。错误码收敛为 `protocol` 与 `game` 的 `Code*` 常量，产生位置与载体写入协议第 5 节。

同时修掉两个真实缺陷：会话不在对局中时 manager 静默丢弃指令（现返回 `not_in_match`），以及 `Forfeit` 在信封里发送非契约的 `reason` 字段（现改为类型化 `MatchEventsData`，断线按认输处理）。C# 会话现在记录待处理请求并按 `requestId` 关联响应，暴露 `LastCommandOutcome` 与 `LastError`。

测试：Go 新增 protocol/match/ws 三层测试（含真实 WebSocket 端到端），C# 由 4 个增加到 11 个。本机 `task ci` 通过；三平台 CI 通过（run 38049279453：ubuntu 2m11s、windows 4m53s、macos 3m10s）。
