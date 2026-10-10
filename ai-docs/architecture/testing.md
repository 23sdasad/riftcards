# 测试策略

## 测试层次

1. 引擎单元测试：规则校验、费用、目标、攻击、死亡、胜负。
2. 事件测试：每类状态变化的事件数量、顺序、可见性和载荷。
3. 状态机测试：对局生命周期与回合阶段的合法迁移、非法迁移不变更、终局原因。
4. 房间测试：版本冲突、串行结算、广播裁剪、断线行为。
5. 指令路径测试：信封 `requestId` 回填、未入局拒绝、错误码产生位置；覆盖 `ws → manager → room`，其中 ws 层用真实 WebSocket 端到端验证。
6. 协议测试：Go 编码结果能被 C# DTO 解析，C# 指令能被 Go 解析；C# 会话按 `requestId` 关联响应。
7. 端到端测试：启动 Go 服务，两个 Godot/WebSocket 客户端完成一局。
8. 回放测试：同一初始状态和指令序列产生相同事件序列。

## 必测规则

- 非当前回合玩家不能出牌、攻击或结束回合。
- 费用不足不能出牌。
- 场地已满不能召唤单位。
- 召唤当回合不能攻击。
- 单位每回合只能攻击一次。
- 对手存在守卫时不能绕过守卫攻击英雄。
- 无效目标不改变状态或修订号。
- 英雄血量归零后立即结束。
- 疲劳伤害可以导致失败。
- 认输立即结束，并广播最终事件。

## 必测协议

- 未知协议版本被拒绝。
- `expectedRevision` 过期或超前时不结算。
- 拒绝指令不递增 `revision` 和 `seq`。
- 所有 `match.command` 响应保留原始信封 `requestId`，并回填载荷中的 `commandId`。
- 会话不在对局中时提交指令返回 `not_in_match`，不静默丢弃。
- 广播（`match.started`、`match.events`）不回填 `requestId`。
- 玩家事件流不包含对手手牌和牌库卡牌 ID。
- 同一事件的 `seq` 连续且单调递增。
- 客户端能解析所有服务端示例消息，并把响应关联回原始请求。

## 固定样例

`ai-docs/contracts/protocol-fixtures/` 是两端共用的协议样例目录：

- Go 侧由 `server/internal/testfixtures` 通过 `pathutil` 定位并读取（不依赖当前工作目录），断言在 `server/internal/protocol/fixtures_test.go`。
- C# 侧从测试程序集位置向上查找同一目录，断言在 `ProtocolFixtureTests.cs`。
- 两端对同一份样例断言相同字段，并各自验证“解析 → 再编码 → 再解析”等价。
- 规则侧的一致性由 `server/internal/game/catalog_test.go` 保证：卡牌定义与起始牌组逐字段对照 `product/game-rules.md`。

## 命令

依赖由 Go 引导器装到 `.tools/`，门禁命令从仓库根目录 `Taskfile.yml` 进入，Windows、macOS、Linux 共用同一份定义：

```powershell
go -C tools/toolchain run . bootstrap   # 首次引导（不依赖 task）
task env          # 检查依赖：工具名、最低版本、安装入口
task bootstrap    # 与首次引导等价；-- --without godot 可跳过组件
task fmt          # 写入格式化（server 与 toolchain 两个 Go 模块 + C#）
task fmt:check    # 只读格式检查
task lint         # go vet、golangci-lint（两个 Go 模块）、dotnet build --warnaserror
task test         # go test -race（server）、go test（toolchain）、dotnet test
task check        # fmt + lint + test（本地完整门禁）
task ci           # fmt:check + lint + test（只读，与 CI 相同）
task vuln         # govulncheck 依赖漏洞扫描
```

端到端测试在工具链可用后使用固定端口和测试房间；测试结束后必须关闭服务进程。

`go test -race` 使用仓库固定的 LLVM 23.1.3。Windows 通过 `tools/llvm/clang-cl.cmd` 调用 `clang-cl` 和 LLVM-MinGW UCRT 运行库，两者的安装位置由 `.tools/toolchain.env` 提供；Linux/macOS 使用系统 `clang`。版本与本地环境变量见[LLVM 工具链](../../tools/llvm/README.md)。

## CI

`.github/workflows/ci.yml` 在 `ubuntu-latest`、`windows-latest`、`macos-latest` 上执行：

1. `actions/setup-go` 按 `server/go.mod` 准备 Go；这是唯一由 CI 单独安装的工具。
2. `go -C tools/toolchain run . bootstrap --without godot`：按 `Taskfile.yml` 声明安装 Go 工具、.NET SDK、Godot（CI 跳过）、Windows LLVM 与 LLVM-MinGW，并按 `packages.lock.json` 以 `--locked-mode` 还原 NuGet 依赖。
3. 缓存 `.tools/cache`（下载归档）与 `~/.nuget/packages`，缓存键包含 `Taskfile.yml` 哈希，版本升级自动失效。
4. `task ci`：Go 格式只读检查、`go vet`、`golangci-lint`、C# 构建与格式检查、Go race 测试和 C# xUnit 测试。

纯文档改动（`ai-docs/**`、`**/*.md`、`LICENSE`）不触发该 workflow；改动代码、`Taskfile.yml`、锁文件或 workflow 本身才会运行。Godot 图形化测试和 010 定义的本地端到端测试暂不进入该 workflow。

## 完成标准

- 纯规则改动需要引擎测试。
- 协议改动需要序列化兼容测试。
- 连接、重连、广播或房间生命周期改动需要房间或端到端测试。
- 修复缺陷必须添加能复现缺陷的测试，除非问题只存在于外部工具且无法自动化。
