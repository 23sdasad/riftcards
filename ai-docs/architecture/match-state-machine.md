# 对局与回合状态机

本文记录对局生命周期与回合阶段的显式状态、合法迁移和事件顺序。玩法事实以[游戏规则](../product/game-rules.md)为准；本文只描述这些规则如何收敛成状态机。

实现位置：`server/internal/game/statemachine.go`（迁移表与终局转换）、`server/internal/game/engine.go`（指令处理）。测试：`server/internal/game/statemachine_test.go`。

## 两个维度

状态机有两个互相独立的维度，不要混用：

| 维度 | 字段 | 取值 | 是否协议可见 |
| --- | --- | --- | --- |
| 生命周期 | `State.Status` | `active`、`finished` | 是（`MatchView.status`） |
| 回合阶段 | `State.Phase` | `idle`、`start`、`draw`、`action`、`ended` | 否（引擎内部） |

生命周期回答“对局是否还能继续”，回合阶段回答“此刻允许做什么”。

## 生命周期迁移

只有一条迁移，且不可逆：

```text
active --(英雄生命归零 / 认输)--> finished
```

- 英雄生命归零：结束原因 `hero_defeated`；双方同时归零时当前回合玩家获胜。
- 认输：结束原因 `surrender`；任意阶段都允许（规则 10，MVP 中断线也按认输处理）。
- 终局写入 `WinnerSeat` 与 `EndReason`，并产生 `match_ended` 事件；对局结束后所有指令（含再次认输）都被拒绝，且不推进 `revision` 或 `seq`。
- 终局不改写 `Phase`：它保持结束时的阶段，生命周期一律以 `Status` 判断。

## 回合阶段迁移

合法迁移是白名单，不在表中的迁移一律拒绝：

| 当前阶段 | 允许迁移到 |
| --- | --- |
| `idle` | `start` |
| `start` | `draw` |
| `draw` | `action` |
| `action` | `ended` |
| `ended` | `start` |

一个回合的固定路径：

```text
idle ──(开局)──┐
               ├─> start ─> draw ─> action ─> ended ─┬─> start（对手新回合）
ended ─────────┘                                      └─> finished（本回合内终局）
```

- 新建对局从 `idle` 出发，走完 `start -> draw -> action`，返回时处于 `action`。
- 先手玩家的第一回合不抽牌：仍经过 `draw` 阶段，但不产生抽牌事件（规则 3.4）。
- `endTurn` 在修改任何状态之前先校验所需迁移，避免失败时留下半个回合。
- 非法迁移返回错误且不修改 `Phase`、`Revision` 或 `LastEventSeq`。

## 指令与阶段

- 玩家指令只在 `action` 阶段被接受；其他阶段返回 `invalid_command`（防御性守卫，正常流程下阶段总是 `action`）。
- 认输不受阶段限制。
- 非当前回合玩家的指令返回 `not_your_turn`。
- 拒绝路径统一保证：不修改权威状态、不推进 `revision`、不推进 `seq`、不产生事件。

## 事件顺序

- 回合开始：`turn_started`，随后是抽牌事件（`card_drawn`、`card_burned` 或 `fatigue_damage`）；先手第一回合没有抽牌事件。
- 出牌：`card_played`，单位再跟 `unit_summoned`，法术直接产生效果事件。
- 攻击：`attack_resolved`，随后是 `damage_dealt`（单位对攻时为两条），单位死亡时补 `unit_died`。
- 回合结束：`turn_ended`，随后是对方回合的 `turn_started` 与抽牌事件。
- 终局：`match_ended` 固定为该指令的最后一条事件。

同一初始状态与同一指令序列必须产生相同的事件序列（含 `seq` 与载荷），回放以此为准。

## 与协议的关系

`Phase` 刻意不进入线协议：

- 客户端只需要 `status` 与 `activeSeat` 就能判断自己此刻能否行动，增加字段会扩大两端 DTO 的同步面。
- 若将来 UI 需要展示“正在抽牌”之类的中间态，必须按协议变更流程同时更新 Go DTO、C# DTO、`contracts/protocol.md` 与契约测试，并单独提交。

## 边界

- 指令入站、`requestId` 关联与错误码分发属于[指令处理路径与错误边界](../task/006-command-path-error-boundaries.md)。
- 客户端表现层不复制状态机，只按事件与投影渲染（见[路径与工程边界](paths-and-boundaries.md)）。
