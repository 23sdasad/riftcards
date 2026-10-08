# WebSocket 协议

版本：`0.1`

传输：WebSocket 文本帧，UTF-8 JSON。

服务端地址：`ws://127.0.0.1:8080/ws`

## 1. 通用信封

客户端到服务端：

```json
{
  "type": "queue.join",
  "requestId": "req-001",
  "data": {}
}
```

服务端到客户端：

```json
{
  "type": "queue.status",
  "requestId": "req-001",
  "data": {
    "state": "waiting",
    "position": 1
  }
}
```

字段：

- `type`：消息类型，必填。
- `requestId`：请求关联 ID，响应可选回填。
- `data`：消息载荷，必填；无内容时使用 `{}`。

未知 `type` 返回 `invalid_message`。新增字段默认忽略，关键字段缺失返回 `invalid_message`。

## 2. 客户端消息

### `hello`

建立会话身份。每个连接必须先发送一次。

```json
{
  "type": "hello",
  "requestId": "req-hello",
  "data": {
    "protocolVersion": "0.1",
    "clientVersion": "0.1.0",
    "displayName": "Player One"
  }
}
```

### `queue.join`

加入两人匹配队列。

```json
{
  "type": "queue.join",
  "requestId": "req-queue",
  "data": {}
}
```

### `queue.leave`

离开匹配队列。

```json
{
  "type": "queue.leave",
  "requestId": "req-leave",
  "data": {}
}
```

### `match.command`

提交玩家意图。服务端只接受当前回合玩家会改变状态的指令。

```json
{
  "type": "match.command",
  "requestId": "req-command",
  "data": {
    "matchId": "m_9a2f",
    "commandId": "cmd-001",
    "expectedRevision": 4,
    "command": {
      "type": "play_card",
      "cardInstanceId": "p0-c3",
      "targetId": null
    }
  }
}
```

指令类型：

- `play_card`
  - `cardInstanceId`：手牌实例 ID。
  - `targetId`：法术目标；单位为 `null`。
- `attack`
  - `cardInstanceId`：攻击者实例 ID。
  - `targetId`：对手单位实例 ID 或英雄 ID `hero-0` / `hero-1`。
- `end_turn`
  - 无额外字段。
- `surrender`
  - 无额外字段。

### `ping`

连接活性检测。

```json
{
  "type": "ping",
  "requestId": "req-ping",
  "data": {}
}
```

## 3. 服务端消息

### `welcome`

```json
{
  "type": "welcome",
  "data": {
    "connectionId": "c_4a91",
    "playerId": "p_01f2",
    "displayName": "Player One",
    "serverVersion": "0.1.0",
    "protocolVersion": "0.1"
  }
}
```

### `queue.status`

```json
{
  "type": "queue.status",
  "requestId": "req-queue",
  "data": {
    "state": "waiting",
    "position": 1
  }
}
```

`state`：`waiting` 或 `left`。

### `match.started`

```json
{
  "type": "match.started",
  "data": {
    "matchId": "m_9a2f",
    "seat": 0,
    "state": {
      "matchId": "m_9a2f",
      "revision": 0,
      "lastEventSeq": 0,
      "turn": 1,
      "activeSeat": 0,
      "status": "active",
      "winnerSeat": null,
      "youSeat": 0,
      "players": [
        {
          "seat": 0,
          "heroId": "hero-0",
          "hp": 30,
          "energy": 1,
          "turnNumber": 1,
          "deckCount": 26,
          "handCount": 4,
          "hand": [],
          "board": []
        },
        {
          "seat": 1,
          "heroId": "hero-1",
          "hp": 30,
          "energy": 0,
          "turnNumber": 0,
          "deckCount": 26,
          "handCount": 4,
          "hand": [],
          "board": []
        }
      ]
    }
  }
}
```

`players` 始终按座位排序。`hand` 仅在 `seat == youSeat` 时包含卡牌内容；其他玩家的 `hand` 为空数组，但 `handCount` 真实。

### `match.command_result`

```json
{
  "type": "match.command_result",
  "requestId": "req-command",
  "data": {
    "commandId": "cmd-001",
    "accepted": true,
    "revision": 5,
    "error": null
  }
}
```

拒绝示例：

```json
{
  "type": "match.command_result",
  "requestId": "req-command",
  "data": {
    "commandId": "cmd-001",
    "accepted": false,
    "revision": 4,
    "error": {
      "code": "insufficient_energy",
      "message": "card costs 2 energy, 1 available"
    }
  }
}
```

### `match.events`

广播已接受指令产生的事件。`baseRevision` 是结算前版本，`revision` 是结算后版本。

```json
{
  "type": "match.events",
  "data": {
    "matchId": "m_9a2f",
    "baseRevision": 4,
    "revision": 5,
    "lastEventSeq": 9,
    "events": [
      {
        "seq": 8,
        "revision": 5,
        "type": "card_played",
        "actorSeat": 0,
        "visibility": "public",
        "data": {
          "cardInstanceId": "p0-c3",
          "cardId": "ember_squire"
        }
      }
    ],
    "state": {}
  }
}
```

`state` 是当前接收玩家视角的最新 `MatchView`。客户端必须使用它校正缓存。

事件可见性：

- `public`：双方都收到。
- `seat:0`：仅座位 0 收到。
- `seat:1`：仅座位 1 收到。

当前事件：

- `turn_started`
- `turn_ended`
- `card_drawn`
- `card_burned`
- `card_played`
- `unit_summoned`
- `attack_resolved`
- `damage_dealt`
- `healed`
- `unit_died`
- `fatigue_damage`
- `match_ended`

### `error`

```json
{
  "type": "error",
  "requestId": "req-command",
  "data": {
    "code": "invalid_message",
    "message": "missing commandId"
  }
}
```

### `pong`

```json
{
  "type": "pong",
  "requestId": "req-ping",
  "data": {
    "serverTime": "2026-10-08T12:00:00Z"
  }
}
```

## 4. 版本与兼容

- `protocolVersion` 使用 `主版本.次版本`。
- 主版本不兼容时拒绝连接；次版本新增字段必须向后兼容。
- 客户端必须容忍未知事件类型，记录日志但不改变状态。
- `expectedRevision` 不匹配返回 `stale_revision`，不自动重放客户端猜测的指令。
