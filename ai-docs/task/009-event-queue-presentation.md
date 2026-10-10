# 009 — 事件队列与表现层解耦

- 状态：in-progress
- 依赖：008
- 优先级：P1
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

`GameSession` 当前直接累积 `GameEvent`，Godot 只把事件打印到日志。后续动画需要按 `seq` 顺序消费事件，并在事件播放和最新快照之间保持一致。本任务把事件队列放到纯 .NET Core，Godot 只实现表现适配。

## 必读

[架构总览](../architecture/overview.md) · [协议](../contracts/protocol.md) · [Godot 对局界面纵切](008-godot-match-ui.md) · [测试策略](../architecture/testing.md)。

## 范围与非目标

交付：

- Core 侧提供按 `seq` 顺序消费的事件队列。
- 支持暂停、继续、清空和基于最新 `MatchView` 的校正。
- 定义 Godot 可实现的动画/提示接口，不把 Godot 类型带入 Core。
- 处理重复、乱序、缺帧和未知事件类型的明确策略。
- 为队列顺序、校正和错误路径添加纯 .NET 测试。

非目标：

- 不改变服务端事件生成或协议字段。
- 不实现服务端重连和事件补帧；属于 Phase 2。
- 不追求正式动画资源。

## 前置条件与待决策

- 依赖 008 的 UI 事件消费点。
- 事件动画必须是可跳过或可立即收敛的，不能阻塞权威快照校正。
- 未知事件类型必须记录并忽略，不得改变客户端规则状态。

## 实施步骤

1. 为事件批次定义最小队列 API 和游标。
2. 在 `GameSession` 中分离协议接收、快照更新和表现事件消费。
3. 实现重复、乱序、缺帧、未知类型和清空语义。
4. 在 Godot 适配层实现接口并接入对局视图。
5. 运行 C# 测试和双客户端冒烟，记录可观察顺序。

## 预计改动

- 新增：`client/src/Core/Session/EventQueue.cs`（按 `seq` 消费的队列与策略）、`client/src/Core/Session/EventPump.cs`（每帧消费 + `IEventPresenter`）。
- 新增：`client/src/Godot/Match/{EventLogPresenter.cs,EventLogPanel.cs}`（表现适配与面板）。
- 修改：`client/src/Core/Session/GameSession.cs`（队列属性、`match.events` 入队、`match.started` 重置；`Queue()` 改名 `JoinQueue()` 以让出 `Queue` 属性名）、`client/src/Core/Protocol/ProtocolNames.cs`（已知事件类型清单）、`client/src/Main.cs`（事件面板与冒烟参数）。
- 新增测试：`client/tests/.../{EventQueueTests.cs,EventPumpTests.cs,GameSessionEventQueueTests.cs,FakeTransport.cs}`；`GameSessionTests.cs` 改用共用假传输。
- 修改：`README.md`（事件面板、跳过动画、冒烟参数）、`ai-docs/architecture/testing.md`（客户端队列测试层）。

## 清理与兼容例外

删除 Godot 侧直接遍历协议事件的逻辑。保留未知事件忽略策略，保证向后兼容。

## 验收标准

- [x] 事件严格按 `seq` 消费，重复和乱序不会重复执行表现（批内排序、已消费与在队重复都丢弃、迟到事件按序插入，均有测试）。
- [x] 最新快照始终能收敛 UI；动画队列满或失败不会阻塞对局（对局面板直接渲染最新快照；容量上限丢弃最旧事件；表现层返回 false 只影响本帧）。
- [x] Core 不引用 Godot，未知事件不会改变规则状态（未知类型只计数并原样投递，由表现层标注忽略）。
- [x] 队列顺序、校正和错误路径有自动测试（顺序、去重、迟到、缺帧、未知、暂停、清空、校正、容量、重置、泵上限与跳过）。
- [x] Godot 冒烟能观察到事件顺序与快照一致（无界面双客户端冒烟：事件按 `seq` 递增消费，座位私有事件按投影过滤）。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | Windows / `dotnet test client/tests/...` | 队列测试通过 | 通过：49 个测试（原 27 个），新增 22 个（队列 13、泵 6、会话集成 3） |
| 2026-10-08 | Windows / `dotnet build client/Riftcards.Client.csproj --warnaserror` | Godot 适配编译通过 | 通过：0 警告 0 错误 |
| 2026-10-08 | Windows / Godot 无界面双客户端冒烟 | 顺序和快照一致 | 通过：同一对局座位 1/0；客户端 A 观察到 `#1 turn_ended, #2 turn_started, #3 card_drawn, #4 turn_ended, #5 turn_started`，客户端 B 观察到 `#1, #2, #4, #5, #6 card_drawn, #7 match_ended`；各自缺失的序号正是对方看不到的座位私有事件，`seq` 单调递增 |
| 2026-10-08 | Windows / `task ci` | 全量门禁通过 | 通过：golangci-lint 0 issues、C# 构建 0 警告、Go 全部包、`dotnet test` 49 通过 |

## 风险与回退

风险是让队列成为新的状态来源或阻塞最新快照。回退时关闭动画消费，只保留快照校正和事件日志。

## 决策与工作记录

- 2026-10-08：创建任务。事件队列放在 UI 纵切之后，但保持 Core/Godot 分层。
- 2026-10-08：明确权威关系：对局面板始终渲染最新 `MatchView`，事件队列只驱动**装饰性**表现。因此 `match.events` 到达时不做自动校正（否则会立刻丢弃刚入队的事件），校正只在「跳过动画」或队列落后时显式触发。
- 2026-10-08：缺帧策略：记录 `HasGap`/`NextMissingSeq` 后照常消费，不阻塞、不猜测；补帧属于 Phase 2。实现时发现“只看队首”会漏掉中间缺号，已改为扫描整个待处理列表。
- 2026-10-08：容量策略：默认 256，超出丢弃最旧事件（快照权威，丢弃旧动画是安全的），并计数。
- 2026-10-08：`GameSession.Queue()`（加入匹配）改名为 `JoinQueue()`，把 `Queue` 让给事件队列属性；`LeaveQueue()` 保持不变以保持对称。
- 2026-10-08：新增 `--smoke-end-turn` 开发参数，使无显示器冒烟也能产生真实事件序列；冒烟确认了事件顺序与按座位投影过滤。

## 完成摘要

事件队列进入纯 .NET Core：`EventQueue` 负责按 `seq` 排序消费、去重（已消费与在队重复都丢弃）、迟到事件按序插入、缺帧记录（扫描整个待处理列表）、容量上限丢弃最旧事件，以及基于最新 `MatchView` 的校正（把游标推进到快照位置，跳过已覆盖的表现）。`EventPump` 每帧最多播放固定数量事件，表现层可通过返回 false 暂停本帧，并提供 `SkipToSnapshot` 立即收敛。`IEventPresenter` 是 Core 定义的接口，Godot 侧由 `EventLogPresenter` 与 `EventLogPanel` 实现，面板显示待播放数量、缺帧与未知事件提示，并提供「跳过动画」按钮。

`GameSession` 现在把三条路径分开：协议事件留档（`Events`）、表现队列（`Queue`）、快照立即成为权威状态（`View`）；`match.started` 会重置队列与游标，避免跨对局串事件。删除了 Godot 侧直接遍历协议事件的日志逻辑。未知事件类型只计数并原样投递，由表现层标注忽略，不改变任何规则状态。

验证：Core 测试由 27 个增加到 49 个；客户端构建 0 警告；Godot 无界面双客户端冒烟观察到事件严格按 `seq` 递增消费、座位私有事件按投影过滤；`task ci` 通过。三平台 CI 待推送确认。
