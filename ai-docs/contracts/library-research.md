# 第三方库调研

调研日期：2026-10-08

由于当前执行环境不能访问外网，本文不声称版本是上游最新版。版本以 `go.mod`、NuGet 锁定文件和团队 CI 实际解析结果为准；首次联网启动时执行升级和漏洞检查。

## 结论

首版依赖保持很小：

### Go

选择：

- `github.com/coder/websocket`

用途：服务端 WebSocket 升级、文本消息读写、关闭和上下文取消。

理由：

- API 围绕 `context.Context` 设计，适合连接和优雅停止。
- 依赖少，读写模型简单。
- 支持消息大小限制和连接关闭语义。
- 服务端只需要 WebSocket，不需要大型 Web 框架。

备选：

| 库 | 优点 | 不选原因 |
| --- | --- | --- |
| `github.com/gorilla/websocket` | 生态成熟、例子多 | 需要自行约束并发写和关闭；项目历史维护节奏曾中断 |
| `github.com/gobwas/ws` | 低层性能好 | API 更接近协议细节，首版复杂度偏高 |
| `nhooyr.io/websocket` | 与 coder 分支同源 | 已迁移到 `github.com/coder/websocket`，新项目不再选择旧路径 |

暂不选择：

- Web 框架：`net/http` 的 `ServeMux` 已能覆盖 `/healthz` 和 `/ws`。
- ORM：当前无数据库。
- 状态机库：规则状态明确，使用领域代码更容易测试。
- 依赖注入框架：构造函数显式注入即可。
- `testify`：标准库 `testing` 足够，减少断言 DSL 迁移成本。
- `zap`/`logrus`：标准库 `log/slog` 已满足结构化日志。
- `google/uuid` 或 ULID：当前连接、玩家和卡牌实例 ID 由服务端内部生成；需要跨服务全局排序 ID 时再引入。

### C# / Godot

选择：

- Godot `WebSocketPeer`
- `System.Text.Json`
- xUnit
- `dotnet format`

理由：

- `WebSocketPeer` 随 Godot 提供，避免桌面、Web 和移动平台下另接网络库。
- `System.Text.Json` 是 .NET 内置库，适合首版小而稳定的 JSON 协议。
- `Core` 纯类库可用 xUnit 快速测试，不要求启动 Godot。
- `dotnet format` 由 SDK 提供，与 CI 集成成本低。

备选：

| 库 | 优点 | 不选原因 |
| --- | --- | --- |
| `ClientWebSocket` | .NET 原生异步 API | Godot 的跨平台线程和生命周期适配不如 `WebSocketPeer` 直接 |
| `Newtonsoft.Json` | 灵活、生态广 | 新项目无必要增加依赖，内置序列化足够 |
| `MessagePack-CSharp` | 带宽小、速度快 | 协议可读性下降，首版先以调试效率优先 |
| `CSharpier` | 统一风格强 | `dotnet format` 已足够；团队若需要更强风格约束再引入 |
| `gdUnit4` | Godot 场景和行为测试 | 首版核心逻辑是纯 .NET；场景测试可作为 Phase 1 增量 |
| `FluentAssertions` | 可读断言 | 许可和版本策略需要额外评估，xUnit 原生断言足够 |

## 格式与 lint 架构

采用三层：

1. 编辑器层：`.editorconfig` 统一换行、缩进、字符集。
2. 本地命令层：当前直接使用 Go、.NET 和 lint 工具；跨平台统一 shell 由 task 004 提供。
3. CI 层：直接复用同一组命令，只增加缓存。

Go：

- `gofmt`：不可协商的格式。
- `go vet`：标准库检查。
- `golangci-lint`：聚合静态分析。
- `govulncheck`：依赖漏洞扫描。
- `go test -race`：并发和房间测试。

C#：

- `dotnet format`：格式化和 analyzers。
- `dotnet build --warnaserror`：把关键分析警告提升为失败。
- `dotnet test`：Core 协议和会话测试。

文档：

- Markdown 保持仓库级 `.editorconfig`。
- 协议和规则变更必须同一提交更新，不额外引入文档生成器。

## 未来需要重新评估的依赖

| 需求 | 候选 | 触发条件 |
| --- | --- | --- |
| 持久化 | PostgreSQL + `pgx` | 需要账号、历史对局或跨进程恢复 |
| 房间路由 | Redis | 需要多实例服务 |
| 指标 | `prometheus/client_golang` | 需要生产监控 |
| 追踪 | OpenTelemetry Go/.NET | 需要跨端到跨服务追踪 |
| 模式校验 | JSON Schema 校验器 | 协议字段增长，手工 DTO 测试不足 |
| 二进制协议 | Protobuf/MessagePack | JSON 带宽或序列化成为瓶颈 |
| Godot 场景测试 | gdUnit4 | 需要自动验证场景节点和交互 |

## 依赖准入规则

新增依赖前必须回答：

1. 标准库或现有依赖能否解决？
2. 维护状态、许可证和最近发布时间是否可接受？
3. 是否进入热路径，是否增加分配、线程或平台限制？
4. 是否能被封装在一个适配层内，便于替换？
5. 是否有对应测试和升级策略？

只有答案明确后才修改 `go.mod` 或 NuGet 引用，并同步更新本文。
