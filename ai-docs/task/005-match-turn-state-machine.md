# 005 — 对局与回合状态机

- 状态：done
- 依赖：003
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

当前 `game.State` 用 `Status` 和 `ActiveSeat` 表示对局状态，`startTurn` 直接执行开始、费用、攻击权和抽牌。规则意图清楚，但阶段迁移没有显式状态，后续指令路径和 UI 很难判断“此刻允许什么”。本任务把对局与回合生命周期收口为可测试的状态机。

## 必读

[游戏规则](../product/game-rules.md) · [架构总览](../architecture/overview.md) · [Path 与工程边界](../architecture/paths-and-boundaries.md) · [测试策略](../architecture/testing.md)。

## 范围与非目标

交付：

- 定义对局生命周期状态及终局转换。
- 定义回合内部阶段及固定事件顺序。
- 把合法迁移、拒绝原因和推进 `revision` / `seq` 的时机写成代码与测试。
- 如状态机改变协议可见字段或规则表述，同步 Go/C# DTO、协议文档和规则文档。

非目标：

- 不实现指令入站和错误码分发；由 006 处理。
- 不设计 UI 操作状态或动画。
- 不增加 Phase 2 重连、计时器或持久化。

## 前置条件与待决策

- 状态机第一版只覆盖对局与回合生命周期。
- 需要确认是否向 `MatchView` 暴露阶段；若暴露，必须按协议变更流程同步两端 DTO。
- 状态迁移不得依赖系统时间或客户端输入。

## 实施步骤

1. 从游戏规则列出对局状态、回合阶段、事件顺序和终局条件。
2. 在 `server/internal/game` 建立显式状态与迁移校验。
3. 为每个合法迁移、非法迁移、拒绝路径和终局路径添加引擎测试。
4. 同步游戏规则、协议与架构文档。
5. 运行 Go 与 C# 全量门禁并记录结果。

## 预计改动

- 新增：`server/internal/game/statemachine.go`（迁移表、阶段推进、终局转换）、`server/internal/game/statemachine_test.go`。
- 修改：`server/internal/game/types.go`、`server/internal/game/engine.go`、`server/internal/game/engine_test.go`。
- 修改：`server/internal/match/room.go`（`NewMatch` 现在返回错误，改为向上传递）。
- 修改：`ai-docs/product/game-rules.md`、新增 `ai-docs/architecture/match-state-machine.md` 并登记索引。
- 未修改协议：`Phase` 只在引擎内部，`MatchView.status` 语义不变，因此不需要动 Go/C# DTO 与 `contracts/protocol.md`。

## 清理与兼容例外

将隐式阶段判断收敛到状态机，删除分散的布尔判断。若保留旧字段，必须注明只读兼容范围与移除条件。

## 验收标准

- [x] 对局和回合的每个状态、允许迁移和终局原因都有测试（迁移表逐项断言、终局原因分 surrender 与 hero_defeated 两组）。
- [x] 非法迁移不改变权威状态，不推进 `revision` 或 `seq`。
- [x] 回合开始、抽牌、行动、结束的事件顺序固定且可回放（顺序断言 + 同种子同指令序列的事件签名一致）。
- [x] 客户端仍只能发送意图，不成为状态机推进者（阶段不进入协议，客户端只用 `status` 与 `activeSeat`）。
- [x] 规则、实现、协议投影和测试一致（`MatchView.status` 语义未变，规则文档与状态机文档同步）。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | Windows / `go test ./internal/game` | 状态与迁移测试通过 | 通过：新增 9 组状态机测试，连同既有引擎测试全部通过 |
| 2026-10-08 | Windows / `task ci` | Go/C# 全部通过 | 通过：golangci-lint 0 issues、C# 构建 0 警告、`go test -race`（game/pathutil/proc）与 toolchain 单测、`dotnet test` 4 通过 |
| 2026-10-08 | 同种子同指令序列回放 | 事件序列一致 | 通过：`TestSameCommandSequenceReplaysIdentically` 比较完整事件签名（seq、revision、类型、可见性、载荷） |
| 2026-10-08 | GitHub Actions run [38018236710](https://github.com/23sdasad/riftcards/actions/runs/38018236710)（`ced61d2`） | 三平台通过 | 通过：ubuntu 2m25s、windows 5m28s、macos 3m35s |

## 风险与回退

主要风险是把规则重构与协议扩展混在一起，造成两端不同步。回退时先保留内部状态机，不暴露新字段；协议扩展拆成独立 commit 和测试。

## 决策与工作记录

- 2026-10-08：创建任务。确认状态机先覆盖对局与回合生命周期，指令处理链路由 006 单独负责。
- 2026-10-08：Taskfile 恢复后，本地门禁统一为 `task test` / `task check`（见 012），验证命令同步更新。
- 2026-10-08：确认 `Phase` 不进入线协议。客户端用 `status` 与 `activeSeat` 已能判断能否行动，暴露阶段会扩大两端 DTO 的同步面；若将来 UI 需要中间态，按协议变更流程单独提交（见状态机文档）。
- 2026-10-08：`NewMatch` 由 `(*State, []Event)` 改为 `(*State, error)`：第二个返回值一直是 nil，改为返回错误后，初始化的阶段推进失败不再被静默忽略；`match.NewRoom` 同步向上传递。
- 2026-10-08：阶段守卫使用既有错误码 `invalid_command`。该码不在 `conventions.md` 的错误码清单中，属于引擎既有用法，与协议错误码的对账留给 006。

## 完成摘要

对局与回合生命周期已收口为显式状态机。生命周期沿用协议可见的 `Status`（`active -> finished`，不可逆），新增内部阶段 `Phase`（`idle -> start -> draw -> action -> ended -> start`）与白名单迁移表：非法迁移返回错误且不修改任何权威状态，`endTurn` 在改动状态前先校验整条迁移链，避免出现半个回合。终局转换集中到 `finish`，写入 `WinnerSeat` 与新增的 `EndReason`（`hero_defeated` / `surrender`）。玩家指令只在 `action` 阶段被接受（认输例外），拒绝路径保证不推进 `revision` 与 `seq`、不产生事件。`NewMatch` 改为返回错误，`match.NewRoom` 同步传递。

测试覆盖：迁移表逐项断言、非法迁移不变更、阶段守卫、终局后所有指令被拒、两种结束原因、固定事件顺序，以及同种子同指令序列的完整事件签名一致。本机 `task ci` 通过（golangci-lint 0 issues、C# 构建 0 警告、`go test -race`、toolchain 单测、`dotnet test` 4 通过）。未改动线协议，因此 C# DTO 与 `contracts/protocol.md` 无需变更；三平台 CI 待推送确认。
