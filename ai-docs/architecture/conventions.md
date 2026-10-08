# 工程约定

## 命名

- Go 包名使用小写单词：`game`、`match`、`protocol`、`ws`。
- C# 命名空间使用 `Riftcards.Client.Core.*` 和 `Riftcards.Client.Godot.*`。
- JSON 字段使用 `camelCase`。
- 协议消息 `type` 使用小写点分形式，例如 `queue.join`、`match.events`。
- 领域事件使用小写下划线形式，例如 `card_played`、`turn_started`。
- 卡牌和实例分别使用 `cardId`、`instanceId`，不要混用。

## 协议

- 所有客户端消息都包含 `type`，有响应的消息包含 `requestId`。
- 所有指令包含 `commandId` 和 `expectedRevision`。
- 所有服务端广播包含 `matchId`、`baseRevision`、`revision`、`lastEventSeq`。
- 新增字段必须保持向后兼容；删除或改变语义必须提升协议小版本。
- 协议 DTO 不做业务校验，业务校验集中在 `game`。

## Go

- 领域层不依赖 `net/http`、WebSocket 或 Godot 相关概念。
- 函数优先返回错误和结果，不通过 panic 处理玩家输入。
- 所有 map 遍历顺序若会影响结果，必须先排序。
- 事件载荷中的时间只记录服务端 UTC 时间。
- 使用 `log/slog` 输出结构化日志。
- 不引入大型框架；标准库无法合理解决时再增加依赖。

## C#

- `Core` 不引用 `Godot` 命名空间。
- `Godot` 适配层不判断卡牌规则，只负责连接、生命周期和展示。
- 协议 DTO 使用不可变属性风格，空集合默认初始化为空数组。
- 解析服务端消息失败时记录协议错误，不尝试猜测字段。
- 共享序列化配置集中在 `ProtocolJson.Options`。

## 错误码

初始错误码：

- `invalid_message`
- `unsupported_protocol`
- `not_authenticated`
- `not_in_match`
- `stale_revision`
- `not_your_turn`
- `invalid_card`
- `insufficient_energy`
- `invalid_target`
- `board_full`
- `match_finished`
- `internal_error`

错误消息面向开发者，客户端行为以错误码为准。

## 文件组织

- 一个规则概念只放一个权威位置。
- 长期说明统一放 `ai-docs`；短生命周期任务说明不能作为契约。
- 公共命令从仓库根目录或后续统一的跨平台 shell 入口进入；不依赖平台专用脚本。
- 新增二进制资源时使用仓库已采用的 Godot 导入流程，不提交 `.godot/`。

## 提交前检查清单

- 规则文档、实现、测试三者一致。
- 协议 Go/C# DTO 一致。
- `architecture/testing.md` 中的本地门禁通过，或明确记录未执行原因。
- 没有日志泄露对手隐藏信息。
- 没有客户端规则计算成为结算依据。
