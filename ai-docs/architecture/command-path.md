# 指令处理路径

本文记录 `match.command` 从 WebSocket 入站到广播的责任划分、关联信息语义和拒绝保证。线协议字段以[协议](../contracts/protocol.md)为准，状态迁移以[对局与回合状态机](match-state-machine.md)为准。

实现位置：`server/internal/transport/ws/handler.go`、`server/internal/match/manager.go`、`server/internal/match/room.go`、`server/internal/game/engine.go`。测试：`server/internal/transport/ws/handler_test.go`、`server/internal/match/{manager,room}_test.go`、`server/internal/protocol/messages_test.go`。

## 责任表

| 层 | 输入责任 | 输出责任 | 可产生的错误 |
| --- | --- | --- | --- |
| `transport/ws` | 读取文本帧、严格解码信封与载荷、校验 `hello` 前置条件 | 协议级 `error` 响应；把信封 `requestId` 与载荷一起交给 manager | `invalid_message`、`unsupported_protocol`、`not_authenticated` |
| `match`（manager） | 会话 → 房间与座位映射；未入局时立即拒绝 | 把 `requestId` 与命令原样转交房间；配对成功时广播 `match.started` | `not_in_match`、`internal_error` |
| `match`（room） | 房间归属检查、`expectedRevision` 检查、房间内串行结算 | `match.command_result`；接受时按座位投影并广播 `match.events` | `not_in_match`、`stale_revision` |
| `game` | 回合、阶段、费用、目标、手牌与终局校验 | `CommandResult`（accepted、revision、events、error） | `invalid_command`、`not_your_turn`、`invalid_card`、`insufficient_energy`、`invalid_target`、`board_full`、`match_finished` |

每层只做自己的事：transport 不含规则，manager 不碰规则，room 不做规则判断，game 不依赖网络。

## 关联信息

- `requestId`：属于**信封**，由客户端提供。服务端逐层作为函数参数传递，不写入任何 DTO 字段；所有请求/响应式消息都必须原样回填。
- `commandId`：属于**命令载荷**，由客户端生成，在 `match.command_result` 中回填，用于确认结果归属。
- `revision`：`match.command_result.revision` 是结算后的权威版本；被拒绝时是当前版本（不推进）。
- 广播（`match.started`、`match.events`）没有单一请求来源，因此不带 `requestId`。

## 串行与一致性

- 房间用一个互斥锁把「版本检查 → 结算 → 取事件与投影快照」包成一次串行区间，因此同一房间的指令严格按到达顺序结算。
- 事件与投影取自同一次结算：`match.events.baseRevision` 是结算前版本，`revision` 是结算后版本，`state.revision` 必须等于 `revision`。
- 发送在解锁之后进行，避免持有锁做 I/O。

## 拒绝保证

任何被拒绝的指令都必须满足：

- 不修改权威状态；
- 不推进 `revision` 与 `seq`；
- 不产生领域事件，也不广播 `match.events`；
- 不写入事件日志（回放只包含已接受指令的事件）；
- 返回带原始 `requestId` 的响应。

会话不在对局中时同样要显式响应（`not_in_match`），不能静默丢弃请求——否则客户端只能等超时。

## 错误码

错误码的产生位置与响应载体见[协议第 5 节](../contracts/protocol.md)。代码中的唯一来源是 `server/internal/protocol` 与 `server/internal/game` 的 `Code*` 常量，禁止在链路里散落字符串字面量。

## 边界

- 断线在 MVP 中按认输处理：由房间产生 `match_ended` 事件，结束原因 `surrender`（规则 10）。信封里不额外添加 `reason` 字段。
- 幂等 `commandId` 缓存、断线补帧与持久化属于 Phase 2，不在当前路径内。
