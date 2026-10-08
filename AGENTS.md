# AGENTS.md

本仓库是服务端权威的两人回合制卡牌游戏。任何自动化代理开始工作前，按以下顺序阅读：

1. `ai-docs/README.md`
2. `ai-docs/architecture/overview.md`
3. `ai-docs/architecture/conventions.md`
4. `ai-docs/product/game-rules.md`
5. `ai-docs/contracts/protocol.md`
6. `ai-docs/task-index.md`
7. 与任务相关的 task、测试和代码

## 强制约束

- Godot 客户端只发送意图，不计算最终结果。
- Go 服务端是唯一能推进权威状态的位置。
- 已接受的玩家指令必须产生单调递增的 `revision` 和 `seq`。
- 被拒绝的指令不得修改状态，也不得推进 `revision`。
- 广播必须按玩家投影，不能泄露对手手牌或牌库内容。
- 回放以服务端事件日志为准，不从客户端 UI 状态反推。
- 新增协议字段必须同步修改 Go DTO、C# DTO、`ai-docs/contracts/protocol.md` 和契约测试。
- 新增或修改卡牌效果必须同步修改 `ai-docs/product/game-rules.md` 和引擎测试。

## 任务工作方式

- 非平凡改动先从 `ai-docs/task/_template.md` 建 task，并在 `ai-docs/task-index.md` 登记后再实现。
- 范围变化先更新 task；任务完成时用可检查的验证证据更新 task 和索引。
- 每个 commit 在消息中写清 `Task: NNN`、`Validation:` 和 `Cleanup:`，规则见 `ai-docs/standards/commits.md`。
- task 记录单次工作的范围、决策和证据；长期规则沉淀回架构、协议或规范文档。

## 工程边界

- `server/internal/game`：纯规则、状态、事件，不依赖网络。
- `server/internal/match`：房间、队列、串行结算和广播。
- `server/internal/transport/ws`：WebSocket 读写，不包含规则。
- `server/internal/protocol`：传输 DTO 和错误码。
- `client/src/Core`：纯 .NET 协议、会话和回放逻辑，不依赖 Godot。
- `client/src/Godot`：Godot API 适配，不包含规则判断。

## 完成任务前

按 `ai-docs/architecture/testing.md` 的“本地门禁”直接运行 Go 与 .NET 格式化、静态检查和测试。跨平台统一 shell 由 task 004 引入，在此之前直接执行各工具命令。

若本机缺少 Godot、Go 或 .NET SDK，必须明确记录未执行的检查，不能把未验证内容描述为已验证。
