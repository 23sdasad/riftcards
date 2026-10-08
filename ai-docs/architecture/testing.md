# 测试策略

## 测试层次

1. 引擎单元测试：规则校验、费用、目标、攻击、死亡、胜负。
2. 事件测试：每类状态变化的事件数量、顺序、可见性和载荷。
3. 房间测试：版本冲突、串行结算、广播裁剪、断线行为。
4. 协议测试：Go 编码结果能被 C# DTO 解析，C# 指令能被 Go 解析。
5. 端到端测试：启动 Go 服务，两个 Godot/WebSocket 客户端完成一局。
6. 回放测试：同一初始状态和指令序列产生相同事件序列。

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
- 玩家事件流不包含对手手牌和牌库卡牌 ID。
- 同一事件的 `seq` 连续且单调递增。
- 客户端能解析所有服务端示例消息。

## 命令

所有门禁命令从仓库根目录 `Taskfile.yml` 进入，Windows、macOS、Linux 共用同一份定义：

```powershell
task env          # 环境检查：工具名、最低版本、安装入口
task bootstrap    # 安装锁定版本的 golangci-lint、govulncheck
task fmt          # 写入格式化
task fmt:check    # 只读格式检查
task lint         # go vet、golangci-lint、dotnet build --warnaserror
task test         # go test -race、dotnet test
task check        # fmt + lint + test（本地完整门禁）
task ci           # fmt:check + lint + test（只读，与 CI 相同）
task vuln         # govulncheck 依赖漏洞扫描
```

端到端测试在工具链可用后使用固定端口和测试房间；测试结束后必须关闭服务进程。

`go test -race` 使用仓库固定的 LLVM 23.1.3。Windows 通过 `tools/llvm/clang-cl.cmd` 调用官方 `clang-cl` 和 LLVM-MinGW UCRT 运行库；Linux/macOS 使用对应平台的 `clang`。版本和本地环境变量见[LLVM 工具链](../../tools/llvm/README.md)。

## CI

`.github/workflows/ci.yml` 在 `ubuntu-latest`、`windows-latest`、`macos-latest` 上执行：

1. 用按 commit SHA 固定的 action 准备 Go（版本取自 `server/go.mod`）、.NET SDK（`global.json`）和 Go Task 3.54.0。
2. Windows 下载并校验 SHA256 的固定 LLVM 23.1.3 压缩包（`clang-cl` 驱动）和固定 LLVM-MinGW；Unix 使用 runner 自带的 `clang`，实际版本打印在日志中。
3. `task bootstrap` 安装锁定版本的 Go 工具。
4. 以 `dotnet restore --locked-mode` 按 `packages.lock.json` 校验 NuGet 依赖。
5. `task ci`：Go 格式只读检查、`go vet`、`golangci-lint`、C# 构建与格式检查、Go race 测试和 C# xUnit 测试。

纯文档改动通过 `paths-ignore` 跳过完整构建。仓库当前没有 C/C++ 源码，因此 `clang-format`/`clang-tidy` 检查只在存在源文件且工具可用时执行。Godot 图形化测试和 010 定义的本地端到端测试暂不进入该 workflow。

## 完成标准

- 纯规则改动需要引擎测试。
- 协议改动需要序列化兼容测试。
- 连接、重连、广播或房间生命周期改动需要房间或端到端测试。
- 修复缺陷必须添加能复现缺陷的测试，除非问题只存在于外部工具且无法自动化。
