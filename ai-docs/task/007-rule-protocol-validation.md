# 007 — 规则与协议验证基线

- 状态：done
- 依赖：006
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

8 张 MVP 卡牌已有规则实现，但现有引擎测试只覆盖少量主路径，尚未逐项证明规则、事件可见性和 Go/C# 协议解释一致。本任务建立 Phase 1 的自动验证基线，让后续 UI 与端到端任务建立在可重复的契约上。

## 必读

[游戏规则](../product/game-rules.md) · [WebSocket 协议](../contracts/protocol.md) · [测试策略](../architecture/testing.md) · [指令处理路径与错误边界](006-command-path-error-boundaries.md)。

## 范围与非目标

交付：

- 逐项覆盖 8 张 MVP 卡牌的召唤、伤害、治疗、守卫和费用行为。
- 覆盖回合开始、抽牌、烧牌、疲劳、攻击、死亡、胜负和认输事件。
- 覆盖事件 `seq` 连续、`revision` 对账、按座位投影和隐藏信息不泄露。
- 建立 Go 与 C# 共用的协议固定样例，验证两端解析和生成结果。
- 对所有拒绝路径断言状态、`revision` 和 `seq` 不变。

非目标：

- 不新增卡牌、规则能力或 UI。
- 不测试真实网络时序；由 010 处理。
- 不引入大型测试框架。

## 前置条件与待决策

- 依赖 006 的稳定指令与错误边界。
- 共享 fixture 的目录和读取方式必须在 003 路径约束下确定。
- 所有测试必须能在 Windows、macOS、Linux 上运行，不得依赖固定路径分隔符或平台 shell 文本处理。

## 实施步骤

1. 从游戏规则和协议列出卡牌、事件、错误和投影的覆盖矩阵。
2. 补齐 Go 引擎测试与房间/协议测试。
3. 建立共享 JSON 样例，接入 Go 契约测试和 C# xUnit 测试。
4. 对失败样例保留最小复现输入和明确错误码。
5. 在三平台执行全量门禁并登记结果。

## 预计改动

- 新增：`ai-docs/contracts/protocol-fixtures/{client-messages.json,server-messages.json}`（两端共用的固定样例）。
- 新增：`server/internal/testfixtures/fixtures.go`（按 003 路径约束定位并读取样例，只供测试导入）。
- 新增：`server/internal/game/{catalog_test.go,cards_test.go,events_test.go,rejections_test.go}`、`server/internal/protocol/fixtures_test.go`。
- 新增：`client/tests/.../ProtocolFixtureTests.cs`。
- 修改：`client/src/Core/Protocol/Models.cs`（补 `PongData`，此前协议里有 `pong` 而 C# 缺 DTO）、`client/src/Core/Session/GameSession.cs`（解析 `pong` 载荷）。
- 修改：`ai-docs/contracts/protocol.md`（新增第 6 节固定样例）、`ai-docs/contracts/README.md`、`ai-docs/architecture/{testing,paths-and-boundaries}.md`。

## 清理与兼容例外

删除重复内联 JSON 和只覆盖实现细节的脆弱断言，改为稳定的公开行为断言。无协议兼容例外。

## 验收标准

- [x] 8 张 MVP 卡牌的关键成功与失败路径均有测试（目录逐字段对照规则、6 张单位的召唤数值与守卫标记、8 张牌的费用不足、法术目标规则与治疗上限、场地已满、当回合不能攻击、每回合一次攻击）。
- [x] 所有当前事件类型至少有一个顺序、可见性或载荷断言（`turn_started`/`turn_ended`/`card_drawn`/`card_burned`/`card_played`/`unit_summoned`/`attack_resolved`/`damage_dealt`/`healed`/`unit_died`/`fatigue_damage`/`match_ended`）。
- [x] 拒绝指令不改变状态、`revision` 和 `seq`（按错误码逐项断言，并比较投影快照）。
- [x] Go 与 C# 能解析同一组固定样例并得到等价 DTO（同一目录、两端断言相同字段、各自验证往返等价）。
- [x] 测试在 Windows、macOS、Linux 上通过（三平台 CI 均执行了新增的规则与样例测试）。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | Windows / `go test -race ./internal/...` | Go 规则/房间/协议测试通过 | 通过：新增 catalog/cards/events/rejections 与协议样例测试；`seq` 连续、事件版本对账、投影不泄露对手手牌与牌库 |
| 2026-10-08 | Windows / `dotnet test` | C# 协议/会话测试通过 | 通过：14 个测试（原 11 个），新增共享样例解析与生成方向比对 |
| 2026-10-08 | Windows / `task ci` | 全量测试通过 | 通过：golangci-lint 0 issues、C# 构建 0 警告、全部 Go 包 `-race` 通过、`dotnet test` 14 通过 |
| 2026-10-08 | GitHub Actions run [38056724869](https://github.com/23sdasad/riftcards/actions/runs/38056724869)（`0fec5cb`） | 三平台一致 | 通过：ubuntu 1m46s、windows 4m54s、macos 2m26s |

## 风险与回退

风险是为追求覆盖率断言内部实现，导致重构困难。回退时删除实现细节断言，保留规则、事件、投影和错误码等公开契约。

## 决策与工作记录

- 2026-10-08：创建任务。将规则与协议验证放在指令路径稳定之后、UI 之前。
- 2026-10-08：共享样例位置定为 `ai-docs/contracts/protocol-fixtures/`（属 003 的“AI 事实来源”，并已登记进路径表）。Go 侧新增 `server/internal/testfixtures`，用 `runtime.Caller` 定位仓库根后再用 `pathutil` 拼接路径，因此不依赖当前工作目录、也不手写分隔符；C# 侧从测试程序集位置向上查找同一目录。
- 2026-10-08：卡牌与牌组一致性用测试直接对照规则文档（8 张定义逐字段 + 30 张配比），把“规则文档 ↔ 实现”的漂移变成可执行断言。
- 2026-10-08：补齐 C# 侧缺失的 `PongData`：协议里已定义 `pong`，C# 没有对应 DTO，样例测试因此暴露该缺口。同时会话解析 `serverTime`。

## 完成摘要

建立了 Phase 1 的规则与协议验证基线。规则侧：卡牌目录与 30 张起始牌组逐字段对照规则文档；6 张单位牌的召唤数值、守卫标记与当回合不能攻击；8 张牌的费用不足路径；法术目标规则与治疗上限；场地已满、每回合一次攻击；事件侧覆盖全部 12 种事件类型，并断言 `seq` 从 1 连续递增、同一次结算的事件同版本、抽牌与烧牌只对本人可见、投影不泄露对手手牌内容与牌库；拒绝路径按错误码逐项断言状态、`revision`、`seq` 与投影不变。

协议侧：新增 `ai-docs/contracts/protocol-fixtures/` 共享样例（客户端 7 条、服务端 10 条），Go 与 C# 读取同一目录并断言相同字段，各自验证“解析 → 再编码 → 再解析”等价；Go 侧还验证按 DTO 生成的指令与样例逐字段一致。过程中补齐了 C# 缺失的 `PongData` DTO。

测试规模：Go 新增 4 个测试文件 + 1 个样例加载包 + 协议样例测试；C# 由 11 个增加到 14 个。本机 `task ci` 通过（golangci-lint 0 issues、C# 构建 0 警告、全部 Go 包 `-race`、`dotnet test` 14 通过）；三平台 CI 通过（run 38056724869：ubuntu 1m46s、windows 4m54s、macos 2m26s），共享样例在三平台都被读取并解析。
