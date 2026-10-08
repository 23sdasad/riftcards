# 013 — 修复最新工具链下的本地门禁

- 状态：done
- 依赖：003、012
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

安装最新稳定工具链后，首次运行本地门禁暴露了仓库此前未验证的编译、依赖和格式问题：Go 缺少 `go.sum`，`room.go` 的 `lastEventSeq` 声明与使用不一致；C# 测试缺少 xUnit 引用；Godot 客户端项目未启用隐式 using，且固定 SDK 版本与已安装的 Godot 4.7.2 不一致。本任务修复这些问题，使格式化、静态检查和测试能够作为后续任务的可靠基线。

## 必读

[Path 与工程边界](../architecture/paths-and-boundaries.md) · [测试策略](../architecture/testing.md) · [技术栈](../contracts/tech-stack.md) · [提交规范](../standards/commits.md)。

## 范围与非目标

交付：

- 生成并提交 `server/go.sum`，使 `github.com/coder/websocket` 可被干净环境还原。
- 修复 `server/internal/match/room.go` 中事件序号变量的生命期，保持接受与拒绝路径都发送正确 `lastEventSeq`。
- 使用 Go 1.27 的 `gofmt` 统一现存 Go 文件格式。
- 修复 C# 测试的 xUnit 引用。
- 启用 Godot 客户端项目隐式 using，修复 `Action`、`Queue<T>`、`List<T>` 等系统类型缺失。
- 把 `Godot.NET.Sdk`、`project.godot` 和文档的 Godot 版本口径对齐到当前稳定版 4.7.2。
- 固定官方 LLVM 23.1.3；Linux/macOS 使用 `clang`，Windows 的 Go cgo 使用官方 `clang-cl` 包装器和 LLVM-MinGW UCRT 运行库。
- 配置 `clang-format`、`clang-tidy` 和 `clangd`，并把 LLVM 版本检查接入三平台 CI。
- 运行并通过仓库当前可执行的 Go 与 .NET 门禁。

非目标：

- 不改变游戏规则、协议字段、房间行为或 UI 设计。
- 不为既有 Go/C# 测试补新功能覆盖；测试暴露的新逻辑缺陷另行建 task。
- 不实现跨平台 shell 入口，该范围仍由 004 负责。

## 前置条件与待决策

- 已通过 winget 安装 Go 1.27.0、.NET SDK 10.0.401 和 Godot Mono 4.7.2。
- Godot 命令行短别名已通过用户级 hard link 补充；仓库不记录机器特定路径。
- CI 仍固定 Go 和 .NET 8 口径；本任务先保证本地最新工具链可执行，不改变 CI 版本矩阵。

## 实施步骤

1. 创建 task 013 并登记索引。
2. 修复 Go 编译和格式，运行 `go mod tidy` 生成 `go.sum`。
3. 修复 C# 测试引用和 Godot 项目配置，对齐 Godot SDK。
4. 逐项运行 Go/C# 格式化、静态检查和测试，修复本任务范围内的失败。
5. 更新技术栈文档和 task 验证结果，提交一次。

## 预计改动

- 新增：`ai-docs/task/013-toolchain-gate-repair.md`、`server/go.sum`、`tools/llvm/`、`.clang-format`、`.clang-tidy`。
- 修改：`server/internal/match/room.go`、现有未格式化 Go 文件。
- 修改：C# 测试文件、`client/Riftcards.Client.csproj`、`client/project.godot`。
- 修改：`.github/workflows/ci.yml`、`.gitattributes`、README、架构与技术栈文档、`ai-docs/task-index.md`。
- 新增：Godot 4.7 导入生成的脚本 `.uid` 元数据。

## 清理与兼容例外

不保留本地工具路径或机器特定别名；仓库只记录标准命令和版本边界。Godot 4.4 项目配置升级到 4.7.2，属于已确认工具链升级的一部分。

## 验收标准

- [x] `gofmt -l server/cmd server/internal` 无输出。
- [x] `go vet ./...`、`golangci-lint run ./...` 和 `go test -race ./...` 通过。
- [x] `dotnet format --verify-no-changes`、客户端构建和 xUnit 测试通过。
- [x] `server/go.sum` 已生成，干净环境可还原依赖。
- [x] Godot 项目 SDK、工程特性版本和文档版本一致。
- [x] LLVM 版本文件与三平台 CI 使用同一版本，Windows cgo 通过 `clang-cl.cmd` 执行 race 测试。
- [x] Godot 4.7.2 完成无界面导入，脚本 `.uid` 元数据纳入版本控制。
- [x] 未修改游戏规则、协议字段或权威状态行为。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | Go 1.27 `gofmt`、`go vet`、`golangci-lint`、`go test -race` | 格式与全部 Go 门禁通过 | 通过：`gofmt` 无输出、`vet` 通过、lint 0 issues、全部 race 测试通过 |
| 2026-10-08 | .NET SDK 10 构建、格式检查和测试 | 客户端与测试项目通过 | 通过：Core/Godot 构建 0 警告，格式检查通过，4 个 xUnit 测试通过 |
| 2026-10-08 | Godot 4.7.2 无界面导入 | 工程版本与编辑器一致 | 通过：文件扫描和全局类注册完成，退出码 0 |
| 2026-10-08 | LLVM 23.1.3 工具检查 | `clang`、`clang-cl`、格式、tidy、clangd 可执行 | 通过：版本均为 23.1.3；Windows cgo 使用 `clang-cl.cmd` + LLVM-MinGW 23.1.1 |
| 2026-10-08 | CI YAML 解析、Markdown 链接和差异检查 | 配置可解析、链接有效、无空白错误 | 通过：YAML 可解析，102 个本地链接有效，`git diff --check` 无输出 |
| 2026-10-08 | Linux/macOS CI 实跑 | 三平台 LLVM 与门禁一致 | 先失败后修复：首轮 `install-llvm-action` 无 23.1.3 资产；012 改为 Unix 用 runner `clang`、Windows 用固定 LLVM 23.1.3 压缩包后，run [37804268319](https://github.com/23sdasad/riftcards/actions/runs/37804268319) 三平台通过 |

## 风险与回退

主要风险是 Godot 4.7.2 SDK 升级改变生成代码或 NuGet 还原要求。若无法在 CI 固定版本下兼容，应回退项目版本并明确记录，不把未验证版本写入长期文档。

## 决策与工作记录

- 2026-10-08：创建任务。以最新稳定工具链为本地验证基线，不改变规则和协议语义。
- 2026-10-08：完成 Go/C#/Godot 修复，并以官方 LLVM 23.1.3 和 `clang-cl.cmd` 打通 Windows race；Linux/macOS 使用同一版本 `clang`。
- 2026-10-08：把 CI 的 LLVM 源文件检查改为 POSIX shell 兼容写法，避免 macOS Bash 3.2 缺少 `mapfile`，并按 C/C++ 选择 `clang-tidy` 标准。
- 2026-10-08：更正 —— 推送后的三平台 CI 实跑失败：`KyleMayes/install-llvm-action@v2` 的资产表没有 23.1.3，三个平台都停在 “Install LLVM”。CI 改为 Unix 使用 runner 自带 `clang`、Windows 下载并校验 SHA256 的固定 LLVM-MinGW；`tools/llvm/VERSION` 仍是本地安装的权威版本。详见 012。

## 完成摘要

已修复 `go.sum`、房间事件序号、Go 格式、xUnit 引用和 Godot 隐式 using，并将 Godot SDK 对齐到 4.7.2。Go 全量 race、`go vet`、golangci-lint、Godot/Core 构建、C# 格式检查和 4 个单元测试均通过；Godot 4.7.2 无界面导入成功。LLVM 固定为 23.1.3，Windows cgo 已通过官方 `clang-cl` 包装器验证，Linux/macOS CI 仍待推送后的三平台实跑确认。
